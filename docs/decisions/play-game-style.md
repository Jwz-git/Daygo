# play 玩法选型：方块拼岛（提议）

> 状态：**提议，待用户确认**。本记录只比较候选与调研依据，不代表任何实现已存在。
> 执行册见 [modules/play](../modules/play.md)，范围待决项见 [09 §9.8 第 26 项](../09-roadmap.md#98-待定设计清单)。
> 调研日期：2026-10-13。

## 1. 问题

为提升回访意愿，在 Daygo 内加入一个益智休闲小游戏，内容必须由用户当日 / 当周的时间线卡片生成。
用户在 2026-10-13 确认的边界：

- 接受「每日一关 / 每日一次」与「长期收集 / 养成」两类粘性机制；
- 接受奖励**适度**与工作时长挂钩，以保证内容丰富度；
- 初始想法：(A) 以当日热力图方块为地形做每日闯关；(B) 以卡片换资源做花园 / 农场养成。

## 2. 调研依据

以下为公开资料的观察，多为行业报道或厂商自述，**不是**针对 Daygo 的实验结果。

| 对象 | 观察 | 对本项目的含义（推断） |
|---|---|---|
| Finch（自我照顾 + 宠物养成） | 报道的 D1/D7 留存 54% / 37%，高于 Duolingo 的 51% / 35%；循环是「真实任务 → 资源 → 宠物 / 房间成长」，并强调宠物在用户离开时也在「生活」；评论指出它几乎全是外在激励，养成本身不益智 | 「真实行为 → 可见成长」能形成回访；但纯养成不满足「益智」要求 |
| Forest（专注种树） | 一次专注 = 一棵树，长期累计成林；可见公开的使用规模与可用性研究，未见严格的留存因果研究 | 长期可视化是强钩子，机制可以极简 |
| Streak 研究 | 连续天数完好时能提升行为，但断签后用户更可能放弃，App 高亮断签会加剧；Khan Academy 曾下线 streak；过度奖励效应（Deci 等，1999）在 App 场景多为推断 | 不做会断的 streak；错过的日子不丢东西 |
| Dorfromantik（板块拼接） | 随机性放在「发牌」而非棋盘；短局、规则少、任务驱动（如森林连片）；官方明确不做资源管理、战斗、多人 | 边缘匹配 + 任务足以构成益智休闲；范围可以很克制 |
| 每日谜题（Wordle 式） | 常见做法是日期种子确定性生成，并扫描到满足质量 / 难度阈值的结果 | 由数据生成的关卡要先保证可解 / 可玩，再交给用户 |

## 3. 候选

| 候选（参考） | 与卡片数据的映射 | 智力性 | 单局 | 每日 + 长期 | 实现成本（现栈手写 SVG，无游戏库） | 主要问题 |
|---|---|---|---|---|---|---|
| **① 方块拼岛 / 边缘匹配**（Dorfromantik、Carcassonne） | 时段 → 方块，分类 → 地貌，时间碎片化 → 混合边 | 中 | 3–10 分钟 | 天然兼有 | 中 | 自定义分类需映射到地貌 |
| ② 今日拼图「先解后挖」 | 当日 15 分钟网格 = 已解布局 | 高 | 2–5 分钟 | 每日强、长期弱 | 中 | 数据少时过于简单 |
| ③ 数织 Nonogram | 工作流网格 = 待解图案 | 高 | 5–15 分钟 | 每日 | 中高（需唯一解求解器） | 数据图常非唯一解，需补提示；上手门槛高 |
| ④ 地形闯关 / 寻路（想法 A 原型） | 热力图 = 地形，从 04:00 走到收工 | 中高 | 不可控 | 每日 | 高（可解性与难度曲线） | 分心多的日子关卡更难，形成惩罚感 |
| ⑤ 花园 / 农场放置（想法 B 原型） | 时长 = 资源 | 低 | 1–2 分钟 | 长期强 | 低–中 | 不益智；最直接地奖励时长 |
| ⑥ 三消 / 合成 | 映射牵强 | 中 | 短 | 弱 | 中 | 与数据无实质关系 |

## 4. 提议

- **主玩法采用 ①「拼岛」**：每天的地形来自当天数据（吸收想法 A），每周拼成一座岛并进入图册（吸收想法 B），
  益智点来自边缘匹配与任务规划，而不是来自奖励数量。
- **② 作为可选二期**「今日拼图」，复用 ① 的方块与地貌；由「先生成已解布局再挖空」构造保证必有解。
- ③ ④ ⑤ ⑥ 已评估、不采用，理由见上表。

设计约束（同时写入执行册）：

1. 不做会断的 streak；本周任一天的方块在本周内都可放置。
2. 时长挂钩设软上限（前 16 块 1:1，之后每 2 个时段 1 块，单日上限 24 块），并用「整块专注、休息、计划完成」
   等结构性信号发放特殊地标。
3. 分心不做惩罚性表达；对应地貌提供正向玩法。
4. 无数据日保底可玩。
5. 纯本地运行：不调用 LLM、不联网、不上报。

## 5. 失效条件与回退

- 若匿名夹具显示软上限仍让长工时日显著占优（12 h 与 8 h 工作日方块数差 > 4），重新设计发放规则。
- 若试玩中多数日子的方块因混合边过多而难以放置，降低混合边比例或改为六边形网格，并在此记录。
- 本记录在用户确认前不得改为「已决定」；未确认期间不实现 `internal/play`、迁移或 UI。
- 回退：功能整体由 `play.enabled` 关闭；游戏表与业务数据隔离，关闭不影响时间线、日报与周报。

## 6. 来源

- Finch：[Deconstructor of Fun 小组件拆解](https://www.deconstructoroffun.com/blog/x0hd2ssr80y5n7gv0w967pg7hwd7tl)、
  [Pratt IxD 设计评析](https://ixd.prattsi.org/2026/02/design-critique-finch-self-care-pet-ios-app/)、
  [Appbot 评论分析](https://appbot.co/blog/finch-app-reviews-emotional-attachment-user-retention-product-loyalty/)
- Forest：[Wikipedia](https://en.wikipedia.org/wiki/Forest_(application))、
  [可用性评估论文条目](https://ideas.repec.org/p/sek/iacpro/15516947.html)
- Streak 与动机：[Wharton：Marissa Sharif 访谈](https://knowledge.wharton.upenn.edu/podcast/ripple-effect/easy-as-1-2-3-the-motivating-power-of-streaks-marissa-sharif/)、
  [Northwestern IPR 工作论文 WP-26-05](https://www.ipr.northwestern.edu/documents/working-papers/2026/wp-26-05.pdf)、
  [Network Cultures 评论](https://networkcultures.org/blog/2026/01/19/baby-please-dont-break-the-streak/)
- Dorfromantik 与每日种子：[游戏简介](https://www.jestr.gg/games/dorfromantik)、
  [Steam 评测（发牌逻辑）](https://steamcommunity.com/id/luckz/recommended/1455840)、
  [Dorfperfekt 求解器](https://pypi.org/project/dorfperfekt/0.2.3/)、
  [无后端每日逻辑谜题](https://dev.to/solsticioapp/a-daily-logic-puzzle-with-no-backend-at-all-1ef6)
