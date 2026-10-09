# Windows 提醒：C++/WinRT 桌面注册与系统一次性排程

> 决策日期：2026-10-09。实现与验证进度见 [daily 执行册](../modules/daily.md)。

## 决策与范围

复用 `daygo_windows_native.dll`，新增版本化通知 C ABI；Go 的 Windows System 通过绝对路径加载
同目录 DLL。使用 Windows 10+ 的 `Windows.UI.Notifications`，不新增 SDK 运行时、PowerShell
子进程或产品定时器。重复与墙钟时刻仍由 [原通知决策](notifications-journal-reminder.md) 的 Go
调度器拥有。立即通知走 `Show`；未来时刻走 `ScheduledToastNotification` / `AddToSchedule`。

桌面身份采用微软 C++/WinRT 兼容示例的当前用户注册：
`HKCU\Software\Classes\AppUserModelId\io.github.jwz-git.Daygo` 的固定显示名与 CustomActivator，
以及固定 CLSID `{EF69D18B-ED0F-4B85-9FDB-1A9458027B0C}` 的 `LocalServer32`（引用加引号的当前
EXE，附 `--daygo-toast-activated`）。不依赖 machine 快捷方式的写权限，也不改安装范围。
注册在 System 初始化中完成，不申请授权；失败时不广告 `notifications`，其余 System 能力继续。
提权运行不广告通知能力（Windows 通知不支持管理员发送者），需按普通用户身份启动。

COM class factory 在适配器拥有的 MTA 线程上注册；退出时撤销 class object 并 join 线程，
不删除已排通知或持久身份，以便已交给 OS 的下一次提醒仍可投递。激活回调仅确认收到，
与 macOS 当前「点击只关闭通知」同范围；点击恢复现有窗口与精确打开日记不是本切片承诺。
进程未运行时 Windows 可以启动注册的 EXE，但不把这一行为当成已验收的唤回闭环。

## 契约与失败条件

- 能力检查为已成功初始化的缓存状态；不查授权、不弹窗、不重复注册。`ToastNotifier.Setting`
  的 Enabled 映射 granted，其余系统 / 用户禁用映射 denied。投递前再次查询，拒绝不伪报成功。
- 原生只接收 ID、标题、正文与绝对投递时刻；不读设置、数据库、屏幕数据，不联网。
  文案仍由九语言 `NativeUiLabelsDTO` 下发；XML 用 DOM 文本节点构造，禁止文本成为 XML 指令。
- Windows 的 scheduled ID 上限为 16 字符。用 SHA-256 前 16 个十六进制字符作 token，完整 ID
  另存 XML launch 属性；改写 / 取消先核对完整 ID，碰撞报错而非误删另一通知。
  相同 ID 先移除旧排程再添加新排程；失败保持 Go 未排状态，下次对账重试。
- 取消同时移除尚未投递的排程与同 ID 的通知历史；不清空其它计划的通知。关闭适配器不等于取消。
- 缺 DLL、旧 DLL 缺入口或原生初始化失败均不可用；权限拒绝保留实现能力。错误只带固定操作与
  数字状态 / HRESULT，不包含标题、正文、用户路径或完整原生异常文本。
- Windows 的定时通知只有五分钟送达窗口；关机 / 休眠超出窗口可能丢弃，不承诺补发。
  进程关闭后只保留已经排好的下一次通知，每日下一次仍需 Go 重新运行对账。

## 验证与回退

先写匿名 Go 驱动夹具与 C++ 边界夹具，再实现。C++ 夹具使用独立随机 AUMID / CLSID，
验证 XML、输入 / 版本拒绝、注册与撤销 COM；系统允许时排 30 分钟后的匿名通知，
验证替换、碰撞拒绝和取消。系统禁用时明确跳过排程执行、验证拒绝，不弹真实通知、不使用 Daygo 身份或数据。
Windows CI 编译 DLL 与夹具，不能替代真实 Windows 桌面的送达验收。
若 runner 为提权进程，验证能力不可用与 ABI / XML 后明确跳过注册 / 权限 / 排程；
不将该跳过记为通知送达或原生排程通过。非提权主机仍要求注册、查询及排程契约成功。

真实门禁属于 G-native：安装后开启提醒、改时刻、关闭、系统禁用 / 恢复、重启读回、
关窗后到点投递、进程退出后已排提醒、升级后身份保持分别记录；点击唤回仍是范围外。
回退为关闭提醒并等对账取消，再撤销本实现；保留日记、设置与数据库，不做 schema 迁移。
当前用户注册在卸载时清理；machine 卸载无法遍历其它用户的通知状态，单独保留为真机验证限制。

## 官方依据

- [微软桌面 C++/WinRT 兼容注册示例](https://github.com/WindowsNotifications/desktop-toasts/blob/master/CPP-WINRT/DesktopToastsCppWinRtApp/DesktopNotificationManagerCompat.cpp)：当前用户 AppUserModelId 与 COM 注册。
- [计划 Toast](https://learn.microsoft.com/zh-cn/windows/apps/design/shell/tiles-and-notifications/scheduled-toast)：OS 排程、取消与五分钟送达窗口。
- [桌面通知](https://learn.microsoft.com/en-us/windows/win32/shell/quickstart-sending-desktop-toast)：显式 AUMID 的 ToastNotifier。
- [Windows 通知限制](https://learn.microsoft.com/en-ie/windows/apps/develop/notifications/app-notifications/toast-notifications-overview)：提权应用不能发送 / 接收通知。
