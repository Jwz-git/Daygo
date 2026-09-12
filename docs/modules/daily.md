# daily — 每日复盘

## 用户结果与范围

用户能查看可粘贴的每日摘要，记录意图、笔记、目标与反思，并设置日记提醒。
摘要使用日历日 standupDay；日记与目标使用 4 点边界的逻辑日 day。
负责 U4、F-V4 及既有日记 / 目标 / 提醒需求；不负责时间线生成、每周视图或外部自动汇报。

依据：[03 洞察与输入](../03-data-model.md#334-洞察与用户输入)、
[05 洞察绑定](../05-interface-contract.md#洞察)、
[05 日期契约](../05-interface-contract.md#532-时间与日期)。

## 当前状态与证据

实现进度：部分实现。已落盘可接入的 [每日页面](../../frontend/src/views/Daily/DailyView.vue)、
[集中式 store](../../frontend/src/stores/daily.ts) 与薄
[API wrapper](../../frontend/src/api/daily.ts)：按后端 `dayStartTs/dayEndTs` 和卡片时间戳呈现
15 分钟工作流、派生指标及只读日报，并区分整页不可用与仅日报不可用 / 失败。
[开发专用匿名样例](../../frontend/dev-fixtures/daily.json) 由 Vite dev middleware 提供，生产构建
无该数据路径。**2026-09-12：日记与目标切片已落盘**——迁移 v5（`journal_entries` / `day_goals` /
`day_goal_categories`，`daily_standup_entries` 因无写入方不预建）、
`storage.JournalRepo`（用户 upsert 不触碰 AI summary 列）与 `storage.GoalRepo`
（分类引用单事务整体替换）、绑定 `GetJournalDay` / `SaveJournalDay` / `GetDayGoal` /
`SaveDayGoal`（分类 id 预检、`journal:updated` / `goal:updated` 事件）、前端
DailyJournalPanel / DailyGoalPanel（写后不乐观更新，等事件重拉）。
文本生成（`GetDailyRecap`，待定 #19）、摘要 AI 写入、通知仍未实现。

## 能力与跨层职责

| 输入 | 可独立推进 | 真实接入条件 |
|---|---|---|
| timeline: time / cards | 以固定卡片和五时区输入验证按日查询 | 日期子能力和 cards repository 独立验收 |
| providers: provider-client | 固定文本响应验证摘要映射、缺失与失败 | 用户配置的文本调用真实可用 |
| data: db-core；preferences: settings-access / ui-bridge | 日记、目标与提醒配置 fixture | 持久化、生成绑定与事件接入；G-host |
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
G-host 限制大规模 UI，其他缺口只阻塞相应文本 / 通知能力。

回退：停止生成任务并取消本模块计划的通知，保留日记 / 目标与旧摘要；
禁用未验收的入口，不删除用户输入或改动 recording 的录制意愿。

## 验证记录

2026-09-11 前端切片验证（Asia/Shanghai，6 张匿名卡片 + 1 份匿名日报）：

- `npm --prefix frontend run typecheck` 与 `npm --prefix frontend run build` 通过；
- 浏览器人工检查浅 / 深主题、700px 窄窗口、中英文界面与复制反馈通过；时间网格在窄窗口
  保持独立横向滚动；
- 生产 bundle 检查不含匿名日报文本、`/__daygo_dev__/daily` 或 `dev-fixture`；
- 这些证据只覆盖前端呈现和开发夹具隔离。真实 Wails 绑定、数据库往返、生成、通知、
  macOS WebView 长期表现及完整用户闭环未验收。

后续每次验证继续记录 commit、时区、匿名卡片、预期 / 实际结果及限制。

2026-09-11：Daily 开始消费与 Timeline 相同的路由日期键；显式历史日期经后端
`GetDayContext` 返回同名 `standupDay`，避免历史工作流误读当天日报。空参数在凌晨 4 点前仍
保持“当前逻辑日 + 当前日历日”的既定双日期语义。真实 `GetTimelineDay` / `GetDailyRecap`
尚未实现，此项只打通日期选择与查询参数，不代表 Daily 数据闭环。

2026-09-12（日记 / 目标绑定与编辑 UI）：`go test ./internal/app/`、前端 typecheck /
build、Vite 浏览器 smoke 通过。journal 往返（summary 保留断言）、goal 分类整体替换、
非法 status / 未知分类 → `invalid_argument`、只读实例 → `not_capture_owner`、事件
payload 均有 Go 断言；浏览器预览验证面板降级态（无桥 unavailable）、中英文、420px
窄宽无横向溢出。`wails dev` 真机保存 → 重启读回未运行。
