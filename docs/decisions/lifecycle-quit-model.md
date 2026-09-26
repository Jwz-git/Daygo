# 生命周期退出模型：软退出留后台，仅状态栏真退出

> **状态：已落盘的纯 Go 拦截 + 原生激活策略切片；真机观感（Dock 图标消失、菜单栏留存、长期后台存活）
> 所依赖的 G-host 门禁已于 2026-09-22 经用户实测验收（无逐项运行记录）。** 本文记录 Daygo 采用哪种退出语义、
> 依赖 Wails 的哪条机制承载，以及代码已经做了什么。行为对照表以
> [架构 §2.6.2](../02-architecture.md#262-关闭) 为准。

## 1. 决策

参考 legacy 原生 App（`legacy/dayflow`）的做法，把「退出」拆成两类：

- **软退出**：Cmd+Q、Dock 右键「退出」、macOS App 菜单「退出」——不终止进程，只隐藏窗口
  并请求 accessory（摘掉 Dock 图标），录制在后台继续；状态栏入口不可用时保留 Dock。
- **真退出**：普通用户主动终止进程通过状态栏“停止录制并退出 Daygo”；更新、系统关机与
  信号另有独立终止意图。先收尾当前分段后停止捕获，普通退出收尾失败时默认保留应用。
- **授权重启**：授权流程内武装后的退出（含 macOS 授权后自弹的「退出并重开」、引导层按钮）——完全退出
  **并自动重启**，让新的屏幕录制授权生效。语义与实现见
  [屏幕录制授权 §1a](recording-screen-recording-permission.md)；它是软退出之外唯一会自动拉起新实例的
  退出路径，而更新重启由 Sparkle 拉起，两者互不改写。

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
- `OnBeforeClose`：`quitAllowed()` 为真则先等待收尾，成功放行；普通退出失败时默认取消并
  提供“保留应用 / 仍然退出”选择。SIGTERM / 系统关机不阻止终止，**不**自重启；
  否则若「授权重启」已武装（`pendingPermissionRestart`），走 `beginPermissionRestart()`（收尾分段 → 调度
  自重启）后返回 false 放行真退出；否则 `runtime.WindowHide` + `enterBackground`（切 accessory）后返回 true
  阻止。授权重启路径的细节见 [屏幕录制授权 §1a](recording-screen-recording-permission.md)。
- 状态栏 `"open"` 走 `showWindow`：先 `exitBackground`（按 Dock 偏好应用策略），再 `runtime.Show`
  （unhide 应用）**最后** `runtime.WindowShow`（order front + activate）。顺序不能颠倒：软退出
  用的是 `runtime.WindowHide`（`orderOut`），unhide 应用不会把它还原，先 order front 再 unhide
  会让窗口只闪一帧。
- macOS 应用重新激活（`NSApplication.didBecomeActiveNotification`）走 `restoreOnActivation`，
  **只在 `Backend.backgrounded` 为真时**执行上面的 `showWindow`；`"quit"` 先 `requestQuit()`
  再 `runtime.Quit`。Wails v2.15.0 没有 reopen 回调，所以原生 `System` 观察该通知，经
  `System.Events` 把激活意图交给 app 层；平台层本身不操作 Wails 窗口。
- 两个状态转换都是**幂等**的：成功应用的同一策略不重复下发；失败策略不记录成功，后续显式
  重开重试。`Backend.backgrounded` 独立记录窗口被 order out，即使策略失败也置位。

**为什么激活路径必须设这道闸**（此前缺失，表现为窗口闪一下就消失）：

1. `didBecomeActive` 是**泛化的激活信号**——Dock 点击、Cmd+Tab、调度中心、以及
   `runtime.Show` / `runtime.WindowShow` 自己调用的 `activateIgnoringOtherApps` 都会触发它。
   在应用已经处于前台时再执行一次 `WindowShow` + `Show`，等于在系统正在完成激活（尤其是调度中心
   的过渡动画）时抢占窗口顺序，窗口会被随即丢弃。
2. `runtime.WindowShow` 与 `runtime.Show` 各自的 `activateIgnoringOtherApps` 会再次触发
   `didBecomeActive`，形成一次自触发重入。跳过前台激活即断掉这条回路。
3. 关窗（`HideWindowOnClose`）在 Wails 里是 `[NSApp hide]`，**不** order out 窗口，也不经过
   `OnBeforeClose`；`[NSApp unhide]` 会连带把窗口带回来，因此前台激活什么都不做才是正确的。

因此 `backgrounded` 是“窗口被我们自己 order out”的判据，也是激活唯一需要撤销的状态；
它与 Dock 偏好独立。窗口可见时也可能因 `showDockIcon=false` 使用 accessory。

`enterBackground` / `exitBackground` 只走 `platform.System.SetActivationPolicy`，不碰系统 API；
`system == nil`（headless）时是 no-op。

激活策略原生实现（此前为空桩，属 [09 §9.8 待定设计第 5 项](../09-roadmap.md#98-待定设计清单)，本切片落地）：

- ABI：[`native/include/daygo_system.h`](../../native/include/daygo_system.h) 新增
  `dg_activation_policy_set(uint32_t policy)` 与 `DG_ACTIVATION_REGULAR/ACCESSORY/PROHIBITED`。
- 实现：`native/darwin/Sources/SystemABI.swift` 在主线程调用 `NSApp.setActivationPolicy`。
- 桥接：`system_bridge_darwin.go`（cgo）映射端口枚举到 ABI；`system_bridge_unavailable_darwin.go`
  在无 cgo 时返回错误。`darwin.System.SetActivationPolicy` 转调之，Windows / fake 保持 no-op。

## 4. 未验证与门禁

- 纯 Go 拦截、激活策略切换与激活闸门有单元测试（`internal/app/lifecycle_test.go` 用 fake System
  断言软退出切 accessory、恢复切 regular、重复转换不重推策略、前台激活不重开窗口）。
- **真机观感：2026-09-22 用户确认已验收（无逐项记录）**：Dock 点击恢复窗口的实现已有 Go 路由测试，但
  `didBecomeActiveNotification` 的触发观感、调度中心激活后窗口是否稳定留存、Dock 图标消失/恢复、
  菜单栏项在 accessory 下可用、关窗后长期后台存活与继续
  离散捕获，均属 G-host 硬门禁范围，已随 2026-09-22 用户实测验收（无逐项运行记录）覆盖（见
  [架构 §2.6.3](../02-architecture.md#263-后台-agent-语义)、[风险 C-1](../10-risks.md#c-1宿主无法承载后台-agent)）。
- 系统关机 / 注销已通过 `willPowerOff` 独立意图接入，详见下文宿主控制加固。此前缺少该路由；
  原文关于「超时强杀」没有逐项实机记录，不能当作已验证事实。

## 媒体可见性（2026-09-26）

退出 / 激活模型保持上述行为。`GetUIVisibility` 与 `ui:visibility-changed` 使用应用隐藏和
窗口 order-out 两个独立状态：关窗按钮的 `NSApp hide` 由 native System 的 didHide / didUnhide
通知接入；软退出与显式重开在 app 层更新窗口状态。应用 unhide 不会自动清除 order-out，
普通激活也不直接改可见性。UI 只暂停媒体与移除图片，不卸载页面，不改变 recorder / 分析任务。
测试已覆盖两个来源的交错顺序；本轮真实关窗 10 分钟持续捕获仍待验证。

## 宿主控制加固（2026-09-26）

本切片保留软退出产品语义，并固定以下失败条件：

- 移除 Dock 前必须确认状态栏入口已经安装且可见；不可用时软退出只隐藏窗口，保留 regular
  策略与后台恢复闸门，Dock 激活仍可带回窗口。
- 激活策略 ABI 等待主线程执行并返回 AppKit 的实际成功值；状态栏重绘仍异步，不在 recorder
  回调中同步等待主线程。策略失败不记作已应用，允许下次显式动作重试。
- 普通真退出先等待 recorder 收尾；失败时恢复窗口，默认“保留应用”，只有明确选择“仍然退出”
  才忽略本次失败继续退出；提示当前分段可能丢失，不能打印原始错误或路径。SIGTERM / 系统关机、
  注销尽力收尾，不阻止系统终止，也不触发授权自重启。
- macOS 使用 NSWorkspace 自身 notificationCenter 的 willPowerOffNotification 上送独立
  system_shutdown 事件。该终态事件在缓冲满时优先入队；关闭事件源后晚到回调不再入队。

实现与验证证据见 recording 执行册；纯 Go 与原生夹具不替代真实关机、注销及关窗十分钟门禁。

## 菜单与 Dock 偏好（2026-09-26）

`showDockIcon` 在启动与设置提交后应用；显式重开按已保存偏好恢复，不强制 regular。
移除 Dock 仍须状态栏入口可用，偏好关闭但入口丢失时保留 regular。偏好保存与原生应用失败
分别报告，重开允许重试。macOS 主应用菜单 Cmd+Q 标题改为“留在后台继续记录”，状态栏
真退出标题明确为“停止录制并退出 Daygo”；20 项主菜单文案随应用语言切换，既有快捷键保留。

状态栏展示只读、不可用、启动、截图重试和系统暂停，定时暂停显示本地恢复时刻；锁屏时
不可手动恢复。普通操作失败采用非阻塞原生提示，不占用 System 事件泵；只有普通退出收尾
失败才在 Wails 的独立退出回调等待选择。相同状态快照不重建菜单，避免每帧回调刷新菜单结构。
