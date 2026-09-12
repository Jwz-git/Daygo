# weekly — 每周复盘

## 用户结果与范围

用户看到本周跟踪时长、专注时长和分类占比。负责 U5、F-V5；
合计排除 System，专注排除 isIdle，空周占比为 0。首个有限前端切片只用现有聚合 DTO
呈现专注比例和分类分布；树状图、热力图、桑基图及其子模型继续待定。本模块不依赖 daily
完成，不重新生成时间线。

依据：[05 周 DTO](../05-interface-contract.md#552-dto-目录)、
[03 日期](../03-data-model.md#32-凌晨-4-点逻辑日)、
[08 属性测试](../08-testing-strategy.md#831-基于属性的测试)。

## 当前状态与证据

实现进度：**部分实现**。已落下
[页面容器](../../frontend/src/views/Weekly/WeeklyView.vue)、
[weekly store](../../frontend/src/stores/weekly.ts)、薄 API wrapper、加载 / 不可用 / 失败 / 空 /
有数据状态、专注概览和分类分布，并提供开发服务器专用的
[匿名聚合夹具](../../frontend/dev-fixtures/weekly.json)。组件不直接调用 Wails；
`timeline:updated` 只触发重新拉取。

**2026-09-12：周聚合与绑定已落盘**——`timeutil.WeekStart` / `WeekWindow`
（周一 4 点对齐，decisions/weekly-boundary-monday）、`storage.CategoryMinutesInRange`
（与 `TotalMinutesTracked` 同一重叠谓词 + categories join 取 is_idle）、
`internal/insight.AggregateWeekly`（tracked 排 System、focus 排 isIdle、share 分母 0
为 0、minutes DESC）、绑定 `GetWeeklyDashboard`（非周一拒绝）、
`DayContextDTO.weekStart`（前端初始周不再自算）。跨周观察（G-stability）仍未运行；
这次不能记为 weekly 用户闭环完成。生产构建缺少绑定时明确显示能力不可用，不加载开发夹具。

## 能力与跨层职责

| 输入 | 可独立推进 | 真实接入条件 |
|---|---|---|
| timeline: time / cards | 固定卡片、分类与周窗口的聚合 fixture | 周边界和卡片 / 分类查询独立验收 |
| data: db-core | repository fake 验证只读查询 | 真实库读取正确；无需维护界面完成 |
| preferences: ui-bridge | DTO / store / 错误与空态 fixture | 生成绑定、事件重拉与 G-host |

internal/insight 负责只读周聚合；数据库查询只在 internal/storage，周边界只在 timeutil。
周边界已决定：周一起始、凌晨 4 点逻辑日对齐（[decisions/weekly-boundary-monday](../decisions/weekly-boundary-monday.md)）。
app 提供 GetWeeklyDashboard，store 查询并响应时间线 / 分类失效事件；组件只呈现。
需要的查询由本模块在 storage 中交付，不复制 cards repository 或写第二套分类体系。
本模块不新增 AI 调用，也不将 daily 日记的存在作为聚合前提。

## 实验与失败条件

| 实验 | 输入与操作 | 预期结果 | 失败条件 / 证据 |
|---|---|---|---|
| 合计 / 占比 | 空周、只有 System、混合 Idle 与工作分类 | tracked 排除 System、focus 排除 Idle，分母 0 时 share=0 | 除零、System 计入、分类合计不一致失败 |
| 周边界 | 跨周一、午夜到 4 点、DST 与半小时 / 45 分钟时区 | 周窗口无重叠 / 间隙，使用后端 day 语义 | 前端推算日界、跨周重复或遗漏失败 |
| 分类变化 | 改名、改类、软删卡片，重拉周结果 | 与 cards 当前事实一致，既有排序语义保持 | stale 聚合或重名双计失败 |
| 真实读取 | 已有真实卡片的一周，对照只读聚合与 UI | DTO、时长和占比一致 | fixture 展示不能记为真实集成通过 |

## 实现切片与集成

1. 固定聚合与边界夹具；与 timeline 确认周子能力，基于 cards 契约即可开工。
2. 实现只读 repository 查询与 insight 聚合，完成 08 行为 7 和属性测试。
3. 已决定现有聚合 DTO 的首屏形态：专注比例环、三项时长和分类比例 / 排行；不增加公共字段。
   更丰富的热力图、应用关系或流向图若进入范围，先更新 05 和双侧契约再实现。
4. 接周绑定、store 失效重拉、UI 与双语言空 / 错误 / 加载态。
5. 用真实 cards 独立验收；跨周一的长期证据另行累计，无须等 daily 或完整分析 UI。

## 验收、阻塞与回退

完成要求：周聚合、日期与契约测试通过，真实卡片可查询且 UI 正确，图表决定已记录。
图表未定不阻塞纯聚合；cards 未就绪可使用 fake，但不能标为真实集成通过。
大规模 UI 遵循 G-host。图表 / DTO 子模型由 weekly 产品与设计在 UI 实现前决定。

回退：禁用未验收周入口，保留原始卡片与分类；只读聚合失败不触发任何数据改写。
时区规则回归先撤销该能力变更，不能用改 fixture 期望掩盖问题。

## 验证记录

2026-09-11，在 macOS / Vite 浏览器预览中完成有限前端验证：

- `npm --prefix frontend run typecheck` 与 `npm --prefix frontend run build` 通过；
- 匿名夹具断言 5 个分类、分类分钟合计 1680、占比合计约为 1，周窗口为 7 天；
- 浅色、深色、700×800 窄窗口和中英文界面人工检查通过，页面控制台无错误 / 警告；
- 生产 bundle 不含夹具分类、夹具时间戳或 `/__daygo_dev__/weekly`，无 Wails bridge 的 production
  preview 显示“每周数据能力尚不可用”。

这只证明前端切片和开发夹具路径；绑定行为、聚合正确性、真实卡片、DST / 半小时 / 45 分钟
时区周边界和跨周一观察均未运行。

2026-09-12（周聚合绑定，Go）：`go test ./internal/...`（timeutil 周边界夹具与属性
测试、insight 表驱动、storage 聚合查询、app 端到端周聚合）、`go vet`、交叉构建通过。
周窗口无重叠 / 无间隙拼接（DST 与半时区 / 45 分钟时区）、空周零值、System / Idle
排除、非周一拒绝均有断言。真实卡片一周的独立验收与跨周一长期观察未运行。

2026-09-12（全局界面重构，Vite 预览）：深色中文匿名周报检查通过；专注环改用全局低饱和蓝，
分类序列继续使用独立颜色。真实卡片周与浅色 / 英文矩阵仍未重跑。
