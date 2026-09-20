# recording 帧分段落盘：Dayflow 式 HEVC 帧段（M2 冻结）

> **状态：已决定（M2 编码决策冻结，2026-09-16）。** 本文冻结 03 §3.4 与
> `recording-image-storage.md` 中悬置的"分段容器与编码格式"：录制端每帧直接以
> 硬件编码追加进 HEVC 段文件，不再经过逐帧 JPEG staging。参数与读写语义照搬
> Dayflow 的 `FrameStore`（本仓库收到的参考实现），实现按 §5 的切片推进。

## 1. 决策

1. **捕获的每一帧在捕获时刻直接追加进当前 HEVC 段文件**，不再写逐帧 JPEG staging。
   编码走 VideoToolbox / `AVAssetWriter`（硬件编码器），容器 MP4。
2. **段参数（对齐 Dayflow `FrameStore`）**：
   - 编码质量 `0.55`（实测保留小号 UI 文字可读性的最低档）；
   - 关键帧间隔 **30 帧**（随机访问最多回退 30 帧，兼顾文件大小）；
   - 段滚动条件：**600 帧或 600 秒**（先到者），分辨率变化立即滚动；
   - 每帧 presentationTime = 段内序号（1fps 压缩时间线），真实时刻在 `screenshots`
     行的 `captured_at`，两者不混用；
   - 帧尺寸变化（接显示器 / 分辨率切换）→ 关闭当前段、开新段。
3. **寻址不变**：`screenshots (segment_path, frame_index)`；`pending_captures`
   增加 `frame_index`（迁移 v15），`relative_path` 语义变为相对段文件路径。
4. **读路径**：`Media.DecodeFrame(s)` 经原生桥按段 + 帧号解码（`AVAssetReader`
   逐段 reader + LRU 缓存约 4 个段），`VTCreateCGImageFromCVPixelBuffer` 转
   CGImage，支持 `maxPixelSize` 缩略图；**旧 JPEG staging 行走 legacy 直读**，
   新旧行共存直到旧文件被清理。
5. **活跃段不可读**：正在写入的段必须先收尾才能解码——与既有契约
   "未收尾的分段可能完全不可读"一致；推送/分析只引用已收尾段。
6. **清理规则不变**：以完整段为单位，绝不删除活跃段（DB-9 与
   `recording-image-storage.md` §1.4 全部保持）。

## 2. 依据（磁盘账）

当前 staging 实测：2584 个 1080p JPEG ≈ **1.0GB**（约 390KB/帧）。同批帧以
HEVC quality 0.55 + 30 帧关键帧间隔编码，相邻帧高度相似时单帧摊销约
20–60KB，同等内容约 **30–80MB**——**10–30 倍差距**，且编码在硬件编码器上
完成，录制路径 CPU 占用可忽略。Dayflow 以此参数长期运行，是本项目对标的
参考实现（其 `FrameStore.swift` 即本决策的出处）。

LLM 发送路径不受影响：仍按 `recording-image-storage.md` §1.5，先选帧引用，
`DecodeFrames` 解出内存图片再编码上传；HEVC 解码侧同样省磁盘但不省发送
前的解码步骤。

## 3. 否决的备选

- **维持逐帧 JPEG staging**：磁盘 10–30× 差距，是本次决策的直接动机。
- **卡片级 timelapse MP4（Dayflow 的 `generateVideoFromScreenshots`）**：
  它解决的是"给用户回放/给 Gemini 喂视频"，不是录制态磁盘效率；作为后续
  可选项挂在 `Media.EncodeVideo`（06 §6.2），不在本决策范围。
- **批后构建分段（原 `recording-image-storage.md` 倾向）**：先写 JPEG 再
  合段需要两遍 IO 且 staging 窗口内崩溃丢帧；直接追加段消除 staging 层。

## 4. 兼容与迁移

- 旧 JPEG staging 行（`frame_index = 0` 的旧行）读路径走 legacy 直读，
  写路径切新段后由既有清理策略逐步消化旧文件；不做一次性转码。
- `pending_captures` 迁移（v15）加 `frame_index INTEGER NOT NULL DEFAULT 0`；
  迁移测试沿用 DB-2 夹具链（`gen.go` 增加 v14→v15 前态夹具）。
- `screenshots.file_size` 分段均摊修复（v16）：对历史库中的多帧 MP4 分段执行
  `file_size = MAX(file_size) / count` 均摊更新，避免每帧累加导致统计虚高；
  录制端追加帧写入 delta 并在分段滚动与暂停收尾时自动执行 `AmortizeSegment`。
- 崩溃恢复：段文件未收尾即崩溃 → 该段未收尾帧丢弃（与现契约一致），
  pending 意图照常 Reconcile 失败 abandoning。

## 5. 实现切片

| 切片 | 内容 | 验收 |
|---|---|---|
| A 原生段存储 | Swift `SegmentWriter`/`SegmentReader`（AVAssetWriter/Reader + 像素池 + LRU），cgo 桥 `dg_frame_append` / `dg_frame_decode` | ★ 已实现：原生 smoke（追加/解码往返、段滚动与 legacy 读通过） |
| B Go 侧接入 | darwin Capture 返回 (段相对路径, 帧号)；recorder 意图与 `screenshots` 行携带；pending v15 迁移 + 夹具 | ★ 已实现：DB-2 v14→v15 迁移夹具与 IT-13 未回归 |
| C 媒体读 | darwin `platform.Media` 实现经桥解码（含 `maxPixelSize`），替换 `/media/frame` handler 与流水线源；legacy JPEG 回退 | ★ 已实现：app 资源 handler 与 analysis 流水线接入 Media 端口 |
| D 清理与门禁 | 清理按段删除（活跃段豁免）；G-host 观察磁盘曲线 | ★ 已实现：按 segment_path 聚合整段删除与 moov atom 对账 |

C 在此之前继续用 `mediafile`（JPEG 直读）作为过渡实现，切片 C 落地后
`mediafile` 保留为非 darwin 平台的兜底实现。

### Windows 对齐

Windows 采用同一 MP4/HEVC、600 帧或 600 秒滚动、1 fps presentation time 和
`(segment_path, frame_index)` 契约，编码/解码使用系统 Media Foundation。Windows Capture
先复用已经验收的 DXGI/WGC 单帧隐私路径取得像素，再交给 Sink Writer；中间 JPEG 是调用期间的
临时文件，成功或失败均删除，不进入数据库，也不作为持久 staging。前台命中屏蔽名单时写入
脱敏占位帧并保持 `blocked` outcome。

该实现不引入 ffmpeg 或新的媒体格式。`platform.Media` 在 Windows 通过 Source Reader 精确读取
目标帧并经 WIC 返回 JPEG，保留历史单 JPEG 文件兼容。源码已通过 Go 无 cgo 交叉构建和公共门禁；
Media Foundation 编译、硬件 HEVC 可用性、逐帧随机读取、滚动和资源释放仍必须在 Windows 主机
运行 `native/windows/build.ps1 -RunSmoke` 及新增分段 smoke 后，才能记为真实通过。

## 6. 验证记录

2026-09-17：切片 A–D 落地并通过门禁与真机 smoke：
- **切片 A 原生段存储**：Swift `SegmentWriter` 实现 VideoToolbox 硬件 HEVC 编码（quality 0.55、关键帧间隔 30、600 帧/600 秒滚动、分辨率变更滚动、隐私占位帧）；`SegmentReader` 实现 `AVAssetReader` 逐段读与 LRU 缓存、`maxPixelSize` 缩略图下采样，以及 legacy `.jpg`/`.jpeg` 直读回退；`native/darwin/build.sh` 构建 universal 静态库（arm64 + x86_64）；通过 `segment_smoke_test.go` 真实 macOS 像素级往返与段滚动验证。
- **切片 B Go 接入**：`internal/platform/ports.go` 扩展 `SegmentCloser` 接口；`darwin.Capture` 接入段追加并返回 `(segment_path, frame_index)`；`internal/recorder` 在适配器满足 `SegmentCloser` 时走段存储模式，并在暂停/退出时收尾活跃段；`storage.Captures` 写入 `(segment_path, frame_index)`；`pending_captures` 迁移至 v15（添加 `frame_index` 且联合唯一 `UNIQUE(relative_path, frame_index)`），通过 v14 真实夹具迁移升级测试（`migrate_test.go`）。
- **切片 C 媒体读**：`internal/platform/darwin` 实现 `platform.Media`；`internal/platform/factory` 提供平台工厂；`internal/app` 的 `/media/frame` 资源处理器与 `internal/analysis` 流水线中的 `mediaFrameSource` 全面接入 `platform.Media` 解码。
- **切片 D 清理与对账**：`internal/storage/cleanup.go` 改写为按 `segment_path` 整段软删除并物理删除段文件，且安全保护未收尾 pending 段与活跃分析批次租用的分段；`Reconcile` 增加 `hasMoovAtom` 检测未最终化的破损 MP4 并自动放弃；全套存储/清理/崩溃夹具测试全部通过。
- **构建与门禁**：`CGO_ENABLED=0 go test ./internal/...`、`CGO_ENABLED=0 go build ./...`、`./scripts/gate.sh` 全绿（前端单元测试 50 通过、typecheck 通过、build 通过、check-docs 0 处问题）。
