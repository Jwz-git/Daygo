# recording 屏幕录制授权：启用时弹框 + 跳设置 + 提示重启

> **状态：已落盘的原生授权切片（查询 / 请求 / 跳设置），前端在点录制时闸门化。真机 TCC 身份、
> 授权后重启生效与长期观察仍受 G-host / G-native 门禁约束，未验收前不得宣称「已支持」。**
> 本文记录 Daygo 采用哪种屏幕录制授权流程、依赖哪些 macOS API 承载，以及代码已经做了什么。
> 权限状态与 pane 枚举以 [平台端口 `enums.go`](../../internal/platform/enums.go) 与
> [C ABI `daygo_system.h`](../../native/include/daygo_system.h) 为准。

## 1. 决策

此前录制启动没有任何授权环节：`recorder` 直接开始捕获，未授权时截图连续失败 3 次
（`captureFailureLimit=3`、`captureRetryDelay=2s`）后回到 `idle`，表现为「点录制后几秒就停」。
这是 [09 §9.8 待定设计](../09-roadmap.md#98-待定设计清单) 中「系统授权」项的落地。

采用的用户流程（**启用功能时才请求，不在启动时弹框**）：

1. 用户点录制 → 前端先查 `GetPermissionState()`。
2. 已授权（`granted`）→ 直接放行 `SetRecording(true)`。
3. 未授权 → 调 `RequestScreenRecordingPermission()`：首次触发系统「是否允许 Daygo 录屏」弹框，
   已拒绝过则为 no-op（系统不再弹）。同时前端升起一个引导层，说明「授权 → 重启」两步，
   并提供「打开系统设置」按钮直达屏幕录制面板。
4. 授权变更**只有重启进程后才生效**（macOS 在启动时缓存 TCC 状态），因此引导层明确提示重启，
   不在同一会话内轮询等待「授权成功」——那是拿不到的。

**为什么请求后仍升起自有引导层**：`CGPreflightScreenCaptureAccess()` 只能区分「已授权 / 未授权」，
无法区分「从未决定」和「已拒绝」。已拒绝时系统弹框不再出现，只有自有引导层 + 跳设置能让用户完成
授权。首次授权时两者短暂并存（系统弹框 + 引导层），可接受。

## 2. macOS 承载机制（查官方确认）

- **查询**：`CGPreflightScreenCaptureAccess() -> Bool`。只反映 `kTCCServiceScreenCapture`，从不弹框。
  `false` 可能是未决定或已拒绝，二者不可区分，均映射为 `not_determined`，交由请求路径处理。
- **请求**：`CGRequestScreenCaptureAccess() -> Bool`。首次调用触发系统弹框，已决定后是 no-op。
  在后台队列异步调用并立即返回：授权只在重启后生效，同步等待没有意义。
- **跳设置**：`NSWorkspace.shared.open(URL)`，URL 为
  `x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture`
  （Monterey→Sequoia 通用，已验证）。
- **重启生效**：多来源确认，System Settings 里打开开关后，App 必须完全退出重开才生效
  （macOS 启动时缓存权限状态）。

## 3. 实现

C ABI（[`native/include/daygo_system.h`](../../native/include/daygo_system.h)，ABI minor 0→1，
向后兼容新增函数）：

- `dg_screen_recording_permission_query()` → `DG_PERMISSION_*`（`NOT_DETERMINED/GRANTED/DENIED`）。
- `dg_screen_recording_permission_request()` → 立即返回 0，弹框异步。
- `dg_open_system_settings(pane)` + `DG_SETTINGS_PANE_*`（`SCREEN_RECORDING/NOTIFICATIONS/LOGIN_ITEMS`）。

Swift（[`native/darwin/Sources/SystemABI.swift`](../../native/darwin/Sources/SystemABI.swift)）：
`@_cdecl` 实现，请求走 `DispatchQueue.global` 异步，跳设置在主线程 `NSWorkspace.open`。

桥接：[`system_bridge_darwin.go`](../../internal/platform/darwin/system_bridge_darwin.go)（cgo）
映射端口枚举到 ABI；[`system_bridge_unavailable_darwin.go`](../../internal/platform/darwin/system_bridge_unavailable_darwin.go)
在无 cgo 时返回错误。`darwin.System` 的 `ScreenRecordingPermission` / `RequestScreenRecordingPermission` /
`OpenSystemSettings` 三个此前的空桩转调之。

Go 绑定（[`system_bindings.go`](../../internal/app/system_bindings.go)、
[`backend.go`](../../internal/app/backend.go)）此前已就绪：`GetPermissionState`、
`RequestScreenRecordingPermission`、`OpenSystemSettings(pane)`，pane 经 `SettingsPane.Valid()` 白名单校验，
从不接受任意 URL。

前端：[`api/system.ts`](../../frontend/src/api/system.ts) 薄 wrapper；
[`stores/recording.ts`](../../frontend/src/stores/recording.ts) 的 `perform('start')`
先过 `ensureScreenRecordingPermission()` 闸门；[`RecordingControl.vue`](../../frontend/src/layout/RecordingControl.vue)
渲染引导层。文案全部经 `vue-i18n`（`recording.permission.*`，zh-CN 为准，en 回退）。
查询失败（Windows 无 System 适配器、或纯浏览器）不闸门，交由后端 `not_capture_owner` 等既有语义处理。

## 4. 未验证与门禁

- **ad-hoc 签名下授权不跨重构保持**：TCC 把授权绑定到 App 的 designated requirement，ad-hoc 签名的
  DR 是 cdhash，每次构建都变，旧授权被孤立。这是与本流程正交的独立问题，见
  [macOS 签名身份决策](delivery-macos-signing-identity.md)。**未用稳定签名前，本流程每次重构后都需重新授权。**
- **真机未验收**：系统弹框实际弹出、跳设置面板准确、授权后重启生效、已拒绝路径的引导有效性，
  均须在真实 macOS 上观察，属 G-host 范围。
- `DENIED` 状态在 macOS 上永不返回（preflight 无法区分），保留在 ABI 中供未来能区分的调用方使用。
- 通知权限（`NotificationsPermission`）仍是 `not_determined` 空桩，不在本切片范围。
