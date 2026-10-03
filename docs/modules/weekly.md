# weekly — 每周复盘

## 用户结果与范围

用户看到本周跟踪时长、专注时长和分类占比。负责 U5、F-V5；
合计排除 System，专注排除 isIdle 与内置 Distraction 分类，空周占比为 0。首个有限前端切片只用现有聚合 DTO
呈现专注比例和分类分布；树状图、热力图、桑基图及其子模型继续待定。本模块不依赖 daily
完成，不重新生成时间线。

依据：[05 周 DTO](../05-interface-contract.md#552-dto-目录)、
[03 日期](../03-data-model.md#32-凌晨-4-点逻辑日)、
[08 属性测试](../08-testing-strategy.md#831-基于属性的测试)。

## 当前状态与证据

> **验收状态（2026-09-26）**：本模块所有已实现能力（含近期增量、长期观察与已实现的真实安装升级）经用户确认已验收，未附逐项运行记录。未实现能力、待定设计与正式证书缺失保持原状态；历史命令的失败、跳过或未运行不改写为通过。统一记录见 [09 §9.1.1](../09-roadmap.md#911-本轮验收记录与证据边界)。

实现进度与验证状态以 [09 §9.1](../09-roadmap.md#91-模块总表) weekly 行为准。当前能力快照：

- 边界与聚合：`timeutil.WeekStart` / `WeekWindow`（周一 4 点对齐，
  decisions/weekly-boundary-monday）、`storage.CategoryMinutesInRange`（03 §3.5
  的窗口交集重叠谓词 + categories join 取 is_idle）、
  `internal/insight.AggregateWeekly`（tracked 排 System、focus 排 isIdle 与 Distraction、share
  分母 0 为 0、minutes DESC）、`DayContextDTO.weekStart`（前端初始周不自算）。
- 绑定与前端：`GetWeeklyDashboard`（非周一拒绝，含按日明细 `WeeklyDayDTO` 与洞察
  `WeeklyInsightsDTO`）、页面容器、weekly store、薄 API wrapper、加载 / 不可用 /
  失败 / 空 / 有数据状态与 Dayflow 原版六张图（分布环形图、上下文切换对比、工作流、
  热力图、矩形树图、桑基图；2026-10-02 起页面只保留这六张，原有的专注概览、分类分布、
  按日时间线、节奏与洞察面板已移除，`WeeklyInsightsDTO` / `WeeklyDayDTO` 仍由绑定返回）；
  开发服务器专用匿名聚合夹具。组件不直接调用 Wails；`timeline:updated` 只触发重新拉取。
- 跨周观察与真实卡片周由用户于 2026-09-22 确认验收，未附逐项运行记录。

## 能力与跨层职责

| 输入 | 可独立推进 | 真实接入条件 |
|---|---|---|
| timeline: time / cards | 固定卡片、分类与周窗口的聚合 fixture | 周边界和卡片 / 分类查询独立验收 |
| data: db-core | repository fake 验证只读查询 | 真实库读取正确；无需维护界面完成 |
| preferences: ui-bridge | DTO / store / 错误与空态 fixture | 生成绑定、事件重拉与 G-host（G-host 已于 2026-09-22 经用户实测验收，无逐项运行记录） |

internal/insight 负责只读周聚合；数据库查询只在 internal/storage，周边界只在 timeutil。
周边界已决定：周一起始、凌晨 4 点逻辑日对齐（[decisions/weekly-boundary-monday](../decisions/weekly-boundary-monday.md)）。
app 提供 GetWeeklyDashboard，store 查询并响应时间线 / 分类失效事件；组件只呈现。
需要的查询由本模块在 storage 中交付，不复制 cards repository 或写第二套分类体系。
本模块不新增 AI 调用，也不将 daily 日记的存在作为聚合前提。

## 实验与失败条件

| 实验 | 输入与操作 | 预期结果 | 失败条件 / 证据 |
|---|---|---|---|
| 合计 / 占比 | 空周、只有 System、混合 Idle / Distraction 与工作分类 | tracked 排除 System、focus 排除 Idle 与 Distraction，分母 0 时 share=0 | 除零、System 或分心计入专注、分类合计不一致失败 |
| 周边界 | 跨周一、午夜到 4 点、DST 与半小时 / 45 分钟时区 | 周窗口无重叠 / 间隙，使用后端 day 语义 | 前端推算日界、跨周重复或遗漏失败 |
| 分类变化 | 改名、改类、软删卡片，重拉周结果 | 与 cards 当前事实一致，既有排序语义保持 | stale 聚合或重名双计失败 |
| 真实读取 | 已有真实卡片的一周，对照只读聚合与 UI | DTO、时长和占比一致 | fixture 展示不能记为真实集成通过 |

## 实现切片与集成

1. 固定聚合与边界夹具；与 timeline 确认周子能力，基于 cards 契约即可开工。
2. 实现只读 repository 查询与 insight 聚合，完成 08 行为 7 和属性测试。
3. 已决定现有聚合 DTO 的首屏形态：专注比例环、三项时长和分类比例 / 排行；不增加公共字段。
   更丰富的热力图、应用关系或流向图若进入范围，先更新 05 和双侧契约再实现。
4. 接周绑定、store 失效重拉、UI 与九语言空 / 错误 / 加载态。
5. 用真实 cards 独立验收；跨周一的长期证据另行累计，无须等 daily 或完整分析 UI。真实卡片周独立验收与跨周一长期观察已于 2026-09-22 经用户实测验收（无逐项运行记录）。

## 验收、阻塞与回退

完成要求：周聚合、日期与契约测试通过，真实卡片可查询且 UI 正确，图表决定已记录。
图表未定不阻塞纯聚合；cards 未就绪可使用 fake，但不能标为真实集成通过。
大规模 UI 遵循 G-host。图表 / DTO 子模型由 weekly 产品与设计在 UI 实现前决定。

回退：禁用不可用的周入口，保留原始卡片与分类；只读聚合失败不触发任何数据改写。
时区规则回归先撤销该能力变更，不能用改 fixture 期望掩盖问题。

## 验证记录

2026-10-03（test 分支未提交工作树）：周报末尾的 Token 用量卡片改为按 `llm.showTokenUsage`
（设置「AI 服务」分区，默认关）决定是否渲染，卡片读取与失败态不变。匿名浏览器预览中开关开启后
周报卡片出现、关闭后日报（同一开关）消失，pageerror 0；`./scripts/gate.sh` 通过；
开关本身的实现、夹具与验证证据见 [providers](providers.md)。未运行：真实 Wails 窗口内的切换与重启读回。

2026-10-02（`frontend-lab` 分支，树图去描边）：分类框与应用方块不再画描边，改用带自身颜色的柔和阴影（细接触影 +
较宽扩散影）分隔，悬停时阴影加深；深色主题改用更深的中性阴影。阴影、边缘渐变、液态玻璃、背板光晕、顶部高光
五种方案曾在开发版周报中并排预览，用户选定阴影，其余方案与预览代码未提交。验证：typecheck、unit（208 项）通过；
无界面 Chrome 浅 / 深色放大截图确认无描边线。

2026-10-02（`frontend-lab` 分支，移除原有面板）：周报末尾的专注概览、分类分布、按日时间线、节奏与洞察 5 个面板及其
专用文案（9 个语言包的 overview / metric / categories / daily / rhythm / insights 组）删除，页面只保留 Dayflow 六张图与
统计范围说明。`stores/weeklyPresentation` 中仅服务这些面板的字段暂未清理（仍有夹具测试），环形图继续读取其
`categories`。验证：typecheck、unit（208 项）、build 通过；匿名夹具页面只渲染六张图卡片。

2026-10-02（`frontend-lab` 分支，树图字号与桑基图数字）：树图应用名与图标加大（衬线体单一字重，用 0.45px 描边加粗而非
合成粗体），时长改细、比应用名小，并按原版 `weeklyTreemapDurationString` 用英文单位（"6hr 4m"），周环比标签按原版
只计分钟（"+ 152m"）并缩小变细；单位串在 9 个语言包中保持英文，作为显式决定加入 i18n 测试的共享词白名单。
块内展示模式改为按当档字号实算所需高度，1400 / 1024 宽下 16 块均无内容溢出。桑基图数字改小、灰色（深色主题用
muted 灰）。验证：typecheck、unit（203 项）、build 通过；无界面 Chrome 浅色截图检查。

2026-10-02（`frontend-lab` 分支，桑基图与树图对齐原版）：桑基图几何移植 Dayflow `WeeklySankeyModelFactory`
到 `lib/sankeyLayout.ts`——1748×933 虚拟画布、三列逐级变高（433 / 702 / 874）、每条丝带铺满目标条形、应用按来源
分类加权中心排序且「其他」置底、曲率 0.15 / 0.42、柱间暖色底纹与原版渐变色标；应用颜色改为原版
`appColorHex`（品牌关键字优先，否则 djb2 哈希取色板，64 位回绕），唯一偏差是 `x` 须为完整名称或域名段，
避免 Firefox 被染黑。悬停 / 固定沿用原版规则（丝带代表其目标节点，无关丝带 0.12、无关节点 0.25）。
矩形树图恢复原版衬线应用名与图标同行，沿用原版字号分级与展示模式但整体加大一档，时长改深色正文，
周环比为原版等宽数字底色标签。匿名开发夹具改 4 个时段的应用（时长不变），让 GitHub / VS Code / YouTube /
Slack 各跨两个分类。验证：`sankeyLayout` 夹具 6 项（含原版规则的手算值）、`sankeyAppColor` 黄金值（在 Swift 中
运行原版 `fallbackColorHex` 得出）、typecheck、unit、build 通过；内置浏览器浅色 1400 / 1024 宽与无界面 Chrome
深色截图检查，悬停 GitHub 只保留两条来源丝带与两个分类。限制：未在真实 Wails 窗口与真实数据下检查。

2026-10-02（`frontend-lab` 分支，图表交互）：修复桑基图丝带全黑——渐变 id 曾由分类名拼接，含空格的分类
（如 `Focus Work`）使 `url(#…)` 引用失效；现改为按序号并以 `useId()` 限定实例。6 张图加入悬停交互与浮动提示：
环形图扇区 / 图例联动并在中心显示该分类；对比图按天联动两侧；工作流与热力图按指针换算网格（格间缝隙不闪烁）、
淡出其它天并描边当前格，工作流图例可高亮同类格；矩形树图只淡化其它块的填充、文字保持可读，块内显示内容按实际
渲染尺寸分级；桑基图悬停高亮相连路径、点击固定。图内文字整体加大加粗。验证：typecheck、unit（197 项）、build、
`git diff --check` 通过；Vite 匿名夹具在内置浏览器（约 780px 宽）逐张悬停截图检查。限制：未在真实 Wails 窗口、
深色外观与九语言长文案下逐项检查；窄窗口下工作流 / 热力图的 SVG 整体缩放，轴标签仍偏小。

2026-10-02（`frontend-lab` 分支）：周报加入 Dayflow 原版的 6 张图（周分布环形图、上下文切换与分心对比、
本周工作流、专注与分心热力图、各分类最常用应用矩形树图、分类与应用桑基图），Daygo 原有区块排在其后。
后端 `WeeklySegmentDTO` 增加 `appSites` 与 `distractions`（Go 以 `timeutil.ResolveClock` 解析并裁剪到时段）；
前端 `stores/weeklyCharts.ts` 移植 Dayflow `WeeklyDashboardBuilder` 的汇总算法，应用身份复用时间线的
`lib/appSiteIcon`，分心仅依据 Distraction 分类与卡片分心区间（不沿用原版的英文标题匹配）；矩形树图的
周环比额外读取上一周周报，失败时仅隐藏变化值。验证：insight / storage / app 夹具、`weeklyCharts` 与
`chartLayout` 前端夹具、typecheck、unit、build 通过；Vite 预览（匿名夹具）用无界面 Chrome 截图检查 6 张图。
限制：未在真实 Wails 窗口与真实一周数据下检查；深色外观、窄窗口与九语言长文案未逐项目检。

2026-09-23：跨午夜 23:50–00:10 匿名周节律夹具通过，00:00 后的 10 分钟归入下一时钟小时；前端 `test:unit`、`typecheck`、`build` 及完整门禁通过。

2026-09-23：日分布图改为左侧日期、共享小时轴与网格线、右侧每日时长。前端单测、
`typecheck` 和 `build` 通过；本次只验证构建与现有交互测试，窄窗口实际视觉布局仍需在应用内复核。

2026-09-11—20：Go 的周边界属性测试、存储与聚合夹具覆盖空周、System / Idle / Distraction 排除、非周一拒绝及 DST / 半小时 / 45 分钟时区；前端 typecheck / build 和 Vite 匿名数据预览覆盖深浅主题、双语言与窄窗口。2026-09-20 修正 Distraction 误计为专注。真实卡片周与跨周观察由用户于 2026-09-22 确认验收，未附逐项运行记录。

2026-09-23：匿名周一 03:30–04:30 卡片夹具验证前后两周各计 30 分钟，日明细各有一个无重叠时间片；`go test ./internal/app ./internal/storage ./internal/insight` 通过。真实历史库尚未单独复核。


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
