# 生命周期退出模型：软退出留后台，仅状态栏真退出

> **状态：已落盘的纯 Go 拦截 + 原生激活策略切片；真机观感（Dock 图标消失、菜单栏留存、长期后台存活）
> 仍受 G-host 门禁约束，未验收前不得宣称「已支持」。** 本文记录 Daygo 采用哪种退出语义、
> 依赖 Wails 的哪条机制承载，以及代码已经做了什么。行为对照表以
> [架构 §2.6.2](../02-architecture.md#262-关闭) 为准。

## 1. 决策

参考 legacy 原生 App（`legacy/dayflow`）的做法，把「退出」拆成两类：

- **软退出**：Cmd+Q、Dock 右键「退出」、macOS App 菜单「退出」——不终止进程，只隐藏窗口
  并把激活策略切到 accessory（摘掉 Dock 图标），录制在后台继续。
- **真退出**：只有状态栏菜单的「退出」会真正终止进程（收尾当前分段后停止捕获）。

这与常驻后台 Agent 的定位一致：退出 UI 不等于用户要求停止录制。此前
[架构 §2.6.2](../02-architecture.md#262-关闭) 把 Cmd+Q 记为「退出」，与该定位相悖，已随本切片改正。

legacy 用原生 `applicationShouldTerminate` + `allowTermination` 开关实现同一语义；Daygo 不复制
它的原生 AppDelegate，而是用 Wails 已经提供的等价挂钩承载（见 §2）。

## 2. Wails 承载机制（v2.15.0，读源码确认）

Daygo 的宿主是 Wails v2，不是原生 AppKit App。Wails 在 macOS 上的退出路由如下：

```text
关闭窗口按钮  → WindowDelegate.windowShouldClose
                └─ HideWindowOnClose=true → [NSApp hide] 并 return false（不经过 OnBeforeClose）
Cmd+Q / Dock「退出」/ App 菜单「退出」
              → AppDelegate.applicationShouldTerminate
                └─ 永远返回 NSTerminateCancel，转发内部 "Q" 消息
runtime.Quit(ctx)  → frontend.Quit()
"Q" 消息 / runtime.Quit → dispatcher case 'Q' → frontend.Quit()
                          └─ 若 OnBeforeClose 返回 false → mainWindow.Quit()（真退出）
                             若 OnBeforeClose 返回 true  → 阻止，进程存活
```

两个关键事实决定了实现形态：

1. **每一条退出路径都经过 `OnBeforeClose`**，包括状态栏 Quit 调用的 `runtime.Quit`。因此不能靠
   「只有状态栏调用 runtime.Quit」来区分，必须显式给出意图。
2. **`OnBeforeClose` 返回 `true` = 阻止退出，返回 `false` = 放行退出。**
3. 关窗按钮因 `HideWindowOnClose: true` 被 Wails 提前短路成隐藏窗口，不触发 `OnBeforeClose`。

## 3. 实现

Go 侧（`internal/app`，可在 `CGO_ENABLED=0` / Linux 下测试）：

- `Backend.allowQuit`（`atomic.Bool`，默认 false）+ `requestQuit()` / `quitAllowed()`。
- `OnBeforeClose`：`quitAllowed()` 为真则返回 false 放行；否则 `runtime.WindowHide` +
  `enterBackground`（切 accessory）后返回 true 阻止。
- 状态栏 `"open"` 与 macOS 应用重新激活（包括用户点击保留在 Dock 的 Daygo 图标）共用
  `openWindow`：先 `exitBackground`（切回 regular），再 `runtime.WindowShow` + `runtime.Show`；
  `"quit"` 先 `requestQuit()` 再 `runtime.Quit`。Wails v2.15.0 没有 reopen 回调，因此原生
  `System` 观察 `NSApplication.didBecomeActiveNotification`，经 `System.Events` 把激活意图交给
  app 层；平台层本身不操作 Wails 窗口。
- `enterBackground` / `exitBackground` 只走 `platform.System.SetActivationPolicy`，不碰系统 API；
  `system == nil`（headless）时是 no-op。

激活策略原生实现（此前为空桩，属 [09 §9.8 待定设计第 5 项](../09-roadmap.md#98-待定设计清单)，本切片落地）：

- ABI：[`native/include/daygo_system.h`](../../native/include/daygo_system.h) 新增
  `dg_activation_policy_set(uint32_t policy)` 与 `DG_ACTIVATION_REGULAR/ACCESSORY/PROHIBITED`。
- 实现：`native/darwin/Sources/SystemABI.swift` 在主线程调用 `NSApp.setActivationPolicy`。
- 桥接：`system_bridge_darwin.go`（cgo）映射端口枚举到 ABI；`system_bridge_unavailable_darwin.go`
  在无 cgo 时返回错误。`darwin.System.SetActivationPolicy` 转调之，Windows / fake 保持 no-op。

## 4. 未验证与门禁

- 纯 Go 拦截与激活策略切换有单元测试（`internal/app/lifecycle_test.go` 用 fake System 断言
  软退出切 accessory、恢复切 regular）。
- **真机观感未验收**：Dock 点击恢复窗口的实现已有 Go 路由测试，但
  `didBecomeActiveNotification` 的触发观感、Dock 图标消失/恢复、菜单栏项在 accessory 下可用、关窗后长期后台存活与继续
  离散捕获，均属 G-host 硬门禁范围，需真实 macOS 观察（见
  [架构 §2.6.3](../02-architecture.md#263-后台-agent-语义)、[风险 C-1](../10-risks.md#c-1宿主无法承载后台-agent)）。
- 系统关机（`willPowerOff`）目前不特殊处理：`applicationShouldTerminate` 返回 Cancel 后由系统
  超时强杀。若观察到关机收尾不足，再补一条系统关机事件让软退出让路（后续切片）。
