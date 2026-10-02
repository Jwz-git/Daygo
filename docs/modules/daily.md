# daily — 每日复盘

## 用户结果与范围

用户能查看可粘贴的每日摘要，记录意图、笔记、目标与反思，并设置日记提醒。
摘要使用日历日 standupDay；日记与目标使用 4 点边界的逻辑日 day。
负责 U4、F-V4 及既有日记 / 目标 / 提醒需求；不负责时间线生成、每周视图或外部自动汇报。

依据：[03 洞察与输入](../03-data-model.md#334-洞察与用户输入)、
[05 洞察绑定](../05-interface-contract.md#洞察)、
[05 日期契约](../05-interface-contract.md#532-时间与日期)。

## 当前状态与证据

> **验收状态（2026-09-26）**：本模块所有已实现能力（含近期增量、长期观察与已实现的真实安装升级）经用户确认已验收，未附逐项运行记录。未实现能力、待定设计与正式证书缺失保持原状态；历史命令的失败、跳过或未运行不改写为通过。统一记录见 [09 §9.1.1](../09-roadmap.md#911-本轮验收记录与证据边界)。

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

文本生成录制后即时触发仍未实现（补生成已覆盖"隔日自动出日报"与"当天每 4 小时刷新"，
录制后的即时触发仍缺）。

**2026-09-28（日记提醒调度）**：日记提醒的 Go 侧调度落盘——决策
[notifications-journal-reminder](../decisions/notifications-journal-reminder.md)、
调度器 `internal/app/journal_reminder.go` 的 `runJournalReminder` / `journalReminderSync`
（ctx 持有、经 app.go RW-only 启动块接入，与 `runStandupBackfill` 同构）、
`nextReminderAt` 墙钟时刻（本地 `time.Location`，DST 安全）；`fake.System` 记录
`ScheduleNotification` / `CancelNotifications` 供夹具断言；设置 UI（通用区开关 + 时刻输入）、
`NativeUiLabelsDTO` 的 `journalReminderTitle` / `journalReminderBody` 文案通路与九语言文案。
Go 侧夹具覆盖：默认关闭不排、按时刻排下一次、过点顺延次日、幂等不重排、改时刻 / 改文案重排、
关闭取消、只读实例空操作、平台失败可重试、能力不可用则静默跳过、文案未下发则等待、DST 与半小时时区。
**原生投递（macOS `UNUserNotificationCenter` / Windows toast）仍未实现**；在此之前
`ScheduleNotification` / `CancelNotifications` 诚实返回 `platform.ErrCapabilityUnavailable`
（不再以 nil 假装成功），调度器据此按能力静默跳过。fake 通过不等于通知送达。

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
模型输出只作为数据；站会日报 AI 生成后可读回，日记不再有 AI summary。手动生成与后台补生成
触发 / 刷新语义已经落在 05；录制后即时触发仍未实现，新增入口须先补契约与双侧夹具。

## 实验与失败条件

| 实验 | 输入与操作 | 预期结果 | 失败条件 / 证据 |
|---|---|---|---|
| 日期归属 | 03:30 / 04:30、DST、跨午夜卡片，分别查询 day 与 standupDay | 摘要日历日，日记 / 目标逻辑日；不由前端计算 | 混用边界、丢失或重复活动失败 |
| 日记 / 目标往返 | 空日、填写 / 修改 / 跳过目标，重启后读取 | DTO 可空与状态正确、内容保留、事件后重拉 | 丢数据、覆盖 AI 只读字段或错误状态失败 |
| 文本生成 | 固定成功 / 畸形 / 超时响应，真实服务单独验证 | 字段形状与失败状态可观察，不要求文本字面相同 | 把模型指令当操作、错误静默覆盖已存内容失败 |
| 提醒 | 默认关闭，开启、改时刻、取消、拒绝权限 | 按设置调度，取消生效，不循环申请权限 | 重复提醒、关闭后仍提醒、无授权却伪报成功失败 |
| 提醒调度（Go） | 开启 / 改时刻 / 改文案 / 关闭各跑一次 `journalReminderSync` | 排一次、幂等不重排、过点顺延次日、关闭即取消、只读空操作 | 重复排、关闭后仍排、未授权仍记已排（夹具 `journal_reminder_test.go`） |

## 实现切片与集成

1. **已实现**：日期 / 卡片 / 日报夹具、工作流与指标、日记 / 目标持久化、手动生成、后台补生成 / 刷新及错误交互；录制后即时触发仍缺。
2. 在 storage 加所需迁移与 repository，独立验证保存、查询和重启；复用 time / cards。
3. 通过客户端接口生成并存储摘要，保持 insight 只读；fixture 后接真实服务。
4. **部分实现**：System 通知 fake 已记录调度 / 取消，提醒设置、文案通路与九语言 UI 已落盘；
   真实原生投递（`ScheduleNotification` 实现、授权弹窗、点击唤回）仍缺。
5. 用真实卡片与持久化日记验收用户闭环，并在真实 macOS 验证提醒；可独立于 weekly 完成。

## 验收、阻塞与回退

完成要求：真实日期与文本输入闭环、日记 / 目标可重启读回、提醒可开关取消、
九种语言完整。上游 UI 无须完成；fake 不能证明通知送达或摘要服务可用。
生成触发与刷新细节由 daily 在实现前补齐 05；通知实现方式已由
[日记提醒决策](../decisions/notifications-journal-reminder.md) 落定（重复归 Go、一次性端口、
墙钟时刻、文案经 `NativeUiLabelsDTO` 下发），原生投递仍受 G-native 门禁。
G-host 限制大规模 UI，其他缺口只阻塞相应文本 / 通知能力。G-host 已于 2026-09-22 经用户实测验收（无逐项运行记录）。

回退：停止生成任务并取消本模块计划的通知，保留日记 / 目标与旧摘要；
禁用不可用的入口，不删除用户输入或改动 recording 的录制意愿。

## 验证记录

2026-10-02（`frontend-lab` 分支）：工作流概览格子随卡片宽度伸缩——每格步长 20–38px、格与间隙保持 Dayflow 的 9:1，
标签与坐标轴字号随之放大至多 1.2 倍，窄于最小步长时仍横向滚动；尺寸由观察卡片宽度（不随内部格子变化）并延后一帧
更新得出，避免 ResizeObserver 循环告警被全局错误处理弹窗。验证：typecheck、unit、build 通过；内置浏览器 1600px
宽匿名夹具下格子 28px、网格铺到卡片右侧内边距、无横向溢出且无错误弹窗。限制：未在真实 Wails 窗口全屏下检查。

2026-09-11—12：Go 绑定测试、前端 typecheck / build 与 Vite 匿名卡片预览覆盖日记编辑、日报展示、目标和日期路由。真实 Wails 保存、重启读回及长期表现由用户于 2026-09-22 确认验收，未附逐项运行记录。

2026-09-28（日记提醒，macOS arm64，未提交工作树）：`./scripts/gate.sh` 通过——`CGO_ENABLED=0 go test ./internal/...`（24 包 ok，含 `journal_reminder_test.go` 14 项夹具）、`go vet`、`gofmt -l` 无输出、前端 `typecheck` 与 `build`、三平台核心交叉构建、`check-docs`（56 篇 0 处问题）与 Windows 安装器匿名夹具（9 项，5 项 skip 因需 Windows 主机的运行与卸载仍按 skip 记录，不倒填为通过）；前端 `test:unit` 178 项通过、0 失败 / 跳过。**这是 fake 与无头门禁证据**：不证明原生通知送达、授权弹窗或点击唤回，后者仍按下文 G-native 门禁单独验收。


2026-10-02（test 分支，Token 用量增量）：新增只读 `GetTokenUsage` 与报告后用量卡片，
默认折线，可切柱状/扇形。每日按凌晨 4 点逻辑日分小时，每周分七个逻辑日；
统计全部已记录调用（含重试/测试），输入统一含缓存，未报告用量单独提示。
无 schema 或 AI 调用变更。契约见 [05](../05-interface-contract.md)。
夹具覆盖左右边界、缓存语义、未知调用、空数据、绑定参数与图表求和。
本次增量真实 Wails/Provider、DST 图表显示尚未验收，历史确认不覆盖。
回退：撤销本次提交即可，原始 `llm_calls` 数据保留。

验证证据（2026-10-02，test 未提交工作树，macOS arm64）：`./scripts/gate.sh` 通过，
`gofmt -l .` 无输出，Go 内部测试 / vet / 无 cgo 构建、三平台核心交叉构建通过，
前端 unit 209 项通过、typecheck / build 通过；文档 57 篇无问题。
Windows 安装器 9 项中 5 项因需 Windows 主机跳过，未记为通过。
首次门禁仅因新增方法尚未进入绑定白名单失败；本次显式扩充
`contractBindings` 为允许 `GetTokenUsage`，与 05 方法表同步，随后重跑通过。
匿名浏览器夹具（Chrome headless，1300px / 760px）检查日报 / 周报三种图表、
深色、空数据、读取失败和重试入口，截图无裁切，pageerror 为 0；
输入/输出总数与夹具相符。夹具不是生产样例，不读取真实用户数据库。
DST、半小时 / 45 分钟时区由 Go 夹具验证；目标平台真实 Wails / Provider 对照未运行。


2026-10-02（test 分支，Token 图时区修复）：用户真实 Wails 报告
`invalid time zone: Local`。前次匿名浏览器只使用 Asia/Shanghai，未覆盖宿主的
Go `Local`；因此前次检查不能证明该输入可渲染。新增 Go 绑定回归夹具在修复前
复现失败（Local / 期望 Asia/Shanghai），后端改用 `timeutil.ZoneName`；
前端日期 formatter 复用 `safeTimeZone`，拒绝 Local / 畸形时区时回退本机时区。
新增前端夹具覆盖日 / 周标签的 Local、无效、空、缺失及合法 IANA 值，保持同一时间戳。
这是格式化修复，统计边界和数据不变；无既有夹具期望修改。

修复验证（2026-10-02，macOS arm64，test 工作树）：`./scripts/gate.sh` 通过
（Go 测试 / vet / 无 cgo 构建、三平台核心交叉构建、gofmt 无输出、前端 210 项 unit /
typecheck / build、文档检查）；Windows 安装器 5 项仍跳过。
Chrome headless 用后端 timeZone=Local 的匿名夹具检查日报 / 周报三种图表、
深色 / 760px、空 / 失败状态，pageerror 为 0。这不是修复后真实 Wails 验收，
真实窗口仍需重看。回退本次修复会重新暴露 Local 崩溃，不涉及数据迁移。


2026-10-02（test 工作树，Token 图位置）：按用户要求将 Token 用量卡片放到
日记面板之后，成为每日页面最后一项；周报原已在末尾。
验证环境 macOS arm64：前端 typecheck / build、文档检查、git diff --check 通过；
Chrome headless 匿名 Local 时区夹具断言 daily / weekly 的 Token 卡片均为
内容容器最后一个元素，pageerror 为 0。真实 Wails 位置未单独验收。
回退本次提交即可恢复原顺序，无数据影响。


2026-10-02（test 工作树，Token 图悬停）：折线 / 柱状按横向时段选中一桶，
显示不随 SVG 缩放增粗的 1px 竖线与悬浮明细，折线同时突出两个数值点。
明细为完整起止时间和时区、输入 / 输出 / 合计 / 调用次数，缺失用量仍显示提示；
不将桶汇总描述成瞬时值。复用既有 SVG 坐标映射与周报 tooltip；
离开绘图区、图表切换 / 数据替换时清除，不新增后端请求。
新增九语言「合计」文案；选点夹具覆盖日 / 周、首末边界、标签区域、空数据与 NaN。
回退：撤销本次提交，仅取消悬停交互，不影响统计数据。

悬停验证（2026-10-02，macOS arm64，test 工作树）：`./scripts/gate.sh` 通过，
含 Go 核心 / 三平台交叉构建、gofmt 无输出、前端 unit 211 项、typecheck / build
与文档检查；Windows 安装器仍 5 项跳过。首次前端类型检查发现共享 pointer 返回的
hovered 类型不能在模板直接充当数组下标，已改为 computed 选中点后重跑通过。
Chrome headless 匿名 Local 时区夹具验证日报 / 周报的折线和柱状图：
第 5 桶输入 3,724、输出 1,252、合计 4,976、调用 3、用量不完整 1，与提示一致；
竖线位于该桶中心，移出后竖线 / 提示清除，切扇形无残留。
1300px 浅色、760px 深色截图检查通过，窄窗口提示在卡片内，pageerror 为 0。
真实 Wails 鼠标交互未单独验收；既有真实验收不覆盖本次增量。
