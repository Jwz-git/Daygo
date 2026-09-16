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
- 崩溃恢复：段文件未收尾即崩溃 → 该段未收尾帧丢弃（与现契约一致），
  pending 意图照常 Reconcile 失败 abandoning。

## 5. 实现切片

| 切片 | 内容 | 验收 |
|---|---|---|
| A 原生段存储 | Swift `SegmentWriter`/`SegmentReader`（AVAssetWriter/Reader + 像素池 + LRU），cgo 桥 `dg_frame_append` / `dg_frame_decode` | 原生 smoke：追加/解码往返，段滚动与 legacy 读 |
| B Go 侧接入 | darwin Capture 返回 (段相对路径, 帧号)；recorder 意图与 `screenshots` 行携带；pending v15 迁移 + 夹具 | DB-2 新夹具 + IT-13 不回归 |
| C 媒体读 | darwin `platform.Media` 实现经桥解码（含 `maxPixelSize`），替换 `/media/frame` handler 后面的实现；legacy JPEG 回退 | 资源 handler 在真实段上行进 |
| D 清理与门禁 | 清理按段删除（活跃段豁免）；G-host 观察磁盘曲线 | 真实 macOS 10 分钟存活 + 磁盘增速对比 |

C 在此之前继续用 `mediafile`（JPEG 直读）作为过渡实现，切片 C 落地后
`mediafile` 保留为非 darwin 平台的兜底实现。
