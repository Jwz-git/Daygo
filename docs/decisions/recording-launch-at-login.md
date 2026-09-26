# recording 开机自启：macOS 用 SMAppService.mainApp，Windows 用 Run 键

> **最新功能验收（2026-09-26）**：本文涉及的所有已实现能力、长期观察与现有身份下真实安装升级，
> 均按本次用户确认记为已验收，未附逐项运行记录；未实现项、待定设计与正式证书缺失保留。
> 下文旧日期的失败 / 跳过 / 未运行结果是历史记录，不倒填为通过；统一范围见
> [09 §9.1.1](../09-roadmap.md#911-本轮验收记录与证据边界)。

> **状态：已决定并实现。** macOS SMAppService、Windows Run 键、设置 patch 消费者与前端开关
> 均已落盘；已实现功能于 2026-09-26 用户确认已验收，未附逐项运行记录。正式证书材料仍缺，
> 保存用户意图不等于读取 OS 实际状态；以下单列尚未实现的对账与授权提示。
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
- `SetLaunchAtLogin(true)`：调用 `register()`，注册异常映射为桥接错误；当前不检查注册后的
  `.requiresApproval`，专门引导属于尚未实现项。
- `SetLaunchAtLogin(false)`：`unregister()`；已停用视为成功。

**当前实现的真相源与边界**：DB 的 `system.launchAtLogin`（默认 `false`）保存用户意图，
OS 的 `status` 表示实际注册。`UpdateSettings` 提交成功后，仅当此键变更时尽力调用
`SetLaunchAtLogin`；失败只记录日志，不撤销 DB 保存，也不将 OS 错误返回为设置保存失败。
`GetSettings` 读回持久化意图，不会调用 OS 查询覆盖该值。

**尚未实现**：设置页加载时与 OS 实际注册状态对账；`.requiresApproval` 的专门引导。
当前 bool 查询把非 enabled 映射为 false，不能区分“未注册”和“需要批准”。
这些缺口不因已实现开关验收而自动成为已交付能力。

## 5. 已实现切片与剩余工作

| 切片 | 当前落点 | 状态 |
|---|---|---|
| C ABI / Swift | `daygo_system.h` 的 `dg_launch_at_login_query` / `dg_launch_at_login_set`；`SystemABI.swift` 的 SMAppService 调用 | 已实现；System ABI 当前为 1.6，不再执行旧 minor 2→3 计划 |
| Go 桥 / 端口 | `system_bridge_darwin.go`、无 cgo 不可用桥、`darwin.System`；Windows Run 键实现保留 | 已实现 |
| 设置消费者 | `internal/app/api_settings.go` 的 `applyLaunchAtLogin`，提交后按改动键尽力应用 | 已实现；OS 失败不回滚保存 |
| UI / i18n | `AppearanceSection.vue` 开关、设置 DTO / patch、九语言文案 | 已实现；当前展示 DB 意图 |
| OS 对账 / 需要批准引导 | 加载时查询实际状态、专门呈现 requiresApproval | 未实现；新增绑定或 DTO 前先补 05 与双侧夹具 |

## 6. 未验证与门禁

- **真机验收（G-host / G-native）**：签名 `.app` 下 `register()` 生效、下次登录真的自启、系统「已添加
  登录项」通知、`.requiresApproval` 引导有效性，都须在真实 macOS 观察；dev / ad-hoc 下预期失败。
- **与生命周期的关系**：SMAppService 登录时拉起 App 主可执行，启动后是否建窗口 / 后台常驻沿用
  现有启动行为（[生命周期决策](lifecycle-quit-model.md)），若要"登录后静默常驻"需另行决定。
- **Windows**：Run 键已实现；真实签名分发身份下的自启行为仍属 [Windows 发布范围](recording-screen-capture-windows.md)（WD 矩阵）。
- **Linux**：仍 unsupported，跟随 [Linux 决策](recording-screen-capture-linux.md)。
