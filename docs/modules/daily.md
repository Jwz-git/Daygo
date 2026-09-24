# daily — 每日复盘

## 用户结果与范围

用户能查看可粘贴的每日摘要，记录意图、笔记、目标与反思，并设置日记提醒。
摘要使用日历日 standupDay；日记与目标使用 4 点边界的逻辑日 day。
负责 U4、F-V4 及既有日记 / 目标 / 提醒需求；不负责时间线生成、每周视图或外部自动汇报。

依据：[03 洞察与输入](../03-data-model.md#334-洞察与用户输入)、
[05 洞察绑定](../05-interface-contract.md#洞察)、
[05 日期契约](../05-interface-contract.md#532-时间与日期)。

## 当前状态与证据

> **验收状态**：已实现能力于 2026-09-22 经用户确认已验收；无逐项运行记录。未实现能力见 [09 §9.1](../09-roadmap.md#91-模块总表)。

实现进度：部分实现。已落盘可接入的 [每日页面](../../frontend/src/views/Daily/DailyView.vue)、
[集中式 store](../../frontend/src/stores/daily.ts) 与薄
[API wrapper](../../frontend/src/api/daily.ts)：按后端 `dayStartTs/dayEndTs` 和卡片时间戳呈现
15 分钟工作流、派生指标及只读日报，并区分整页不可用与仅日报不可用 / 失败。
[开发专用匿名样例](../../frontend/dev-fixtures/daily.json) 由 Vite dev middleware 提供，生产构建
无该数据路径。

**2026-09-12**：日记切片已落盘——迁移 v5（`journal_entries`）、`storage.JournalRepo`
（用户 upsert 不触碰 AI summary 列）、绑定 `GetJournalDay` / `SaveJournalDay`
（`journal:updated` 事件）、前端 DailyJournalPanel（写后不乐观更新，等事件重拉）。

**2026-09-15**：`GetDailyRecap` / `SaveDailyRecap` 绑定已落盘——迁移 v13
（`daily_standup_entries`）、`storage.StandupRepo`（JSON 存储 highlights/tasks 数组）、
后端绑定 `GetDailyRecap`（读取已有日报或返回空结构）与 `SaveDailyRecap`
（只提供日报存储入口）。前端 DailyRecapPanel 支持真实日报和日记草稿两种视图。

**2026-09-15（生成触发）**：`GenerateDailyRecap(standupDay)` 绑定落盘——按日历日窗口读取
当日活动卡片（排除 System / Idle 分类），经 `insight.StandupPrompt` /
`insight.StandupOutput`（结构化输出 schema）调用 provider 链生成站会三段，校验后写入
`daily_standup_entries` 并发 `recap:updated` 失效事件。`SaveDailyRecap` 也开始发同一事件。
错误映射：无 provider → `provider_not_configured`，模型 / schema 失败 → `provider_failed`；
只读实例 → `not_capture_owner`。前端「重新生成」按钮接通（生成中禁用、失败提示、
事件后重拉）。Go 侧有 httptest 全链路断言；真实 provider 与 `wails dev` 真机往返已于 2026-09-22 经用户实测验收（无逐项运行记录）。

**2026-09-22（后台补生成）**：读写实例启动时 + 此后每小时后台扫描，从最早活动卡片日历日到
今天逐日检查 `daily_standup_entries`：已结束完整日缺失即生成且绝不覆盖；今天在缺失或
`generated_at` 早于 4 小时时(重)生成。契约见 `docs/05-interface-contract.md §5.5.1`
「日报补生成触发」。存储新增 `CardRepo.EarliestCardStart` 与 `StandupRepo.ExistingDays`；
`GenerateDailyRecap` 拆出 `dayActivityCards` / `generateRecapFromCards`（接收 ctx，供补生成
复用）；runner 在 `internal/app/standup_backfill.go`，经 app.go RW-only 启动块
`go runStandupBackfill(ctx)` 接入。空活动日跳过、无 provider 静默等待、连续失败中止本轮。
Go 侧 httptest 全链路 + 存储夹具断言（补历史、空活动跳过、不覆盖已存、今天生成 / 刷新 /
新鲜跳过、只读实例空操作、取消即停）；真实 provider 与隔夜真机往返已于 2026-09-22 经用户实测验收（无逐项运行记录）。

**2026-09-22（移除日记 AI 摘要）**：日记的 AI summary 从未生成，且概念上就是站会日报，故全栈移除：
迁移 v19 重建 `journal_entries`（去掉 summary 列，夹具 `v18-card-ratings.db` + DB-2 验证保数据、
去列）、`storage.JournalEntry` 与 `app.JournalDayDTO` 去掉 Summary 字段、前端 DailyJournalPanel
删除「AI 摘要」块与 i18n。

文本生成录制后即时触发、通知仍未实现（补生成已覆盖"隔日自动出日报"与"当天每 4 小时刷新"，
录制后的即时触发仍缺）。

## 能力与跨层职责

| 输入 | 可独立推进 | 真实接入条件 |
|---|---|---|
| timeline: time / cards | 以固定卡片和五时区输入验证按日查询 | 日期子能力和 cards repository 独立验收 |
| providers: provider-client | 固定文本响应验证摘要映射、缺失与失败 | 用户配置的文本调用真实可用 |
| data: db-core；preferences: settings-access / ui-bridge | 日记与提醒配置 fixture | 持久化、生成绑定与事件接入；G-host |
| System 通知端口 | fake 测试开启、改时间、取消、拒绝权限 | 真实 macOS 通知授权与计划 / 取消实验 |

输出 notifications 能力，由本模块负责 fake 与真实实现；公共 System 端口变更与 recording 协调。
internal/insight 负责只读日视图；写入 / 生成通过 app 编排消费者接口，AI 调用走 providers，
repository 位于 internal/storage。notifications 设置、日记 / 目标表和 UI 归本模块。
模型输出只作为数据，AI summary 前端只读；如需新增生成触发 API，先补 05 和双侧契约，
不得臆造现有绑定。v1 所需生成触发与刷新语义须在生成切片前明确。

## 实验与失败条件

| 实验 | 输入与操作 | 预期结果 | 失败条件 / 证据 |
|---|---|---|---|
| 日期归属 | 03:30 / 04:30、DST、跨午夜卡片，分别查询 day 与 standupDay | 摘要日历日，日记 / 目标逻辑日；不由前端计算 | 混用边界、丢失或重复活动失败 |
| 日记 / 目标往返 | 空日、填写 / 修改 / 跳过目标，重启后读取 | DTO 可空与状态正确、内容保留、事件后重拉 | 丢数据、覆盖 AI 只读字段或错误状态失败 |
| 文本生成 | 固定成功 / 畸形 / 超时响应，真实服务单独验证 | 字段形状与失败状态可观察，不要求文本字面相同 | 把模型指令当操作、错误静默覆盖已存内容失败 |
| 提醒 | 默认关闭，开启、改时刻、取消、拒绝权限 | 按设置调度，取消生效，不循环申请权限 | 重复提醒、关闭后仍提醒、无授权却伪报成功失败 |

## 实现切片与集成

1. **部分完成**：固定日期、卡片及只读日报 fixture，完成工作流 / 指标 / 日报呈现；日记与目标
   fixture、生成触发 / 刷新和错误交互仍待契约决策。
2. 在 storage 加所需迁移与 repository，独立验证保存、查询和重启；复用 time / cards。
3. 通过客户端接口生成并存储摘要，保持 insight 只读；fixture 后接真实服务。
4. 补 System 通知 fake / 原生与提醒设置；接每日绑定、事件、store、UI 和完整空 / 错误 / 加载态。
5. 用真实卡片与持久化日记验收用户闭环，并在真实 macOS 验证提醒；可独立于 weekly 完成。

## 验收、阻塞与回退

完成要求：真实日期与文本输入闭环、日记 / 目标可重启读回、提醒可开关取消、
两种语言完整。上游 UI 无须完成；fake 不能证明通知送达或摘要服务可用。
生成触发与刷新细节由 daily 在实现前补齐 05；通知实现方式由 daily 工程在原生接入前决定。
G-host 限制大规模 UI，其他缺口只阻塞相应文本 / 通知能力。G-host 已于 2026-09-22 经用户实测验收（无逐项运行记录）。

回退：停止生成任务并取消本模块计划的通知，保留日记 / 目标与旧摘要；
禁用不可用的入口，不删除用户输入或改动 recording 的录制意愿。

## 验证记录

2026-09-11—12：Go 绑定测试、前端 typecheck / build 与 Vite 匿名卡片预览覆盖日记编辑、日报展示、目标和日期路由。真实 Wails 保存、重启读回及长期表现由用户于 2026-09-22 确认验收，未附逐项运行记录。
