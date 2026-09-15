# recording 屏幕截屏（Windows）：DXGI/WGC 实现、差异与限制

> **状态：已落盘并完成有限真机 smoke 的实验性原生切片，不在发布范围。** 提交 `c2950cf` 为 Windows 实现了与 macOS
> 同一套 C ABI v1 的 `dg_capture_once`。Windows 是否进入发布仍是
> [09 §9.8 第 18 项](../09-roadmap.md#98-待定设计清单) 的待定项，本文不改变该结论。
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
4. **DXGI 报告真实桌面更新但仍返回全零帧时才回退到 GDI**：
   `BitBlt(SRCCOPY | CAPTUREBLT)` 抓主监视器矩形。pointer-only 黑帧不会触发回退；这使 DXGI
   保持主路径，同时保留驱动/合成器异常时的有限兼容层。
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
不会忽略屏蔽名单，也不会用 GDI 生成可能泄漏的图片。

第四行是当前实现与 ABI 语义之间的**真实缺口**：`flags` 被接受却未生效。补齐方式是合成
`DXGI_OUTDUPL_FRAME_INFO.PointerPosition` 指针，或在 ABI 上明确"该 flag 为平台尽力而为"。
在两者之一落盘前，不要假定 Windows 截图包含光标。

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
| 链接库 | `d3d11 dxgi dxguid ole32 oleaut32 windowscodecs user32 gdi32 advapi32`，外加 MinGW 运行时 `stdc++ gcc gcc_eh` |
| Go 侧 | `internal/platform/windows/bridge_windows.go`（`windows && cgo`） |
| 构建接线 | `cmd/daygo/wails.json` 的 `preBuildHooks["windows/*"]` |
| 开发入口 | `scripts/dev.ps1`（与 `scripts/dev.sh` 对应的 PowerShell 版本；Go 1.25 自动启用 `GOEXPERIMENT=nodwarf5`） |
| 生产构建入口 | `scripts/build.ps1`（`npm ci` → 生成绑定 → Wails `windows/amd64` 构建 → 校验 EXE + DLL） |

Windows 宿主额外限制 DLL 搜索路径为应用目录与 System32，避免从当前工作目录
加载同名 `daygo_windows_native.dll`；窗口主题跟随系统，Windows 11 使用 Mica 背景。

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
受保护内容、GDI 是否绕过保护、光标与 24 小时资源仍未验证。Windows 仍不在发布范围。

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
  typecheck/build 通过。完整 WC 竞态、受保护内容、HDR/旋转及长期资源仍未验收。

## 7. 边界与回退

- 不得为了让 Windows 出图而放宽隐私规则：把 `privacy_unsupported` 降级成"只检查前台"
  或"忽略屏蔽名单"都是隐私回归，直接否决。
- 不得为 Windows 引入第二套 `Capture` 契约、第二个 ABI 或平台专用 DTO。
- 回退方式：composition root 不注入 `windows.NewCapture()`，并保留
  `unavailable_windows.go` 的 `unsupported` 路径。移除 DLL 与代理对象即可恢复仅 DXGI 且隐私
  失败关闭的旧实现。
