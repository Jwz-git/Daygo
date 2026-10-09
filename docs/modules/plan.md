# plan — 计划与提醒

> 公共执行规则和门禁见 [09](../09-roadmap.md)，字段级契约唯一出处是 [05](../05-interface-contract.md)。

## 用户结果与范围

用户把一天要做的事排成**计划块**（时段 + 可自由编辑的标题与备注 + 可选分类），在时间线
检查器的「今日计划」中新增、编辑、标记完成 / 跳过、删除；计划块不进入卡片轨道，只画圆头虚线——
日时间线在时间栏、周时间线在各列右缘，重叠计划并排分道；点击线条弹出详情（完成 / 跳过 / 编辑，
编辑转到检查器表单）。计划块开始时、计划进行中分心过多、当日分心超过目标上限时，Daygo 发系统通知。
外部 AI 与终端经 CLI / MCP / agent.sock 读取当天计划并在完成后标记（受「允许外部工具修改」
门禁），应用内对话同样可用。

非目标（本切片）：重复计划、跨天计划块、日历导入、AI 自动排程、点击通知唤回窗口、
计划与实际的周汇总图。

## 当前状态与证据

实现进度：部分实现（已在 `test` 合入，基础提交 `4dac8b7`）。

- 存储：迁移 v20 `plan_blocks`（[03](../03-data-model.md)）、`storage.PlanRepo`；v19 → v20 匿名
  夹具 `internal/storage/testdata/v19-journal-no-summary.db`（生成器 `writeV19`）。
- 时间：`timeutil.ResolveDayClock` / `ResolveDayRange`——`HH:mm` 落入逻辑日（00:00–03:59 属次日日历日，
  结束 `04:00` = 当日结束，DST 跳过的时刻向前顺延，同一时刻起止视为零长拒绝）。
- 派生：`insight.PlanReviews` / `PlanBlockCoverage` / `DistractionMinutes`——块内同分类卡片分钟与分心分钟
  （分心 = Distraction 分类卡片 ∪ 卡片内嵌分心区间 ∪ 当日目标的分心分类，并集去重，截至此刻）。
  绑定、chat 与 CLI / MCP 共用同一函数。
- 绑定：`GetPlanDay` / `SavePlanBlock` / `SetPlanBlockStatus` / `DeletePlanBlock`，事件 `plan:updated`
  （[05 §5.5.1](../05-interface-contract.md#551-绑定方法目录)）。
- 外部：CLI `daygo plan [day]`；`agent.sock` / MCP / chat 写操作 `plan_add` `plan_update`
  `plan_complete` `plan_delete`，MCP / chat 读工具 `plan`（[05 §5.9](../05-interface-contract.md#59-b6对外接口推迟到-v11)）。
- 提醒：`internal/app/plan_reminder.go`，读写实例上运行的对账循环（2 分钟 + 写入后立即唤醒）。
- 能力：`GetCapabilities.features` 的 `notifications` 区分实现与授权；不可用时禁用提醒编辑、
  新计划默认不提醒，已有提醒偏好保留，九语言说明复用设置文案。
- 原生投递：macOS `UNUserNotificationCenter`（`internal/platform/darwin/notifications_darwin.go`），
  见 [通知决策](../decisions/notifications-journal-reminder.md)；Windows 同端口已于 10-09 接入 C++/WinRT 排程与取消，见
  [Windows 决策](../decisions/notifications-windows-toast.md)；真实送达待验收，点击唤回仍在范围外。
- 前端：`api/plan.ts`、`stores/plan.ts`（当日 + 周视图各日、检查器定位请求）、
  `views/Timeline/PlanPanel.vue` / `PlanBlockForm.vue` / `PlanBlockPopover.vue`、`planLayout.ts`
  （重叠分道 `planLanes`、阶段 `planPhase`，夹具见 `frontend/tests/plan.test.ts`）、日轨道时间栏与
  周视图列右缘的圆头虚线（mask 平铺圆角矩形）、九语言 `timeline.plan` / `native.plan` 文案、`App.vue` 推送通知文案。

## 能力与跨层职责

| 输入能力 / 契约 | 负责模块 | 可独立推进 / fake 可证明什么 | 真实接入前置条件 |
|---|---|---|---|
| 逻辑日与 `HH:mm` 换算 | timeline（timeutil） | 时区 / DST / 半小时区 / 跨午夜夹具 | — |
| 卡片与分心区间 | timeline | 派生分钟的手算夹具 | 真实卡片由分析批次产出，分心随批次延迟 |
| 当日目标的分心分类与上限 | daily | 当日分心提醒夹具 | — |
| 系统通知端口 | recording / daily（notifications） | fake 记录排程 / 取消 / 立即投递 | 真机授权弹窗与投递，见门禁 |
| agent 写门禁 | agent | 共享执行器与 socket 夹具 | `agentEditsEnabled` 开启 |

## 实验与失败条件

| 实验 / 风险 | 输入与操作 | 预期结果 | 失败条件 / 证据 |
|---|---|---|---|
| 开始通知 | 新建 5 分钟后开始、勾选提醒的计划块 | 到点弹出「开始：标题」 | 不弹 / 弹两次 / 改时间后仍按旧时间弹 |
| 进行中分心提醒 | 计划块进行中分心 ≥10 分钟且 ≥ 已过时长 25% | 弹一次，不重复 | 未达阈值即弹、或同一块反复弹 |
| 当日分心上限 | 当日目标设分心上限，分心超过 | 当天弹一次 | 无上限时弹、或重复弹 |
| 授权 | 首次投递 | 系统授权弹窗一次；拒绝后不再追问 | 循环申请权限 |
| 外部标记 | `daygo write plan_complete '{"blockId":N}'` | 界面立即显示已完成 | 界面不刷新 / 绕过门禁 |

阈值为固定常量（`planDistractionMinMinutes = 10`、`planDistractionShare = 0.25`）；提醒状态只在进程内，
重启后已发的分心提醒可能再出现一次；分心来自已分析卡片，滞后于屏幕一个分析批次间隔。

## 验收、阻塞与回退

- 门禁：Go `CGO_ENABLED=0` 测试、前端 typecheck / unit / build；原生投递须在真实 macOS（签名 `.app`）观察，
  属 G-native 同类门禁，fake 通过不等于通知送达。
- 回退：停止 `runPlanReminder` 并取消 `plan-start-*` 通知；`plan_blocks` 为新增表，不影响其它数据，
  降级迁移不做破坏性删除。

## 验证记录

2026-10-09：Windows 复用通知端口的实现与证据见 [daily 验证记录](daily.md#验证记录)及
[Windows 决策](../decisions/notifications-windows-toast.md)。原生 DLL 编译 / ABI / XML 门禁通过，
提权 CI 主机的注册 / 权限 / 系统排程明确跳过；计划通知的真实 Windows 桌面投递未运行。

2026-10-07（`test`，基于 `5b6d903` 的工作树，macOS arm64）：新增无授权副作用的
`NotificationAvailability`，仅数据库已打开且当前平台 / 构建有原生投递时广告 `notifications`。
日记与计划提醒 UI 缺能力时禁用并显示九语言说明；新计划默认不提醒，既有偏好不清除。
先写支持 / 不支持 / 权限拒绝 / 无数据库的匿名 Go 夹具，旧代码支持分支失败；前端 capability
失败重试与真实 PlanBlockForm SSR 夹具通过（SSR 不证明真实窗口视觉）。完整 `./scripts/gate.sh`
通过，前端 259 项 / 0 失败 / 跳过；`gofmt -l .` 无输出，文档检查 60 篇 / 0 处问题。
`go test ./internal/platform/darwin -run TestNotificationsUnavailableWithoutAppBundle -count=1`
通过，只证明无 bundle 时不广告原生通知；新增 cgo 查询初次编译发现 BOOL 类型差异，统一通过
C int 返回值后通过。不证明授权或投递；真实 macOS bundle、Windows 主机与点击唤回未运行。
Windows 安装器 15 项中 10 项需 Windows 主机而跳过。回退：撤销本次能力门禁提交，无 schema 变更。

- **2026-10-03（`feature/plan` 工作树，未提交，macOS arm64）**：`gofmt -l` 无输出、`go vet ./...`、
  `CGO_ENABLED=0 go test ./internal/...` 全部通过（含 storage v19→v20 夹具、repository、timeutil 日内时刻 /
  DST / 跨午夜、insight 派生分钟手算夹具、app 绑定 / 校验 / 只读实例 / 共享执行器 / socket、提醒的开始 /
  取消 / 幂等 / 分心阈值 / 当日上限、MCP 与 chat 封闭工具集）；darwin cgo 与非 cgo、Windows / Linux 交叉构建通过；
  `go test ./internal/platform/darwin/`（cgo，无 bundle 时返回不可用、不崩溃）通过；前端 typecheck、
  unit（209 项）、build 通过；Vite 匿名预览中经表单新增两个计划块、标记完成，检查器与时间栏标记显示正确
  （预览用内存替身，非真实后端）。**未运行**：真实 Wails 窗口内的端到端、macOS 通知授权与投递、
  CLI / MCP 对真实数据库的往返。
- **2026-10-03（同上工作树，前端增量）**：计划块在日 / 周时间线以圆头虚线显示、可点击（详情弹层：
  完成、跳过、编辑），重叠分道，卡片轨道内不再画占位块；检查器改为分块卡片排版；当日目标改为 Dayflow 风格（分类池循环 / 拖放、专注与分心面板、
  时 / 分滚轮）；全前端原生 `<select>` 换为公共 `components/DgSelect.vue`。前端 typecheck、unit（211 项）、
  build 通过；无头 Chrome 截图核对：Vite 匿名预览（日视图、弹层、目标编辑器浅 / 深色、设置页下拉）与
  运行中的 wails dev 实例（只读打开周 / 日视图，用户自建的两个重叠计划分道显示、弹层数据正确）。
  **未运行**：在 Wails 窗口内点击完成 / 跳过 / 编辑的写入往返、拖放在 WKWebView 中的行为。

- **2026-10-07（`test`，基于 `3cd8f40` 的工作树，macOS arm64）**：计划 store 同时订阅
  `plan:updated`、`timeline:updated`、`goal:updated`，刷新当日与已加载周内的派生分钟；
  周请求按周与日版本隔离，迟到响应不得插回旧周或覆盖较新事件。先加匿名夹具，旧实现三项失败，
  修复后前端 260 项通过、0 失败 / 跳过；完整 `./scripts/gate.sh` 通过（包括三平台核心交叉构建，
  Windows 主机执行项按跳过记录），`gofmt -l .` 无输出。未复测真实 Wails 写入、原生通知或外部客户端。
