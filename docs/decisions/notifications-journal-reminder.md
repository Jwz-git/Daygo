# notifications 日记提醒：Go 拥有重复、一次性原生通知、墙钟时刻

> **状态：已决定；Go 调度、前端与 macOS / Windows 原生投递已实现，真实通知投递待验收。**
> macOS `UNUserNotificationCenter` 已随 `4dac8b7` 合入 `test`；Windows C++/WinRT toast 于 10-09
> 落盘，见 [Windows 通知决策](notifications-windows-toast.md)。缺对应原生运行环境 / DLL 时仍返回
> `platform.ErrCapabilityUnavailable`。2026-10-07 起
> `NotificationAvailability` 无授权副作用地报告投递实现能力，绑定以 `notifications`
> feature 下发；设置与计划编辑器在能力缺失时禁用提醒并显示说明，保留已有提醒偏好。
> 支持投递不等于获准投递，授权仍由平台投递时校验。调度与 fake 证据不替代 §6 真机门禁。

## 1. 决策

「日记提醒」指用户设定的一个**本地墙钟时刻**（`HH:mm`），到点由系统弹一条通知，提醒写当天日记。
它是 [09 §9.3](../09-roadmap.md#93-能力接入表) 中「notifications」能力（负责模块 daily），
也是 [09 §9.8](../09-roadmap.md#98-待定设计清单) 需逐项落地的一项；[daily 执行册](../modules/daily.md)
§「实验与失败条件」已列出提醒的验收条件（默认关闭、开启 / 改时刻 / 取消 / 拒绝权限、
不循环申请权限、重复提醒失败）。

四条核心决定：

1. **原生端口是一次性通知，重复由 Go 拥有。** [`System.ScheduleNotification`](../../internal/platform/ports.go)
   接受 `Notification{ID, Title, Body, DeliverAt *time.Time}`，没有重复字段
   （[`types.go`](../../internal/platform/types.go)）。因此「每天 18:00」不是交给 OS 的
   重复规则，而是 Go 每次只排下一次，投递或时刻变更后再排下一次。接口不新增重复字段。
2. **调度用墙钟时刻，不用 4 点逻辑日。** 提醒时刻是用户对钟表的期望，与「一天从凌晨 4 点
   开始」的业务日期边界无关。`nextReminderAt` 用本地 `time.Location` 逐日构造，
   DST 与半小时 / 45 分钟时区由 `time.Date` 的本地语义承担，不做 `Truncate(24h)` 之类的
   时长算术。
3. **只有一个调度器，且只在读写 / 捕获所有者实例上运行。** 与 `runStandupBackfill` 同构：
   ctx 持有的 goroutine，由 app.go 的 RW-only 启动块拉起，用可注入的 `b.clock`。
   只读实例不排任何通知（它也不该持有系统通知身份）。
4. **文案经 `NativeUiLabelsDTO` 下发，适配层不持有 locale。** 通知标题 / 正文是 webview
   之外的原生面，沿用既有 seam（05 §5.5.1）：前端在语言变化时推送翻译后的
   `journalReminderTitle` / `journalReminderBody`，app 层转发，适配层无语言状态。

## 2. 为什么重复归 Go

| 候选 | 结论 | 理由 |
|---|---|---|
| **Go 每次排下一次（一次性端口）** | **选定** | 端口已是一次性；Go 是唯一写入方与唯一进程级状态持有者，能在时刻变更、关闭、实例易主时精确重排 / 取消。 |
| 端口加 `Repeat` / cron 字段，交给 OS | 否决 | 会把「用户改了时刻」的语义压进平台层；适配层将被迫理解设置与冲突规则，违反「适配层不读设置为自己决策」。且 macOS `UNCalendarNotificationTrigger` 与 Windows toast 的重复语义不一致，跨平台行为无法用一套夹具证明。 |
| 用定时器在进程内 `sleep` 到点再投递 | 否决 | 关机 / 睡眠期间不会触发；常驻 agent 重启后时刻丢失。必须把 `DeliverAt` 交给系统，由系统在进程不在时仍投递。 |

## 3. 调度语义

- **稳定 ID。** 单条提醒固定 `ID = "journal-reminder"`，因此重排是「同 ID 覆盖」，
  取消是 `CancelNotifications(ctx, ["journal-reminder"])`。绝不产生累积的多条通知。
- **下一次时刻。** `nextReminderAt(now, "HH:mm", loc)`：取本地当天该 `HH:mm`；若已过去（或等于
  now），取次日同一 `HH:mm`。返回的 `time.Time` 带 `loc`，交给适配层时再取绝对时刻。
- **同步是幂等的对账，不是事件驱动的增量。** 调度器每次 tick 读当前设置快照，计算「期望状态」：

  | 设置 | 期望动作 |
  |---|---|
  | 关闭 | 进程首次对账或此前已排 → `CancelNotifications`；随后幂等无事 |
  | 开启，时刻未变且已排给同一 `deliverAt` | 无事 |
  | 开启，时刻变更 / 未排 / `deliverAt` 已过 | 覆盖排下一次 |

  为此进程内保存上次下发的 `(enabled, time, deliverAt, title, body)`；仅在期望与已下发不一致时
  调用平台端口。文案随语言变化也算「变更」，会重排（否则通知停在旧语言）。
- **权限。** 端口只有 [`NotificationsPermission(ctx)`](../../internal/platform/ports.go) 查询，
  没有独立的 request 方法（对比 `RequestScreenRecordingPermission`）。首次投递时由原生
  `UNUserNotificationCenter.requestAuthorization` 弹一次系统授权；**Go 不主动轮询申请**，
  避免 daily 执行册禁止的「循环申请权限」。未授权时平台端口返回错误 → 记日志、不重试轰炸，
  等用户在系统设置里开启后由下一次 tick 的对账自然恢复。
- **能力不可用 vs 权限 / 瞬时失败。** 端口返回 `platform.ErrCapabilityUnavailable`（尚未接入
  原生投递的平台或构建）视为「本平台不投递」：`journalReminderSync` 以 `errors.Is` 判定后
  静默跳过——不 arm、返回 nil、不每 tick 记日志，与「无 System」同类。其它错误（未授权、
  瞬时失败）仍返回 `apperr.NativeUnavailable`，记日志并在下一次 tick 重试。
- **设置 / 文案变更。** 提交提醒设置或下发原生文案后非阻塞唤醒现有 Go 对账循环；
  不等待五分钟 tick，不另起调度器。启动时若设置关闭也取消一次稳定 ID，以清理前进程遗留排程。
- **tick 间隔。** 复用较低频的轮询（与 `standupBackfillInterval` 同量级），因为对账只需覆盖
  「设置改了」「跨过当天时刻」两类变化；分钟级精度由 `DeliverAt` 交给系统保证，不靠 tick 命中。

## 4. 文案与定位（`NativeUiLabelsDTO`）

- 新增两个字段 `JournalReminderTitle` / `JournalReminderBody`，随既有 `SetNativeUiLabels`
  通路下发；`App.vue` 在 locale 变化时一并推送，`defaultNativeUiLabels` 提供 zh-CN 种子
  （默认界面语言，避免首帧空标题）。
- 文案进 9 语言 `native` 命名空间；`frontend/tests/i18nKeys.test.ts` 的 key 一致性与漏译检查覆盖。
- 通知正文**不含任何屏幕内容、窗口标题或用户数据**（07 §7）；只写「该写今天的日记了」这类
  固定引导语，与提醒的功能定位一致。

## 5. 已实现切片

| 切片 | 落点 | 状态 |
|---|---|---|
| 设置键 / 规范化 / patch / DTO | `internal/settings`、`api_settings.go`、`settings_dto.go` | 已实现（本轮之前即已就位） |
| Go 调度器 | `internal/app/journal_reminder.go` 的 `runJournalReminder` / `journalReminderSync` | 已实现 |
| fake 夹具 | `fake.System` 记录 `ScheduleNotification` / `CancelNotifications` 调用 | 已实现 |
| 设置 UI + i18n | 通用区开关 + 时刻输入，9 语言 | 已实现 |
| 文案通路 | `NativeUiLabelsDTO` 两个字段 + `native` 命名空间 + `App.vue` 推送 | 已实现 |
| **原生投递** | darwin `UNUserNotificationCenter`（`internal/platform/darwin/notifications_darwin.go`，cgo 内联 Objective-C，与已安装应用枚举同一形态，未进 Swift 静态库）；windows C++ toast | **darwin 已实现（`4dac8b7`）**；Windows C++/WinRT 桌面身份注册、立即 / 定时排程与取消已实现，见 [Windows 决策](notifications-windows-toast.md)；两平台真实送达待验收（§6） |

## 5.1 计划通知（2026-10-03 增补）

[plan](../modules/plan.md) 复用同一端口与「Go 拥有调度、适配层只投递」的分工：计划块开始通知以
`plan-start-<id>` 用 `DeliverAt` 交系统按时投递；分心提醒以 `plan-distraction-<id>` /
`plan-day-distraction-<day>` 立即投递（`DeliverAt` 为空）。macOS 实现：首次投递时
`requestAuthorization` 一次，本进程内不再追问；无 bundle（`go test`、裸二进制）返回
`ErrCapabilityUnavailable`；前台时由 delegate 仍显示横幅；定时用日历日期触发器，不受睡眠与改时钟漂移。
因 cgo 的 `#cgo CFLAGS` 在包内合并，本文件不开 ARC，以免改变同包其它 Objective-C 的编译方式。

## 6. 未验证与门禁

- **on-device 投递（G-native）**：`ScheduleNotification` 的真实实现、系统授权弹窗、
  `DeliverAt` 到点真的弹出、进程不在（软退出为后台）时仍投递、点击通知唤回窗口
  （`SystemEventData.NotificationID` 已预留：`EventNotificationClick`），都必须在真实
  macOS / Windows 观察。缺对应原生运行环境的构建仍返回 `platform.ErrCapabilityUnavailable`，
  调度器据此静默跳过；两平台投递实现已落盘，但 **fake / CI 通过不等于真实桌面通知送达**（daily 执行册明确此点）。
- **签名身份**：macOS 未签名 / `wails dev` 下 `requestAuthorization` 行为与正式签名 `.app`
  可能不同，属 [G-native](delivery-macos-signing-identity.md)。
- **回退**：停止 `runJournalReminder` 并取消已排通知，保留设置键与 UI 开关；
  不做破坏性迁移，不影响日记 / 目标数据。
