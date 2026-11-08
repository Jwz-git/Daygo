# 生命周期退出模型：关窗留后台，退出结束进程

> **最新验收确认（2026-10-13，`test` / `bf49b34` 工作树）**：本文涉及的已实现能力及后续增量，
> 此前仍待验收的项目均由用户确认已验收，未附逐项运行记录；未实现项与待定设计保留。
> 下文旧日期的失败 / 跳过 / 未运行结果是历史记录，不倒填为通过；后续新改动需独立验收。
> 正式身份与签名材料的证据边界、统一范围见 [09 §9.1.1](../09-roadmap.md#911-本轮验收记录与证据边界)。

## 1. 决策

2026-11-08 用户调整了 macOS 语义。点击主窗口红叉后隐藏窗口并在后台继续录制，
菜单栏可重开时切换 accessory，移除 Dock 运行标记。菜单栏入口不可用时保留 Dock 作为恢复入口。
Dock「退出」、应用菜单「退出 Daygo」（Cmd+Q）及状态栏「停止录制并退出 Daygo」收尾后终止进程。
普通退出收尾失败时默认保留应用，可明确选择仍然退出。授权重启、更新重启与系统关机
分别保留各自的意图，授权重启由 Daygo 拉起新实例，更新重启由更新器拉起。
行为对照见 [架构 §2.6.2](../02-architecture.md#262-关闭)。

## 2. Wails 承载机制（v2.15.0）

- Wails 的 `WindowDelegate.windowShouldClose` 在 `HideWindowOnClose=true` 时调用
  `NSApp.hide`。macOS 适配器在 AppKit 完成启动后给主窗口红叉安装独立 target，执行同样的
  应用隐藏，并通过 System ABI 上送 `window_close_requested`。普通 Cmd+H 仍只有应用隐藏事件。
- Dock / 系统发起的退出由 Wails 的 `AppDelegate.applicationShouldTerminate` 转成内部 `Q` 消息，
  应用菜单的 Quit 项和 `runtime.Quit` 也进入 `Frontend.Quit`。Go 侧 `OnBeforeClose`
  返回 false 时放行，返回 true 时取消退出。未显式覆盖且已武装的授权重启走
  [授权流程](recording-screen-recording-permission.md)，状态栏、更新、信号和关机可覆盖该意图。
- `enterBackground` 在收到红叉事件时记录恢复意图并通过 `platform.System` 请求 accessory，
  与已保存的 Dock 偏好独立。重开清除恢复标记、重试并应用保存策略，再用 `runtime.Show` unhide；
  不再 order out / order front 窗口，避免与 Mission Control 的激活动画冲突。
- 菜单栏显式重开走同一恢复路径；`didBecomeActive` 只有在后台恢复标记存在时才调用它。
  普通 Dock 点击、Cmd+Tab 和 Mission Control 激活由 AppKit 处理，避免重入。
  激活策略仅缓存成功值，状态栏不可用时保留 regular。Windows / Linux 保留原退出路由。

## 3. 验证

新增的宿主增量及回退见 [recording 执行册](../modules/recording.md#2026-11-08-macos-关窗与退出回归)。
2026-09-22 与 2026-10-13 用户确认保留为历史证据，10 分钟持续离散捕获门禁见
[架构 §2.6.3](../02-architecture.md#263-后台-agent-语义)。

## 媒体可见性（2026-09-26）

当时保留原有的软退出模型。`GetUIVisibility` 与 `ui:visibility-changed` 使用应用隐藏和
窗口 order-out 两个独立状态：关窗按钮的 `NSApp hide` 由 native System 的 didHide / didUnhide
通知接入；软退出与显式重开在 app 层更新窗口状态。应用 unhide 不会自动清除 order-out，
普通激活也不直接改可见性。UI 只暂停媒体与移除图片，不卸载页面，不改变 recorder / 分析任务。
测试已覆盖两个来源的交错顺序；本轮已实现可见性 / 捕获连续性于 09-26 用户确认已验收，未附十分钟逐项记录。

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
