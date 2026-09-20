# recording Windows 分段编码：运行时探测与 HEVC → H.264 → 逐帧 JPEG 降级

> **状态：已决定（2026-09-20）。** Windows 分段录制不得假定 HEVC 编码器存在：
> 编码格式在**首次需要建段时探测一次**，按 `HEVC → H.264 → 逐帧 JPEG` 取
> 第一个真正可用的编码，并把结果缓存到进程结束。探测与降级**全部落在
> Windows 原生适配层内部**，ABI、Go 层与 macOS 实现均不变。
>
> 背景规格见 [HEVC 帧分段](recording-frame-segments-hevc.md)（§「Windows 对齐」），
> Windows 原生调用链见 [屏幕截屏（Windows）](recording-screen-capture-windows.md)。

## 1. 问题

[HEVC 帧分段](recording-frame-segments-hevc.md) 把 Windows 段编码定为
`MFVideoFormat_HEVC`（`native/windows/Sources/daygo_segment.cpp`）。但 HEVC **不是
Windows 的必装组件**：它来自 OEM 预装或商店里的「HEVC 视频扩展」，大量机器上没有。
缺失时 `MFCreateSinkWriterFromURL` / `AddStream` / `BeginWriting` 返回
`MF_E_TOPO_CODEC_NOT_FOUND`（`0xC00D5212`）或等价的失败码。

后果不是「画质差一点」，而是**录制永久停止**：

1. `dg_frame_append` 每帧返回 `DG_CAPTURE_E_IO`；
2. `internal/recorder` 的 `capture()` 把每次失败计入 `consecutiveFailures`；
3. 连续 3 次（`captureFailureLimit`）后 `run()` 返回，状态回到 `idle`；
4. `idle` 的语义是「用户主动关闭」，**没有任何自动恢复路径**，录制就此结束。

默认采集间隔下这大约发生在十几秒内，表现为「点了开始录制，过一会就停了」。
这与代码签名、开发者身份、管理员权限都无关——Windows 截屏本身不要求这些。

## 2. 决策

1. **编码在运行时探测，不写死。** 首次需要建段时按偏好顺序尝试，取第一个能
   真正建起 writer 并写出一帧的编码，结果缓存在进程内；后续段复用该结果，
   不重复探测。
2. **偏好顺序：HEVC → H.264 → 逐帧 JPEG。**
   - `HEVC`（`MFVideoFormat_HEVC`）：体积最优，保留原有默认行为。
   - `H.264`（`MFVideoFormat_H264`）：Media Foundation 自带
     `CLSID_MSH264EncoderMFT`，非 N 版 Windows 开箱可用，覆盖面远高于 HEVC。
   - **逐帧 JPEG**：WIC 是系统组件，任何机器都有，是**一定能成的下限**。
     每帧写入 `segments/daygo-<ms>-<pid>-<seq>.jpg`，即**每帧自成一个单帧段**，
     `frame_index` 恒为 0。
3. **探测要有实证。** 仅确认「writer 建得起来」不足以证明编码可用；探测必须
   走完 `BeginWriting` **并写入一帧**才记为可用。商店版 HEVC 编码器正是能建流、
   却在使用时失败的类型。
4. **首帧写失败也要降级。** 真实段的第一帧若 `WriteSample` 失败（探测未能覆盖的
   残余情形），丢弃这个空段文件、按 §2.2 的顺序降一级，让 recorder 的下一次尝试
   落在可用的编码上。**只有首帧可以降级**：段内已有帧之后再换编码，会让这个段
   没有单一解码器能完整读出。
5. **降级是能力选择，不是隐私放宽。** 任何一级都完整保留屏蔽名单语义；
   不允许为了让 Windows 出图而回到「只检查前台」或忽略名单。
6. **边界：只改 Windows 原生适配层。** ABI 字段与函数签名不变
   （`dg_frame_append` / `dg_frame_decode` / `dg_segment_probe` /
   `dg_segment_close_active` 全部保持），`internal/platform/windows` 之上的
   Go 层、`internal/recorder` 与 macOS 的 `SegmentWriter.swift` 均不改动。

## 3. 为什么不用其它方案

- **默认改用 H.264。** 能解决绝大多数机器，但没有下限：N 版 Windows 或
  裁掉 Media Feature Pack 的镜像仍会失败，且我们无法预知。作为**中间档**保留，
  不作为唯一。
- **引导用户安装 HEVC 扩展。** 把平台自带能力的缺失转嫁给用户，且对已装
  Daygo 的用户不产生任何正向效果；不采用。
- **在 Go 层做降级。** `internal/recorder` 是跨平台代码，在那里分支会同时改变
  macOS 行为，也违反「可移植逻辑不进适配层」的依赖规则（`AGENTS.md`）。
- **把整个录制退回逐帧 JPEG 逐帧 staging。** 这正是
  [HEVC 帧分段](recording-frame-segments-hevc.md) §2 要消除的 10–30× 磁盘开销；
  JPEG 只作为**编码器不可用时的下限**，不是常态路径。
- **完整探测后再建真实段（两遍编码）。** 探测本身需要一次真实编码，与直接把
  首次建段当作探测在成本上等价；分开做只是多一次文件往返。

## 4. 与既有契约的一致性

逐帧 JPEG 段只是「一个段的帧数为 1」，既有寻址与清理契约无需改动：

| 契约 | JPEG 段的满足方式 |
|---|---|
| `(segment_path, frame_index)` 寻址 | 每帧独立路径，`frame_index` 恒为 0 |
| `dg_frame_decode` 的 `.jpg` 分支 | 已存在，直接读文件返回 JPEG 字节 |
| `dg_segment_probe` 的 `.jpg` 分支 | 已存在，返回 `frame_count = 1` |
| 活跃段不可读 | 单帧 JPEG 写完即完整；无未收尾容器 |
| 清理以完整段为单位 | 按 `segment_path` 删除，单帧段即单文件 |
| `screenshots.file_size` 均摊 | `AmortizeSegment` 对单帧段为 `total / 1`，与 `Commit` 写入值一致 |
| `Reconcile` 的 moov 检测 | 仅对 `.mp4` 后缀生效（`internal/storage/captures.go`），不适用于 `.jpg` |
| recorder 的段滚动摊销 | 路径每帧变化 → 上一帧段被摊销，帧增量取该文件全量，与单帧语义一致 |

`internal/storage` 与 `internal/recorder` 因此**不需要任何改动**。

容器段另有一个既存但此前未记录的性质：`dg_segment_probe` 返回的是**编码尺寸**，
Media Foundation 会把它向上取整到编码器的块粒度——64×36 的合成帧在 HEVC 段里读回 64×48。
`dg_frame_append` 报告的则始终是采集尺寸，两者只在尺寸恰为块粒度整数倍时相等
（真实桌面常见的 1280×720 即如此）。当前没有生产代码消费 `SegmentInfo.Width/Height`
（只有 darwin 的 smoke 测试），因此不构成缺陷，但读回尺寸**不能假定等于采集尺寸**。

## 5. 实现切片

| 切片 | 内容 | 验收 |
|---|---|---|
| A 探测与降级 | `daygo_segment.cpp` 内编码偏好链、一次性探测、JPEG 单帧段写入路径与 WIC 编码 | 无编码器机器上录制不中断（真机） |
| B 可观测 | `DAYGO_CAPTURE_DEBUG=1` 输出选中/降级到的编码，不输出路径或图像内容 | 日志可见 `codec.*` 阶段 |
| C 原生 smoke | `native/windows/smoke.cpp` 的分段断言同时接受多帧 MP4 与单帧 JPEG 两种形态 | build.ps1 -RunSmoke 通过 |

## 6. 边界与回退

- 探测结果在进程内缓存，**不落盘、不进设置**：适配层不读设置替自己决策，
  重启后重新探测（与 `AGENTS.md` 的平台适配层约束一致）。
- 残余风险：探测用的合成帧尺寸远小于真实桌面，编码器在真实分辨率下失败的情形
  只能靠 §2.4 的首帧降级兜住。**该路径未在真机验证。**
- 回退：把偏好链收敛为单一 `HEVC` 即可回到本决策前的行为；JPEG 写入路径可以
  整体删除而不影响其它调用方。

## 7. 验证状态

2026-09-20（macOS 开发主机）：决策落盘与原生实现完成，`./scripts/gate.sh` 通过
（含 Windows `CGO_ENABLED=0` Core 交叉构建）。**编码探测、降级链与 JPEG 单帧段
均未在 Windows 真机运行**：Windows 原生构建（`native/windows/build.ps1`）、
`-RunSmoke` 与「移除 HEVC 后的降级」都必须在 Windows 主机上补做，才能从
「已实现」提升为「已验证」。在此之前不得宣称 Windows 录制不再因编码器缺失中断。

2026-09-20（Windows 11 build 26200，RTX 3050 Laptop GPU）：在 Windows 真机补跑
`native/windows/build.ps1 -RunSmoke` 通过。`DAYGO_CAPTURE_DEBUG=1` 显示
`codec.resolved codec=0`（HEVC，本机装有 HEVC 视频扩展），`container segment ok: 2 frames`
证明多帧 MP4 的写入、探测与解码闭环成立；Go 层与 macOS 未改动。

仍未验证的部分（不得据此扩大结论）：**降级链本身在真机上没有被走过**——本机 HEVC 与
H.264 都可用，`codec.degraded` 与 JPEG 单帧段分支只有源码层面的证据。因此 §2 的偏好链
记为「已实现、主路径已真机验证、降级路径未验证」。§6 的残余风险不变：探测用的合成帧
远小于真实桌面，真实分辨率下的编码失败只能靠 §2.4 的首帧降级兜住，该路径同样未在真机触发。
