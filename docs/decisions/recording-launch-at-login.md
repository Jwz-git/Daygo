# recording 开机自启：macOS 用 SMAppService.mainApp，Windows 用 Run 键

> **状态：方案（待实现）。** 本文只落地"开机自启"的设计决策——端口不变、macOS 选型、
> Windows 现状、状态映射与实现分片。macOS 原生实现尚未编写，Windows 已实现但真机 / 签名身份
> 下的自启行为仍受 G-host / G-native 门禁约束，缺可复核证据时不得宣称「已支持」。
> 端口签名以 [平台端口 `ports.go`](../../internal/platform/ports.go) 为准，
> System C ABI 以 [`daygo_system.h`](../../native/include/daygo_system.h) 为准。

## 1. 决策

"开机自启（Launch at login）"指用户登录系统时自动拉起 Daygo，是 [09 §9.3](../09-roadmap.md#93-能力接入表)
中归 recording 的 System「自启」能力，也是 [09 §9.8](../09-roadmap.md#98-待定设计清单)「平台适配形态」
下需要逐平台落地的一项。它与 `internal/app/auto_start.go` 的 `maybeAutoStartRecording`（App 启动后
自动开始**录制**）是两个概念，本文只谈前者。

- **端口不变。** [`System`](../../internal/platform/ports.go) 已有
  `LaunchAtLogin(ctx) (bool, error)` 与 `SetLaunchAtLogin(ctx, enabled) error`，
  设置键 `system.launchAtLogin`、DTO、patch 通路也已就绪。本决策不改端口签名。
- **macOS：`SMAppService.mainApp`（ServiceManagement，macOS 13+）**，`register()` / `unregister()` /
  `status`。不引入 login-item helper，不写 LaunchAgent plist。
- **Windows：保留现状**——per-user Run 键
  `Software\Microsoft\Windows\CurrentVersion\Run`，值名 `Daygo`，已在
  [`system_settings_windows.go`](../../internal/platform/windows/system_settings_windows.go) 实现。
- **Linux：暂不支持**，返回 unsupported / no-op，跟随
  [Linux 截图与系统能力决策](recording-screen-capture-linux.md) 另行落地。
- **前端**在「通用与外观」加一个可选开关；`UpdateSettings` 在该键变更时调用 `SetLaunchAtLogin`。

## 2. macOS 选型与取舍

App 最低系统为 **macOS 14.0**（`scripts/package-macos.sh:39` `MACOS_MIN_VERSION=14.0`），
因此 SMAppService（macOS 13 引入）在支持范围内始终可用，无需 <13 回退路径。

| 候选 | 结论 | 理由 |
|---|---|---|
| **`SMAppService.mainApp`** | **选定** | macOS 13+ 官方 API，注册主 App 自身为登录项，无需 helper bundle 或 plist，直接 `register` / `unregister` / `status` 三个调用即可。语义正好是"登录时打开这个 App"。 |
| `SMLoginItemSetEnabled` | 否决 | macOS 13 起废弃；需要单独打包 login-item helper，复杂且不受长期支持。 |
| `LSSharedFileList` | 否决 | macOS 10.11 起废弃的半私有 API，行为不稳定。 |
| `SMAppService.agent`（LaunchAgent plist） | 否决（本次） | 面向后台 agent / daemon，以 launchd 常驻语义拉起，`KeepAlive` 等行为与 Daygo 自有的
[生命周期 / 软退出模型](lifecycle-quit-model.md) 冲突。若未来要"登录即无窗后台常驻"再单独评估。 |

## 3. macOS 承载机制（官方确认）

来源：Apple ServiceManagement 文档与 SMAppService 用法（2026-09-22 核对）。

- **启用**：`try SMAppService.mainApp.register()`。首次注册系统会弹一条「已添加登录项 / Login Item
  Added」通知，属正常行为。
- **停用**：`try SMAppService.mainApp.unregister()`。
- **查询**：`SMAppService.mainApp.status`，取值：
  - `.enabled` → 已开启，下次登录会启动；
  - `.requiresApproval` → 用户在「系统设置 › 登录项」里手动关掉了，需用户重新批准，
    此时仅 `register()` 不足以生效，应把用户引导到登录项面板；
  - `.notRegistered` / `.notFound` → 未开启。
- **必须是正确打包并签名的 `.app`**：从非 bundle 或 ad-hoc 上下文调用 `register()` 会得到
  `SMAppServiceErrorDomain Code=1 "Operation not permitted"`。`wails dev` 与未签名开发构建下
  预期失败——这与 [macOS 签名身份决策](delivery-macos-signing-identity.md) 绑定，属 **G-native** 门禁。
- **深链已就绪**：`PaneLoginItems` 已映射到
  `x-apple.systempreferences:com.apple.LoginItems-Settings.extension`
  （[`SystemABI.swift`](../../native/darwin/Sources/SystemABI.swift) `dg_open_system_settings`），
  `.requiresApproval` 时可直接复用它跳转。

## 4. 状态映射与对账（端口是 bool）

SMAppService 有四态，端口只暴露 bool，映射约定：

- `LaunchAtLogin()`：读 `status`，`.enabled` → `true`，其余 → `false`。
- `SetLaunchAtLogin(true)`：`register()`；若注册后 `status` 仍为 `.requiresApproval`，
  交由前端引导用户到登录项面板重新批准（不把它当作端口错误）。
- `SetLaunchAtLogin(false)`：`unregister()`；已停用视为成功。

**真相源**：DB 里持久化的 `system.launchAtLogin`（默认 `false`）是用户意图，OS 的
`status` 是实际状态，两者可能因用户在系统设置里手动改动而分叉。约定：

- 写：`UpdateSettings` 检测到 `launchAtLogin` 变更时调用 `SetLaunchAtLogin`，与 recorder 转发
  （`api_settings.go`）同处；失败经 `apperr` 映射，不静默吞掉。
- 读 / 显示：设置页加载时以 `LaunchAtLogin()`（读 OS `status`）对账开关显示态，避免显示 DB 值
  而 OS 已被用户改动。具体是复用现有绑定回读还是新增只读查询，实现时定，不提前过度设计。

## 5. 实现分片（待实现，本次仅方案）

1. **C ABI**（`native/include/daygo_system.h`，minor 2→3，向后兼容新增函数）：
   `dg_launch_at_login_query()` → 状态枚举（`NOT_REGISTERED` / `ENABLED` / `REQUIRES_APPROVAL` /
   `NOT_FOUND` / `UNSUPPORTED`）；`dg_launch_at_login_set(uint32_t enabled)` → 0 或负值。
2. **Swift**（[`SystemABI.swift`](../../native/darwin/Sources/SystemABI.swift)）：`@_cdecl` 实现，
   `if #available(macOS 13, *)` 用 SMAppService，否则返回 `UNSUPPORTED`；register / unregister 的
   `throws` 映射为负返回码。
3. **cgo 桥接**（[`system_bridge_darwin.go`](../../internal/platform/darwin/system_bridge_darwin.go)）
   映射枚举；[`system_bridge_unavailable_darwin.go`](../../internal/platform/darwin/system_bridge_unavailable_darwin.go)
   无 cgo 时返回错误。
4. **darwin 端口**（[`system.go`](../../internal/platform/darwin/system.go)）：把现有两个 no-op 桩
   （`LaunchAtLogin` / `SetLaunchAtLogin`）改为转调桥接。
5. **绑定接线**（`internal/app/api_settings.go`）：`UpdateSettings` 在 `launchAtLogin` 变更时调用
   `system.SetLaunchAtLogin`，错误经 `apperr`。
6. **fake**（[`fake/system.go`](../../internal/platform/fake/system.go)）：加记录字段，供 app 层断言接线。
7. **前端**：`frontend/src/api/dto.ts` 补 `launchAtLogin`；`AppearanceSection.vue`（通用与外观）加一行
   `SwitchControl`，用 `useSettingsSection` 的 `persist({ launchAtLogin })`；`.requiresApproval` 时用
   `OpenSystemSettings(PaneLoginItems)` 引导。
8. **i18n**：`general.launchAtLogin` / `launchAtLoginHint` 在 `zh-CN` 与 `en` 两侧（zh-CN 为准）。
9. **测试**：fake 契约断言 set / query；app 层 `UpdateSettings` 接线测试；Windows 现有 Run 键行为保留。

## 6. 未验证与门禁

- **真机验收（G-host / G-native）**：签名 `.app` 下 `register()` 生效、下次登录真的自启、系统「已添加
  登录项」通知、`.requiresApproval` 引导有效性，都须在真实 macOS 观察；dev / ad-hoc 下预期失败。
- **与生命周期的关系**：SMAppService 登录时拉起 App 主可执行，启动后是否建窗口 / 后台常驻沿用
  现有启动行为（[生命周期决策](lifecycle-quit-model.md)），若要"登录后静默常驻"需另行决定。
- **Windows**：Run 键已实现；真实签名分发身份下的自启行为仍属 [Windows 发布范围](recording-screen-capture-windows.md)（WD 矩阵）。
- **Linux**：仍 unsupported，跟随 [Linux 决策](recording-screen-capture-linux.md)。
