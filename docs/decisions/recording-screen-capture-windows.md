# recording 屏幕截屏（Windows）：DXGI 实现、差异与限制

> **状态：已落盘的实验性原生切片，不在发布范围。** 提交 `c2950cf` 为 Windows 实现了与 macOS
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
2. 在该适配器上创建 `ID3D11Device`，`DuplicateOutput` → `AcquireNextFrame(timeout_ms)` →
   复制到 staging texture → map 读回 BGRA。
3. 按 `DXGI_OUTPUT_DESC.Rotation` 做 90/180/270 旋转，再最近邻等比缩放到 `target_height`，
   宽度为 `round(width × targetHeight / height)`，最小为 1。
4. **DXGI 返回全零帧时回退到 GDI**：`BitBlt(SRCCOPY | CAPTUREBLT)` 抓主监视器矩形。
   这条回退用于驱动或合成器返回空帧的情形，不改变对外语义。
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

## 4. Windows 上目前跑不通的部分

`internal/storage/lock_windows.go` 的 `tryLock` 直接返回错误（`flock` 是 POSIX 设施，
`LockFileEx` 尚未实现）。后果是一条完整的链路结论：

```text
storage.Open → 取写入锁失败 → 返回 error
            → app.Run 记录 storageError 并继续启动窗口
            → 没有数据库：GetSettings / UpdateSettings / GetDiagnostics 返回 database_error
```

因此 **Windows 目前只有"单次截图"这一层可用**，设置、诊断、维护和任何持久化都不可用。
在 `lock_windows.go` 落实 `LockFileEx` 之前，不要把 Windows 构建当作可用产品，也不要以
"Windows 上设置存不下来"为由新开第二套持久化机制（见
[data 实例锁](data-locking.md#5-回退)：替换 `tryLock` 即可，调用方不变）。

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

**未记录。** 本仓库没有任何 Windows 实机验证记录：提交 `c2950cf` 只包含实现。
需要跑的矩阵是 [08 §8.6.3 的 WC 用例](../08-testing-strategy.md#863-wc真实-windows-捕获矩阵)；
在 WC-1…WC-8 有记录之前，Windows 捕获只能标为"已实现、未验证"。

已经确认的只有编译层面的事实（macOS 主机，2026-09-11，commit `c2950cf`）：
`GOOS=windows CGO_ENABLED=0 go build ./internal/...` 通过，即无 cgo 路径返回 `unsupported`
且不破坏核心门禁。**这不是 Windows 截图通过。**

## 7. 边界与回退

- 不得为了让 Windows 出图而放宽隐私规则：把 `privacy_unsupported` 降级成"只检查前台"
  或"忽略屏蔽名单"都是隐私回归，直接否决。
- 不得为 Windows 引入第二套 `Capture` 契约、第二个 ABI 或平台专用 DTO。
- 回退方式：composition root 不注入 `windows.NewCapture()`（当前也尚未注入），
  并保留 `unavailable_windows.go` 的 `unsupported` 路径。删除静态库产物即可回到纯 Go 构建。
