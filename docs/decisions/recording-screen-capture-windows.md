# recording 屏幕截屏（Windows）：DXGI/WGC 实现、差异与限制

> **状态：已落盘并完成有限真机 smoke 的原生切片；已排期，发布范围与完整 WC 矩阵经决策推进。** 提交 `c2950cf` 为 Windows 实现了与 macOS
> 同一套 C ABI v1 的 `dg_capture_once`。Windows 发布范围按
> [09 §9.8 第 18 项](../09-roadmap.md#98-待定设计清单) 排期推进，进入发布的门槛见 §6。
>
> 本文只记录**代码已经做了什么、与 macOS 有哪些不能忽略的差异、以及还缺什么**。
> ABI 字段布局以 [`native/include/daygo_capture.h`](../../native/include/daygo_capture.h) 为准；
> macOS 侧实现见 [屏幕截屏 v2](recording-screen-capture-v2.md)，跨平台原始规格见
> [单次调用契约与原生 ABI](recording-screen-capture.md)。

## 1. 调用链

```text
上层 Go 调用方
  └─ platform.Capture.Capture(ctx, request)       与 macOS 完全相同的端口
       └─ internal/platform/windows               bridge_windows.go 是唯一 import "C" 的文件
            └─ daygo_capture.h                    与 macOS 共用的同一份头文件
                 └─ dg_capture_once(...)
                      ├─ 无隐私名单：C++ / DXGI Desktop Duplication
                           ├─ 解析系统主监视器
                           ├─ AcquireNextFrame（使用本次 timeout_ms）
                           ├─ 旋转与等比缩放到 target_height
                           ├─ 会话建立失败或帧不可用时回退 GDI BitBlt
                           ├─ WIC 编码 JPEG
                           └─ 排他发布结果文件（不覆盖已有路径）
                      └─ 有隐私名单且 build 26100+：MSVC C++/WinRT helper
                           ├─ 按 exe 哈希 ID 枚举目标 HWND
                           ├─ WGC SetWindowExclusionList
                           ├─ 等待 ConfigurationIteration 生效
                           └─ WIC 编码并排他发布 JPEG
```

Go 侧没有任何 Windows 专用分支：`windows.NewCapture()` 与 `darwin.NewCapture()` 实现同一个
`platform.Capture`，请求校验复用 `CaptureRequest.Validate()`，错误映射到同一套
`CaptureErrorCode`。**平台差异全部落在原生层的能力上，不落在契约上。**

## 2. 已落盘的行为

1. 主监视器由 `EnumDisplayMonitors` + `MONITORINFOF_PRIMARY` 解析，再匹配到对应的
   `IDXGIOutput1`；调用方不传显示器 ID，与 macOS 的 `CGMainDisplayID()` 语义一致。
2. 在该适配器上创建 `ID3D11Device`，`DuplicateOutput` → `AcquireNextFrame(remaining_timeout)` →
   复制到 staging texture → map 读回 BGRA。若拿到的只是 pointer-only 更新
   （`AccumulatedFrames=0`、`LastPresentTime=0`）且像素全零，则释放该帧并在**同一 timeout 预算**
   内继续等待桌面更新；这修复了部分驱动/虚拟显示组合下首次调用看似成功却得到黑帧的问题。
3. 按 `DXGI_OUTPUT_DESC.Rotation` 做 90/180/270 旋转，再最近邻等比缩放到 `target_height`，
   宽度为 `round(width × targetHeight / height)`，最小为 1。
4. **DXGI 会话建立失败、或报告真实桌面更新但仍返回全零帧时回退到 GDI**：
   `BitBlt(SRCCOPY | CAPTUREBLT)` 抓主监视器矩形。触发条件分两类：
   ①`find_output` / `D3D11CreateDevice` / `DuplicateOutput` 无法建立重复会话——虚拟显示器、
   流式/远程显示器与部分驱动会在这一步直接返回 `DXGI_ERROR_UNSUPPORTED`；②`AcquireNextFrame`
   在本次 timeout 预算内超时，或拿到的帧全零。pointer-only 黑帧不会触发回退。
   会话建立之后 `AcquireNextFrame` 返回的错误码仍按错误上报，不属于回退范围，
   因此该分支只覆盖“这条路径在本机根本用不了”，不掩盖运行中的真实故障。
   GDI 只出现在空屏蔽名单的路径上：非空名单在进入该分支前已交给 WGC 隐私 helper，
   因此回退不改变任何隐私语义。
5. WIC 编码 JPEG，`ImageQuality = jpeg_quality / 100`。
6. 原子发布：同目录创建 `.<文件名>.daygo-<pid>-<tick>.partial`（`CREATE_NEW`），
   编码后 `FlushFileBuffers`，再用 `MoveFileExW(..., MOVEFILE_WRITE_THROUGH)` 发布。
   **没有 `MOVEFILE_REPLACE_EXISTING`**，因此目标已存在时失败并映射为 `io`，与
   [单次调用契约 §3.1](recording-screen-capture.md#31-调用不变量) 第 5 条一致。
7. 全部 C++ 异常在 ABI 边界内截获，返回 `DG_CAPTURE_E_INTERNAL`。
8. `DAYGO_CAPTURE_DEBUG=1` 输出阶段日志，不含路径、窗口标题、应用标识或图像内容。
9. `windows && !cgo` 构建返回 `unsupported`，保证 `CGO_ENABLED=0` 的核心门禁不受影响
   （已验证：`GOOS=windows CGO_ENABLED=0 go build ./internal/...`）。
10. 非空隐私名单在 Windows build 26100+ 改走 Windows.Graphics.Capture：前台目标先返回
    `blocked`；后台窗口全部写入 `SetWindowExclusionList`，且仅接受达到该配置 iteration 的帧。
    捕获前后 HWND 集合变化时丢弃本帧并返回 `blocked`，旧系统仍返回 `privacy_unsupported`。

## 3. 与 macOS 的差异——四条不能忽略

| 项 | macOS | Windows | 后果 |
|---|---|---|---|
| 画面级屏蔽 | `SCContentFilter(excludingApplications:)` | build 26100+ 的 `IDisplayGraphicsCaptureSession.SetWindowExclusionList` | 名单非空改走 WGC；更旧系统 `privacy_unsupported` 且不产生图片 |
| 前台应用标识 | Bundle ID | 规范化 exe 路径的 SHA-256 ID | picker 与运行进程使用同一算法；原始路径不进入前端或设置 |
| 屏幕录制授权 | TCC，`CGPreflightScreenCaptureAccess` 预检 | 系统无对应授权 | Windows 路径不会产生 `permission_denied` |
| 光标 | 尊重 `showsCursor` | **忽略** `DG_CAPTURE_SHOWS_CURSOR` | Desktop Duplication 不含指针，实现也未合成；置位不报错但无效果 |

Windows 隐私能力的硬门禁是 build 26100。更旧系统继续按
[单次调用契约 §7](recording-screen-capture.md#7-windows-约束) 失败关闭；不会降级为只检查前台、
不会忽略屏蔽名单，也不会用 GDI 生成可能泄漏的图片。这里的“不会用 GDI”指**隐私路径**：
GDI 兼容层只在屏蔽名单为空时被使用，且不会被用来伪造窗口排除。名单非空时，
build 26100 以下仍是 `privacy_unsupported` 且不产出图片。

第四行的处置**已决定**（[09 §9.8 #21](../09-roadmap.md#98-待定设计清单)）：ABI 将 `ShowsCursor`
定为**平台尽力而为**，Windows v1 不合成指针（Desktop Duplication 不含指针），置位记为 no-op 且不报错——
因此**不要假定 Windows 截图包含光标**。合成 `DXGI_OUTDUPL_FRAME_INFO.PointerPosition` 指针留作后续
可选增强，落地前不改变该语义，也不阻塞 §9.8 #18 的发布推进。

## 4. Store 兼容层

`internal/storage/lock_windows.go` 已用 `LockFileEx` 实现与 POSIX `flock` 相同的两把非阻塞排他锁：
`daygo.sqlite.lock` 决定唯一 writer，`capture.lock` 决定唯一捕获者。争用映射为 `ErrLockBusy`，
因此 `storage.Open`、连接层只读降级、`Instance` 与上层绑定无需 Windows 专用分支。

Windows 实机测试覆盖了跨进程争用、正常关闭后重取、子进程强制终止后由内核释放、第二实例
只读降级及捕获所有者互斥。它证明设置、诊断与维护不再因“锁未实现”而无法启动；不证明业务表、
recorder 或长期数据库并发已经交付。

## 5. 构建

```powershell
native\windows\build.ps1            # 编译静态库
native\windows\build.ps1 -RunSmoke  # 额外链接并运行原生 smoke
```

| 项 | 值 |
|---|---|
| 工具链 | 外层 cgo ABI：MinGW-w64 C++17；WGC/应用 ABI helper：VS 2022 MSVC C++20 + Windows SDK 26100 |
| 产物 | `build/native/windows/amd64/libdaygo_capture.a` + `daygo_windows_native.dll`（复制到 `build/bin` 与 EXE 同目录） |
| 链接库 | `d3d11 dxgi dxguid ole32 oleaut32 windowscodecs user32 gdi32 advapi32`，外加 MinGW 运行时 `stdc++ gcc gcc_eh`；`stdc++` 必须静态链接，理由见下 |
| Go 侧 | `internal/platform/windows/bridge_windows.go`（`windows && cgo`） |
| 构建接线 | `cmd/daygo/wails.json` 的 `preBuildHooks["windows/*"]` |
| 开发入口 | `scripts/dev.ps1`（与 `scripts/dev.sh` 对应的 PowerShell 版本；Go 1.25 自动启用 `GOEXPERIMENT=nodwarf5`） |
| 生产构建入口 | `scripts/build.ps1`（`npm ci` → 生成绑定 → Wails `windows/amd64` 构建 → 校验 EXE + DLL） |
| 分发验收入口 | `scripts/package-windows.ps1`（NSIS；先签 EXE/DLL，再封装并签安装器；输出 commit + SHA-256 manifest） |

MinGW 的 C++ 运行时必须**静态链接**。`-lstdc++` 在 MinGW 下解析到 DLL import library，
默认会让 EXE 在加载期依赖 `libstdc++-6.dll`；而该文件不在任何分发产物里（NSIS 只封装 EXE
与 `daygo_windows_native.dll`），于是应用只在恰好有 MinGW 运行时在 PATH 上的机器能启动：
没有时 `STATUS_DLL_NOT_FOUND`（`0xC0000135`），命中了版本不匹配的一份时
`STATUS_ENTRYPOINT_NOT_FOUND`（`0xC0000139`）。用 `-Wl,-Bstatic -lstdc++ -Wl,-Bdynamic`
可以把这条依赖从加载期清单里去掉。`internal/platform/windows` 下**每一处**含 `-lstdc++`
的 `#cgo LDFLAGS` 都要这样写：cgo 把同包所有文件的 LDFLAGS 合并成一条链接命令，漏掉任何
一处都会把 DLL 依赖带回来。不要用 `-static-libstdc++`——它只作用于编译器驱动隐式添加的
那份 `-lstdc++`，对显式写出的 `-lstdc++` 无效。

Windows 宿主额外限制 DLL 搜索路径为应用目录与 System32，避免从当前工作目录
加载同名 `daygo_windows_native.dll`；窗口主题跟随系统，Windows 11 使用 Mica 背景。

Wails v2.15 默认 NSIS 模板只封装 EXE，会让已构建成功的应用在安装后因缺少
`daygo_windows_native.dll` 而失去 WGC、应用身份及部分系统能力。Daygo 因此维护
`scripts/windows-installer/project.nsi`，明确把 EXE 与 DLL 放在同一安装目录。打包脚本的第一遍
NSIS 只用于物化 Wails 生成的 include 与 WebView2 bootstrap；签完 EXE/DLL 后必须再运行 makensis，
最后才签安装器。任何省略第二遍封装的流程都不能作为发布候选。

`native/windows/smoke.cpp` 直接链接静态库跑 DXGI 与 WGC 两条截图路径，并用 WIC 解码校验
存在非黑像素；单独的 Edge 基线 / 排除集成图用于验证目标窗口确实不在结果中。

Go 1.25 的 Windows+cgo debug 链接存在已知 DWARF5 PE 布局缺陷
（[golang/go#75077](https://github.com/golang/go/issues/75077)）：Wails `dev` 可完成编译，但生成的
EXE 会被 Windows loader 以 `%1 is not a valid Win32 application` 拒绝。`dev.ps1` 只在检测到
Go 1.25 时，为 Wails 子进程临时设置 `GOEXPERIMENT=nodwarf5` 并在退出后恢复原环境；这保留调试
信息，仅切回旧 DWARF 布局。不要把该错误归因于 DXGI、WebView2 或 CPU 架构。

## 6. 验证状态

2026-09-11，Windows 11（NT 10.0.26200）、NVIDIA RTX 4060 Laptop GPU
（驱动 32.0.15.9174）、双显示器环境：

- `native/windows/build.ps1 -RunSmoke` 通过；WIC 回读 JPEG 为 1280×720 且含非黑像素；
- Go cgo smoke 通过，`CaptureResult` 的宽高/字节数与落盘一致；
- 调试记录显示首帧为 pointer-only 全零纹理，重试后 DXGI 返回 3840×2160 非零 BGRA，最终未命中 GDI；
- 当时非空屏蔽名单返回 `privacy_unsupported`；该限制已由下方 2026-09-13 实现替代；
- `go test ./internal/platform/...`、`go vet ./internal/platform/windows` 与无 cgo 构建通过。

因此 WC-1 只能记为**本机有限通过**；WC-2–8 的完整构造条件、目标路径冲突、多屏切换/旋转、
受保护内容、GDI 是否绕过保护、光标与 24 小时资源仍未验证——这些是 §9.8 #18 排期推进时进入发布的门槛。

2026-09-12（同一 Windows 主机，go1.25.4）：复现 Wails debug EXE 的 PE
`SizeOfHeaders=1352`、`FileAlignment=512`，Windows loader 拒绝启动；启用 `nodwarf5` 后为
`1536/512`，loader 正常启动。随后发现 macOS 状态栏接线在 Windows 的 nil `System` 上调用导致
panic；composition root 增加可选能力守卫后，按 Wails dev 等价 tags 构建的 EXE 持续运行 5 秒。

2026-09-13（同一 Windows 主机，build 26200、SDK 26100）：

- Explorer `.exe` 选择与 application ABI 往返返回 Edge 的稳定哈希 ID、`Microsoft Edge` 名称和
  2849 字节 PNG 图标；设置页显示真实 Windows 版本及 26100 门禁；
- `SetWindowExclusionList` 返回 configuration iteration，集成路径等待对应 frame iteration；
- 1280×720 基线 JPEG 中可见 Edge，排除 JPEG 中 Edge 完全消失并露出下方窗口，两张均非黑；
- `native/windows/build.ps1 -RunSmoke`、Windows platform/app 测试、`go vet`、前端
  typecheck/build 通过。完整 WC 竞态、受保护内容、HDR/旋转及长期资源经用户确认已验收（无逐项记录）。

2026-09-20（macOS 开发主机，仅无头可验证部分）：Windows NSIS 打包入口、内层 EXE/DLL 签名后
再封装流程、版本格式校验和 SHA-256 验收清单已落盘；`./scripts/gate.sh` 通过，含 Windows
`CGO_ENABLED=0` Core 交叉构建。PowerShell、makensis、signtool、安装/卸载/升级均未在 Windows
执行，因此只记为“可进入 WD 验收”，不记为 WD 通过或 Windows 可发布。

2026-09-20（Windows 11 build 26200，RTX 3050 Laptop GPU；主机装有 GameViewer 与 MuMu
虚拟显示适配器）：用户报告 Windows 安装包安装后「开始录制，过了一会就停了」。在本机复现并
定位到两条 Windows 原生缺陷，均与本机是否具备代码签名、开发者身份、管理员权限无关：

- `IDXGIOutput1::DuplicateOutput` 在主输出上返回 `DXGI_ERROR_UNSUPPORTED`（`0x887A0004`）。
  `capture_pixels` 在该步失败时**直接返回**，而既有的 GDI 兼容层只在「DXGI 出了帧但全零」时
  才生效，于是每一帧都失败；`internal/recorder` 的 `captureFailureLimit = 3` 连续失败后
  `run()` 返回并停在 `idle`，`idle` 没有自动恢复路径——这就是「录一会就停」。
  同一台机器上 GDI `BitBlt` 可正常抓到 1536×864 且 100% 非黑的画面，因此 GDI 是一条真实可用的退路。
- `read_video_frame` 用 NULL 属性仓库建 Source Reader，HEVC 解码器输出 NV12 时
  `SetCurrentMediaType(RGB32)` 返回 `MF_E_INVALIDMEDIATYPE`，分段因此被判为不可读。

修复后 `native/windows/build.ps1 -RunSmoke` 在本机通过：`stage=dxgi.duplication_unavailable`
确认走了 §2.4 的 GDI 回退并产出 1280×720 非黑 JPEG 与正确的落盘字节数；WGC 隐私路径
（`privacy capture ok`）不受影响；`container segment ok: 2 frames` 证明分段写入/探测/解码闭环成立；
system event smoke 通过。改动只在 `native/windows/Sources/` 与 `native/windows/smoke.cpp`，
Go 层与 macOS 未改动。

仍未验证：`AcquireNextFrame` 在会话建立之后返回的错误码、多屏/旋转、受保护内容与长时间运行；
本机是**虚拟/流式显示**环境，DXGI 回退才被触发，不代表普通物理显示器主机也会走到 GDI。

同一轮里另修了 §5 的静态链接问题：此前 `Daygo.exe` 在加载期依赖 `libstdc++-6.dll`
（`dumpbin -dependents` 是唯一一个非系统 DLL），用一个不含 MinGW 的 PATH 运行即
`STATUS_DLL_NOT_FOUND`；安装包并不封装这个文件，应用只是「碰巧」在装有 MinGW 的机器上能起来。
改掉两处 `#cgo LDFLAGS` 后该依赖从清单里消失，同一个最小 PATH 下运行成功。
`./scripts/gate.sh` 至此在本机**全绿通过**（binding 生成、Go 测试、vet、三平台无 cgo 交叉构建、
前端单测/typecheck/build、docs 检查）。修复前 gate 停在此处：binding 生成二进制因 PATH 上
Git for Windows 那份版本不匹配的 `libstdc++-6.dll` 以 `0xC0000139` 启动失败，以及
`internal/platform/windows` 的 `TestSystemEventKind` 与同文件的
`TestSystemEventKindIncludesScreensaverAndDisplays` 对 kind 5 的期望自相矛盾。

Windows 侧仍未验证的项目不变，另外新增一项：上述结论都来自**本机**（装有 MinGW-w64 与
Git for Windows 的开发机），干净 Windows 机器上的安装/启动尚未复测。

2026-09-20（同一台机器）：隐私设置页出现两个 Edge 图标，根因在枚举层而非渲染层，已按 §8 的
规则修复（应用去重与名称读取，见 [应用身份解析](recording-application-picker.md) §1.4 / §1.5）。
本机复跑枚举：候选路径 196 条 → 86 个条目，`Microsoft Edge` 只剩一条且与运行进程身份一致，
带非打印字符的名称归零。改动只落在 `internal/platform/windows` 与
`native/windows/Sources/daygo_windows_native.cpp` 的名称读取，ABI、Go 端口与 macOS 未改动。

2026-09-20（同一台机器）：用户报告录下来的画面是倒着的。根因在 §8 的分段写入路径——
`sink writer` 的输入类型没有声明行序，Media Foundation 因此按自下而上读走自上而下的采集
缓冲区，每个分段都被写成上下镜像。修复是在 `build_writer` 的输入类型上声明
`MF_MT_DEFAULT_STRIDE`；读取端不动（实测解码输出的内存行序与画面一致）。探针数据、被否定的
「读取端翻回」假设与 smoke 回归断言见
[Windows 分段编码](recording-windows-segment-codec.md) §7。改动只在
`native/windows/Sources/daygo_segment.cpp` 与 `native/windows/smoke.cpp`。

## 7. 边界与回退

- 不得为了让 Windows 出图而放宽隐私规则：把 `privacy_unsupported` 降级成"只检查前台"
  或"忽略屏蔽名单"都是隐私回归，直接否决。
- 不得为 Windows 引入第二套 `Capture` 契约、第二个 ABI 或平台专用 DTO。
- 回退方式：composition root 不注入 `windows.NewCapture()`，并保留
  `unavailable_windows.go` 的 `unsupported` 路径。移除 DLL 与代理对象即可恢复隐私失败关闭的
  旧实现。若要单独回退 §2.4 扩容后的 GDI 回退，把 `capture_duplication` 的
  `DuplicationResult::kUnavailable` 当作 `kFailed` 直接返回即可，GDI 在调用点被跳过。

## 8. macOS 能力差集接入（2026-09-20）

- Windows Capture 已接 `SegmentCloser`，非空 `SegmentDirectory` 走 Media Foundation 分段；
  读取走 Source Reader + WIC，历史 JPEG 仍可读。格式和滚动语义以
  [HEVC 分段决策](recording-frame-segments-hevc.md)为准，编码选型与降级见
  [Windows 分段编码决策](recording-windows-segment-codec.md)：HEVC 不是 Windows 必装组件，
  缺失时按 HEVC → H.264 → 逐帧 JPEG 降级，避免录制因编码器缺失而永久停止。
  读取端建 Source Reader 时必须设置 `MF_SOURCE_READER_ENABLE_ADVANCED_VIDEO_PROCESSING`：
  HEVC 解码器输出 NV12，只有启用该属性 Media Foundation 才会插入转 RGB32 的 video processor；
  否则 `SetCurrentMediaType(RGB32)` 返回 `MF_E_INVALIDMEDIATYPE`，分段被判为不可读，
  时间线与分析拿不到任何帧。
- System 事件新增屏保开始/结束与显示器变化。屏保状态通过
  `SPI_GETSCREENSAVERRUNNING` 定时观察状态转换，显示器变化使用 `WM_DISPLAYCHANGE`；不生成重复
  状态事件。
- 已安装应用从 Windows 注册表的 App Paths / Uninstall（用户/机器、32/64 位视图）枚举，候选
  必须再经过同一 `ApplicationInspector` 生成规范化路径哈希 ID，不能用注册表键名充当隐私身份。
  一个应用只占一个条目：来源按 App Paths 优先，显示名称、文件名与字节内容都相同的候选视为
  同一个程序安装两次而合并——Edge 就是这样把 `msedge.exe` 装了两份并分别注册的，只保留
  `App Paths` 那条（运行进程实际使用的路径）。规则与去重理由见
  [应用身份解析](recording-application-picker.md) §1.4。
- Windows 不存在 `NSApplicationActivationPolicy` 的同构 API；通知区生命周期已经由 System
  适配器持有。因此 regular/accessory/prohibited 请求做参数校验和幂等状态记录，窗口可见性仍由
  app/Wails 层处理，不声称它能改变任务栏中任意窗口的样式。

上述项目当前只有 macOS 主机上的源码检查、Go 测试和 Windows 无 cgo 交叉构建证据；必须补真实
Windows 原生构建及交互 smoke，才能从“已接线”提升为“已验证”。
