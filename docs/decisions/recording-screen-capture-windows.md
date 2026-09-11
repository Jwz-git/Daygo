# recording 屏幕截屏（Windows）：DXGI 实现、差异与限制

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
                      └─ C++ / DXGI Desktop Duplication
                           ├─ 解析系统主监视器
                           ├─ AcquireNextFrame（使用本次 timeout_ms）
                           ├─ 旋转与等比缩放到 target_height
                           ├─ WIC 编码 JPEG
                           └─ 排他发布结果文件（不覆盖已有路径）
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

## 3. 与 macOS 的差异——四条不能忽略

| 项 | macOS | Windows | 后果 |
|---|---|---|---|
| 画面级屏蔽 | `SCContentFilter(excludingApplications:)` | **无公开等价能力** | 屏蔽名单非空且前台未命中时直接返回 `privacy_unsupported`，**不产生图片** |
| 前台应用标识 | Security framework 读签名 identifier，对所有应用可用 | `GetApplicationUserModelId`，只有打包（MSIX）应用有 AUMID | 传统 Win32 应用在前台时取不到标识，同样返回 `privacy_unsupported` |
| 屏幕录制授权 | TCC，`CGPreflightScreenCaptureAccess` 预检 | 系统无对应授权 | Windows 路径不会产生 `permission_denied` |
| 光标 | 尊重 `showsCursor` | **忽略** `DG_CAPTURE_SHOWS_CURSOR` | Desktop Duplication 不含指针，实现也未合成；置位不报错但无效果 |

第一、二行合起来意味着：**只要用户配置了任何屏蔽应用，Windows 上这一路调用就永远拿不到画面。**
这是 [单次调用契约 §7](recording-screen-capture.md#7-windows-约束) 要求的"失败关闭"，不是缺陷；
但它决定了 Windows 在隐私能力补齐前不能承载真实录制，也不能进入发布构建。

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
| 工具链 | MinGW-w64 `g++` / `ar`（**不是 MSVC**），`-std=c++17` |
| 产物 | `build/native/windows/amd64/libdaygo_capture.a` |
| 链接库 | `d3d11 dxgi dxguid ole32 oleaut32 windowscodecs user32 gdi32 advapi32`，外加 MinGW 运行时 `stdc++ gcc gcc_eh` |
| Go 侧 | `internal/platform/windows/bridge_windows.go`（`windows && cgo`） |
| 构建接线 | `cmd/daygo/wails.json` 的 `preBuildHooks["windows/*"]` |
| 开发入口 | `scripts/dev.ps1`（与 `scripts/dev.sh` 对应的 PowerShell 版本） |

`native/windows/smoke.cpp` 直接链接静态库跑一次截图，并用 WIC 解码校验存在非黑像素——
它证明"这台机器上能出图"，不证明隐私、指示器或长期行为。

## 6. 验证状态

2026-09-11，Windows 11（NT 10.0.26200）、NVIDIA RTX 4060 Laptop GPU
（驱动 32.0.15.9174）、双显示器环境：

- `native/windows/build.ps1 -RunSmoke` 通过；WIC 回读 JPEG 为 1280×720 且含非黑像素；
- Go cgo smoke 通过，`CaptureResult` 的宽高/字节数与落盘一致；
- 调试记录显示首帧为 pointer-only 全零纹理，重试后 DXGI 返回 3840×2160 非零 BGRA，最终未命中 GDI；
- 非空屏蔽名单返回 `privacy_unsupported` 且没有目标文件，符合失败关闭；
- `go test ./internal/platform/...`、`go vet ./internal/platform/windows` 与无 cgo 构建通过。

因此 WC-1 只能记为**本机有限通过**；WC-2–8 的完整构造条件、目标路径冲突、多屏切换/旋转、
受保护内容、GDI 是否绕过保护、光标与 24 小时资源仍未验证。Windows 仍不在发布范围。

## 7. 边界与回退

- 不得为了让 Windows 出图而放宽隐私规则：把 `privacy_unsupported` 降级成"只检查前台"
  或"忽略屏蔽名单"都是隐私回归，直接否决。
- 不得为 Windows 引入第二套 `Capture` 契约、第二个 ABI 或平台专用 DTO。
- 回退方式：composition root 不注入 `windows.NewCapture()`（当前也尚未注入），
  并保留 `unavailable_windows.go` 的 `unsupported` 路径。删除静态库产物即可回到纯 Go 构建。
