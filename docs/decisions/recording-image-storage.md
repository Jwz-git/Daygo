# recording 图片存储流水线：staging、分段、清理与发送

> **状态：架构方向已决定，尚未实现。** 本文固定“像素不进 SQLite、JPEG 仅作 staging、
> 批量构建不可变分段、整段清理、解码后以内存 image parts 发送”的边界。分段容器、编码参数
> 和 Media builder 接口仍须在实现前用匿名夹具验证后冻结。

## 1. 结论

1. **图片像素不写入 SQLite BLOB。** SQLite 保存结构化事实、相对路径、帧序号和生命周期状态；
   像素保存在 `recordings/` 下的文件中。
2. `Capture.Capture` 当前原子输出的 JPEG 是短生命周期 staging 文件，不是永久逐帧图库。
3. 先积累一个分段窗口的 staging JPEG，再由 Media 一次性构建同目录 partial segment，flush 后
   无覆盖原子发布。只有完整 segment 才进入 `screenshots` 可读索引。
4. 清理只删除已关闭的完整 segment；绝不删除 staging、正在构建、活跃或被 pending/processing
   分析批次租用的 segment。
5. 发给 LLM 时不上传 segment，也不制作 zip：analysis 先选定最多 20 个帧引用，批量调用
   `Media.DecodeFrames` 得到有界内存图片，再交给 provider client 编码为对应协议的 image parts。

## 2. 为什么不把图片存进 SQLite

截图是体积大、写入频繁、写后基本不可变的数据；SQLite 更适合它的索引和状态，而不是像素本体。
把 JPEG/BLOB 写进数据库会：

- 放大 WAL、checkpoint、`VACUUM INTO` 备份和恢复成本；
- 让每次截图像素写入占用唯一 writer，增加设置、批次和时间线事务的争用；
- 让数据库损坏的影响同时覆盖结构化历史与全部像素；
- 清理一批旧截图后仍需额外 vacuum 才可能归还文件空间；
- 最终发送 provider 时仍要把 BLOB 读回内存，不能省掉解码和边界检查。

因此文件系统存大块不可变媒体，SQLite 存可事务查询的索引与状态。数据库备份只备份结构化数据；
录制像素留存由独立的 segment 清理策略负责。

## 3. 推荐磁盘布局

```text
<user-config>/Daygo/
├── daygo.sqlite
├── recordings/
│   ├── staging/
│   │   └── <capture-id>.jpg
│   └── segments/
│       └── <segment-id>.<ext>
└── backups/
```

数据库只保存相对 `recordings/` 的规范路径。任何删除或读取操作都必须由 Go 将相对路径解析到
固定 root，并确认解析后的绝对路径仍在 root 内。数据库内容、原生返回值和文件名都不能绕过
这项校验。

## 4. 推荐状态模型

现有目标 `screenshots` 表只表示**已提交且可读**的帧。为支持崩溃恢复，recording 后续迁移应
增加两类内部状态；下列是语义草图，不是已冻结 SQL：

```text
pending_captures
  capture_id        幂等键
  staging_path      唯一相对 JPEG 路径
  requested_at      请求时间
  captured_at       成功后回填
  width/height/bytes
  segment_group     本次准备进入的分段组

recording_segments
  segment_path      唯一相对路径
  state             building | closed | deleting | deleted
  first/last_capture_at
  frame_count
  total_bytes
```

`screenshots` 继续用 `(segment_path, frame_index)` 唯一寻址。是否增加显式 segment 外键，以及
pending 表的准确名称和列，由 recording 的迁移夹具决定；不要在没有恢复测试时先冻结 schema。

## 5. 一次捕获到完整分段

```text
1. Go 生成 capture_id 与 staging 相对路径
2. 短事务 INSERT pending；事务结束
3. Go 从可信 recording root 生成绝对 OutputPath
4. platform.Capture → ABI 原子发布 JPEG
5. Go 校验 outcome、文件存在、尺寸和 FileSize
6. 短事务回填 pending 元数据
7. 达到尺寸变化 / 帧数 / 时长边界时，冻结该 pending 集合
8. Media 读取这些 JPEG，构建同目录 partial segment
9. Media flush、关闭并排他原子发布 final segment
10. 单事务：写 recording_segments(closed)、批量 INSERT screenshots、删除对应 pending
11. 事务成功后删除 staging JPEG
```

任何 SQLite 事务都不得跨越截图、编码或文件删除等慢 I/O。第 10 步是结构化提交点：要么整个
segment 的帧全部可见，要么一帧都不可见。

先 staging、后一次性构建会临时多占一份磁盘，但避免长期打开容器中的三种含糊窗口：容器已追加
而 SQL 未提交、SQL 已保留 frame index 而编码器未 flush、崩溃后 active segment 完全不可读。
默认 10 秒间隔下，一个 600 秒段约 60 张 JPEG，这个权衡优先恢复确定性。

## 6. 启动对账

| 数据库 / 文件状态 | 动作 |
|---|---|
| pending 存在，JPEG 不存在 | 记录失败并删除/重试 pending，不创建 screenshot |
| pending 与 JPEG 都存在 | 校验后重新进入待构建集合 |
| final segment 存在，segment 仍为 building | `ProbeSegment`；可读则完成结构化提交，不可读则删除并保留 staging 重建 |
| segment 已 closed，staging 尚在 | 删除已提交 segment 对应的 staging 遗留 |
| segment 标记 deleting，文件不存在 | 完成 screenshots 软删除并标记 deleted |
| 文件存在但数据库从未登记 | 移入隔离/记录诊断；不得自动信任、发送或按陌生路径操作 |

为了让“segment 已发布、数据库尚未提交”可恢复，构建前必须先持久化确定性的 segment path 和
参与的 capture IDs；不能等编码完成后才第一次记录目标路径。

## 7. 清理流程

清理按 `recording_segments` 而不是单个 screenshot 工作：

1. 计算所有 `closed` segment 的 `total_bytes`，而不是对目录做不受控扫描；
2. 排除当前 building segment，以及被 `analysis_batches.status IN ('pending','processing')` 引用的段；
3. 从最旧 segment 开始选到低于 `storage.recordingsLimitBytes`；
4. 短事务把候选改为 `deleting`，形成可恢复意图；
5. 事务外删除完整 segment 文件；
6. 第二个短事务软删除对应 screenshots，并把 segment 标为 `deleted`；
7. 任一步崩溃由启动对账根据 `deleting` 与文件是否存在继续。

时间线卡片和 observations 不因像素被清理而删除；UI 应显示“该时段媒体已清理”。

## 8. 分析与打包发送

```text
BatchRepository 选出一批 screenshot IDs
  → ScreenshotRepository 返回按 captured_at 排序的 FrameRef
  → analysis 先采样到最多 20 帧并确定 max pixel size
  → Media.DecodeFrames 批量解码 JPEG/PNG/WebP bytes
  → 校验单张 ≤ 5 MiB、合计 ≤ 20 MiB
  → 构造 internal/ai.Request 的有序 text/image parts
  → provider client 按对应协议编码并发送
  → 请求结束后释放内存 bytes
```

不创建持久 zip、不复制到数据库、不把本地路径交给 provider，也不记录请求正文或图片。采样应在
解码前完成，避免先解码数百张再丢弃；同一 batch 内可用有界内存缓存，但它不是持久事实来源。

## 9. 实现前必须补齐的接口

当前 `platform.Media` 只有 decode、timelapse encode 与 probe，没有“由 staging JPEG 构建一个
recording segment”的写入能力。实现前应在消费者侧定义最小接口，例如：

```go
type SegmentBuilder interface {
    Build(ctx context.Context, request BuildSegmentRequest) (BuiltSegment, error)
    Probe(ctx context.Context, relativePath string) (SegmentInfo, error)
}
```

请求必须一次性携带有序 JPEG 输入、final 输出路径、帧率/尺寸和取消上下文；实现只处理媒体，
不读设置、不写 SQL。接口最终放进 `platform.Media` 还是 recording 私有消费者接口，要与容器实验
一起决定。

## 10. 验收条件

- 成功、编码失败、路径冲突、取消时只出现完整 final 或无 final；
- 在步骤 2–11 的每个边界终止进程，重启对账不重复 screenshot、不发送陌生文件；
- segment 构建提交为全有或全无，`(segment_path, frame_index)` 无重复；
- 清理不删除 building、pending 或分析租用中的 segment，崩溃后可继续；
- 录制占用最终收敛到上限，时间线文字仍保留；
- LLM 输入满足 20 张 / 单张 5 MiB / 合计 20 MiB，图片和路径不进入日志、审计表或备份；
- Windows 与 macOS 使用同一 repository/recorder 测试，只替换 Capture/Media 适配实现。
