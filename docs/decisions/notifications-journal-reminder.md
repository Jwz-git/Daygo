# notifications 日记提醒：Go 拥有重复、一次性原生通知、墙钟时刻

> **状态：已决定；Go 调度与前端已实现，原生投递待真机验收。** 本轮落盘：决策、Go 侧调度器
> `runJournalReminder`、fake 夹具、设置 UI、九语言文案与 `NativeUiLabelsDTO` 文案通路。
> macOS / Windows 原生投递（`ScheduleNotification` 真实实现）**尚未实现**，属本文 §6 的
> on-device 门禁；`internal/platform/{darwin,windows}` 当前仍是 no-op 桩。

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
  | 关闭 | 若在排 → `CancelNotifications`；否则无事 |
  | 开启，时刻未变且已排给同一 `deliverAt` | 无事 |
  | 开启，时刻变更 / 未排 / `deliverAt` 已过 | 覆盖排下一次 |

  为此进程内保存上次下发的 `(enabled, time, deliverAt, title, body)`；仅在期望与已下发不一致时
  调用平台端口。文案随语言变化也算「变更」，会重排（否则通知停在旧语言）。
- **权限。** 端口只有 [`NotificationsPermission(ctx)`](../../internal/platform/ports.go) 查询，
  没有独立的 request 方法（对比 `RequestScreenRecordingPermission`）。首次投递时由原生
  `UNUserNotificationCenter.requestAuthorization` 弹一次系统授权；**Go 不主动轮询申请**，
  避免 daily 执行册禁止的「循环申请权限」。未授权时平台端口返回错误 → 记日志、不重试轰炸，
  等用户在系统设置里开启后由下一次 tick 的对账自然恢复。
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
| **原生投递** | darwin Swift `UNUserNotificationCenter`、windows C++ toast | **未实现**（§6） |

## 6. 未验证与门禁

- **on-device 投递（G-native）**：`ScheduleNotification` 的真实实现、系统授权弹窗、
  `DeliverAt` 到点真的弹出、进程不在（软退出为后台）时仍投递、点击通知唤回窗口
  （`SystemEventData.NotificationID` 已预留：`EventNotificationClick`），都必须在真实
  macOS / Windows 观察。当前桩返回 nil，Go 调度器的正确性只能靠 fake 证明——**fake 通过
  不等于通知送达**（daily 执行册明确此点）。
- **签名身份**：macOS 未签名 / `wails dev` 下 `requestAuthorization` 行为与正式签名 `.app`
  可能不同，属 [G-native](delivery-macos-signing-identity.md)。
- **回退**：停止 `runJournalReminder` 并取消已排通知，保留设置键与 UI 开关；
  不做破坏性迁移，不影响日记 / 目标数据。
