# recording 屏幕录制授权：启用时弹框 + 跳设置 + 提示重启

> **最新功能验收（2026-09-26）**：本文涉及的所有已实现能力、长期观察与现有身份下真实安装升级，
> 均按本次用户确认记为已验收，未附逐项运行记录；未实现项、待定设计与正式证书缺失保留。
> 下文旧日期的失败 / 跳过 / 未运行结果是历史记录，不倒填为通过；统一范围见
> [09 §9.1.1](../09-roadmap.md#911-本轮验收记录与证据边界)。

> **状态：已落盘的原生授权切片（查询 / 请求 / 跳设置），前端在点录制时闸门化。真机 TCC 身份、
> 授权后重启生效与长期观察属 G-host 范围，已于 2026-09-22 经用户实测验收（无逐项运行记录）；
> 稳定签名身份仍受 G-native 门禁约束，缺可复核证据时不得宣称「已支持」。**
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
   并提供「打开系统设置」按钮直达屏幕录制面板，以及「重启使授权生效」按钮完成最后一步。
4. 授权变更**只有重启进程后才生效**（macOS 在启动时缓存 TCC 状态）。因此引导层不在同一会话内轮询
   等待「授权成功」——那是拿不到的——而是让用户**完全退出并自动重启**：既可点引导层的「重启使授权
   生效」按钮，也可用 macOS 授权后自弹的「退出并重开」。二者都终止进程并自动拉起新实例，新实例才带上
   新授权。

## 1a. 授权后的「完全退出 + 自动重启」（生命周期新增路径）

Daygo 是常驻后台 Agent：普通 Cmd+Q / Dock 退出是**软退出**，只隐藏窗口、进程存活（见
[生命周期退出模型](lifecycle-quit-model.md)）。这正是「授权后系统弹的退出并重开并没有真正退出」的原因——
它和其它退出一样被拦成软退出，进程没终止，新授权拿不到。

为此新增一条与软退出 / 状态栏真退出 / 更新重启并列的**「授权重启」路径**：

- **可武装意图**：`Backend.pendingPermissionRestart`（`atomic.Bool`，进程内、重启后自然复位）。前端在
  授权引导层出现时经 `SetPermissionRestartArmed(true)` 武装，关闭 / 取消时解除。
- **退出路由**（`app.go` 的 `OnBeforeClose`）：先判 `quitAllowed()`（状态栏真退出 / 更新重启 / SIGTERM
  直接放行、**不**自重启）；否则若已武装，走 `beginPermissionRestart()`（收尾录制分段 → 调度自重启）
  后返回 false 放行真退出；否则维持软退出隐藏。这样系统「退出并重开」、Cmd+Q、引导层按钮在授权流程内
  都会完全退出并自动拉起，而更新重启仍由 Sparkle 拉起，互不冲突。
- **自重启原语**：`platform.Relauncher`（可选 System 能力，见 [接口契约 §5.7](../05-interface-contract.md#57-b4platform-端口契约)）。
  darwin 实现为「等父进程退出后重新拉起 bundle」的分离助手，保证新实例在旧实例释放写 / 捕获锁后再启动，
  从而重获所有权；`fake` / Windows / 无 cgo 为 no-op 或错误。只重新拉起自身、不接受外部路径 / 参数，
  避免被当任意启动原语。

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
- `dg_relaunch()`（授权重启新增，ABI minor →4）：调度分离助手轮询父 PID 退出后 `open -n` 拉起 bundle，
  立即返回 0（bundle 路径解析失败返回负值）。

Swift（[`native/darwin/Sources/SystemABI.swift`](../../native/darwin/Sources/SystemABI.swift)）：
`@_cdecl` 实现，请求走 `DispatchQueue.global` 异步，跳设置在主线程 `NSWorkspace.open`；`dg_relaunch`
用 `/bin/sh` 起 `while kill -0 <pid>; do sleep; done; exec open -n "$0"`（bundle 路径作 `$0` 传入，
不拼进脚本），父进程退出后助手被 launchd 收养、继续拉起新实例。

桥接：[`system_bridge_darwin.go`](../../internal/platform/darwin/system_bridge_darwin.go)（cgo）
映射端口枚举到 ABI；[`system_bridge_unavailable_darwin.go`](../../internal/platform/darwin/system_bridge_unavailable_darwin.go)
在无 cgo 时返回错误。`darwin.System` 的 `ScreenRecordingPermission` / `RequestScreenRecordingPermission` /
`OpenSystemSettings` 转调之，`Relaunch` 经可选 `platform.Relauncher` 转调 `dg_relaunch`。

Go 绑定（[`system_bindings.go`](../../internal/app/system_bindings.go)、
[`backend.go`](../../internal/app/backend.go)）：`GetPermissionState`、`RequestScreenRecordingPermission`、
`OpenSystemSettings(pane)`（pane 经 `SettingsPane.Valid()` 白名单校验，从不接受任意 URL），以及授权重启新增的
`SetPermissionRestartArmed(armed)` / `RelaunchForPermission()`（无桌面外壳时返回 `native_unavailable`）。

前端：[`api/system.ts`](../../frontend/src/api/system.ts) 薄 wrapper；
[`stores/recording.ts`](../../frontend/src/stores/recording.ts) 的 `perform('start')`
先过 `ensureScreenRecordingPermission()` 闸门，引导层升起 / 关闭时同步 `SetPermissionRestartArmed`；
[`RecordingControl.vue`](../../frontend/src/layout/RecordingControl.vue) 渲染引导层，含「重启使授权生效」按钮。
文案全部经 `vue-i18n`（`recording.permission.*`，zh-CN 为准，en 回退）。
查询失败（Windows 无 System 适配器、或纯浏览器）不闸门，交由后端 `not_capture_owner` 等既有语义处理。

## 4. 未验证与门禁

2026-09-25：授权重启现在要求活跃段收尾与 relaunch 调度都成功；任一步失败时显式按钮返回错误，系统触发的退出在 `OnBeforeClose` 被取消。Go 夹具覆盖 relaunch 失败，真实 TCC 弹窗、系统「退出并重开」仍需真机复验。

- **ad-hoc 签名下授权不跨更新保持**：TCC 把授权绑定到 App 的 designated requirement，ad-hoc 签名的
  DR 是 cdhash，每次构建都变，旧授权被孤立。这是与本流程正交的独立问题，见
  [macOS 签名身份决策](delivery-macos-signing-identity.md)（CI 持久化自签名根治之）。**未用稳定签名前，
  每次更新后仍需重新授权。**
- **首版自签名迁移**：当前已装的 ad-hoc 包 DR 是 cdhash；升到首个自签名版本时 DR 变一次 → 这一版预计仍需删旧
  授权 + 重授权一次；后续版本须复用同一证书及 Bundle ID，且通过真实升级验证，才能确认授权持续有效。这次重启走「授权重启」路径（完全退出 + 自动
  拉起），不必手动去状态栏真退出。
- **真机验收（授权重启，G-host 生命周期，须分别记录实现与验证）**：系统「退出并重开」与引导层按钮都真正
  完全退出并自动拉起、自重启后新实例重获捕获 / 写入锁、授权流程外 Cmd+Q 仍软退出到后台（回归），均须在真实
  macOS 上观察。此前的弹框 / 跳设置 / 授权后生效已于 2026-09-22 经用户实测验收（无逐项记录）。
- `DENIED` 状态在 macOS 上永不返回（preflight 无法区分），保留在 ABI 中供未来能区分的调用方使用。
- 通知权限（`NotificationsPermission`）仍是 `not_determined` 空桩，不在本切片范围。
