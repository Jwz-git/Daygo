# timeline — 自动时间线

## 用户结果与范围

用户看到由捕获自动产生的按逻辑日活动卡片，可展开帧条、搜索、改标题 / 分类、软删除，
重试失败批次、重新生成单张卡片，对卡片记录专注 / 中性 / 分心判断，并对其摘要投拇指评分。
失败可见，空闲批次不调用 LLM。
负责 U1/2/3、F-A1–6、F-V1–3、F-S7；包括分类管理、媒体资源及既有 timelapse 需求。
不包含每日 / 每周页面、Chat；卡片审阅判断属于 timeline，其他尚无交互与绑定的目标数据只跟踪设计，
不自动扩展范围。

依据：[03 §3.5](../03-data-model.md#35-时钟串派生)、
[04 §4.3](../04-data-flow.md#43-分析流水线)、
[05 时间线绑定](../05-interface-contract.md#时间线)、[08 行为测试](../08-testing-strategy.md#83-行为测试)。

## 当前状态与证据

> **验收状态**：已实现能力于 2026-09-22 经用户确认已验收；无逐项运行记录。未实现能力见 [09 §9.1](../09-roadmap.md#91-模块总表)。

实现进度与验证状态以 [09 §9.1](../09-roadmap.md#91-模块总表) timeline 行为准。当前能力快照：

- 存储：cards / categories / batches / observations repository（`ReplaceCardsInRange`
  单事务改写、时钟串三日锚点派生、尝试上限、软删除）；迁移链含各版旧库夹具。
- 分析：两阶段流水线（帧分组转录 → 卡片生成 / 融合，首批单卡、持续窗口按活动证据重分 +
  `activityPoints`；每张卡 15–60 分钟，只有承载体改写窗口末端的那张卡可以更短，不足下限的
  片段并入邻卡且**跨分类也并**，合并卡取占多数时间的活动分类）、分批器、空闲判定、
  失败分类与自动重排（5 次上限）、
  请求级超时、时区统一为 store `Location()`、融合分类闸门
  （跨分类的模型融合被夹紧回批次窗口，前卡保留；横跨批次起点的前卡无条件拥有；
  该闸门不受 15 分钟下限放松）、
  融合卡片继承被吸收前卡的 `appSites`（模型未点名应用时回填，避免图标消失）。
- 绑定与前端：`GetTimelineDay`（卡片 / 分类 / 合计 / 失败分组一次带回）、卡片写操作
  （改分类 / 标题 / 摘要 / 软删除）、`RetryBatches` / `DeleteBatches` / `ReprocessDay` /
  `ReprocessCard`（后者同步重写该卡片**自己的时间窗**，复用窗内已存 observations、
  不重新转录取图，两侧相邻卡不动，详见 [04 §4.3.5](../04-data-flow.md#435-单卡重写)）、分类整体覆盖
  （重命名同事务改写卡片）、分类管理向导（未改动的默认分类按界面语言显示，编辑并保存即
  改写为本地文案）、失败 / 处理中状态、当前日 15 秒实时跟随与 4 点边界自动
  重拉、帧回放（`GetCardMedia` + `/media/frame`）、周视图（hover 展开、日历选择）、
  卡片审查流；审阅判断已持久化（`card_reviews`），支持在详情页读取 / 修改并刷新当日统计；
  摘要拇指评分已持久化（`card_ratings`），详情页可投上 / 下并再次点击撤销，两者都不改写卡片，
  因此都不发 `timeline:updated`。Windows 时区回退
  （`ZoneName` 注册表回退）已补齐，只影响新读取的页面。
- 开发便利：Vite 开发服务在绑定缺失时提供匿名只读样例（`frontend/dev-fixtures/`），
  页面明确标记"仅开发"；production bundle 不含其 payload。

**已知偏差（provisional）**：帧读取经 app 层 `stagingFrameSource` 而非 `platform.Media`
（#7/#8 未定）；卡片阶段互斥是全局而非按重叠范围。

**已知未修（需先决策再动）**：auth 批次 UI 标志说"不会自动重试"但 `RequeueFailed`
仍会重排（语义需决策）；Retry-After 无抖动（多组同限流时刻齐重试）；转录组 20 图上限
与低图片数网关的错配待配置化；用户编辑被相邻批次回滚，需先对 docs/03 §3.5
明确重处理语义归属。

验证摘要见下方[验证记录](#验证记录)。

## 能力与跨层职责

| 输入 | 可先推进 | 真实接入条件 |
|---|---|---|
| recording: capture / media-read | 匿名帧索引、分段与解码 fake | G-data、真实持久帧与分段可读 |
| providers: provider-client | 固定 LLM 响应测试分批、空闲、解析与重处理 | 用户配置服务真实可用、取消和重试契约一致 |
| data: db-core / diagnostics | 固定卡片库设计事务、查询和诊断计数 | 迁移 / 所有权 / 事务通过，失败计数可查询 |
| preferences: settings-access / ui-bridge | DTO、事件与 store fixture | G-host、真实绑定和生成 DTO |

输出 time 与 cards，可分别供 daily / weekly 验收，不等时间线页面全部完善。
internal/analysis 拥有调度、媒体准备、批次路由粘性与范围串行化；internal/ai 拥有统一文本 /
图片 / JSON Schema 调用、协议、重试、提示词与解析，且不读取分段路径；internal/insight 只读
聚合；卡片、批次、observations、分类、llm_calls 和审阅 / 评分（`card_reviews`、
`card_ratings`）的 schema / repository
统一落 internal/storage。llm_calls 只记录 attempt 元数据，不保存模型正文。本模块完善
internal/timeutil。
internal/app 负责绑定与数字 ID 资源入口，平台 Media 读像素；媒体缓存有界。
分类设置归本模块；通知 / Provider / 录制设置继续归各自功能。

## 实验与失败条件

| 实验 | 输入与操作 | 预期结果 | 失败条件 / 证据 |
|---|---|---|---|
| 时间 / DB-5 | 五时区、03:30–04:30、DST、三日锚点、跨午夜、解析失败 | 最近窗口解析、4 点 day、合法跨度、SkippedCards 被消费计数 | 静默跳过、日期错位、System 卡片误删失败 |
| 分批 / 空闲 | 边界尾批、缺帧、整批空闲和混合活动 fixture | 批次不重不漏，纯空闲零 LLM 调用 | 停滞、重复、真实活动错为空闲失败 |
| 事务 / 并发 | 两任务范围重叠、主备失败切换、重处理、分类重命名 | 范围串行化、回退粘性、succeeded 唯一成功终态、事务一致 | 重复卡片、越界覆盖、分类失联失败 |
| LLM 输出 | 匿名畸形 JSON、错误类型、额外指令、未知分类 | 仅解析数据、按规范修复 / 报错，未知分类不创建 | 执行模型指令或创建未知分类失败 |
| 资源 / UI | 数字 ID、路径越界、缺段 / 解码失败、200 张卡片和懒加载 | 白名单参数、正确错误与缓存、帧按需读取 | 路径泄漏、UI 主线程阻塞或无界缓存失败 |
| 用户闭环 | 真实捕获和 Provider 连续 7 天，实际操作编辑与重试 | 卡片来自真实输入、无未解释缺口、失败可见可恢复 | fixture 卡片不构成 G-loop 证据 |

## 实现切片与集成

1. 固定时间、分批、空闲、解析、事务夹具；补完整 time 能力并先提供给 daily / weekly。
2. 在 internal/storage 交付卡片与分类、批次等 repository，独立验收 cards；
   ReplaceCardsInRange 单事务保持 03 §3.5 四项规则，SkippedCards 必须计入诊断。
3. 用 Capture / Provider / Media fake 打通“解码匿名帧 → 结构化 observations → 结构化 cards”
   两段分析流水线、取消、重试、批次内 fallback 粘性与范围串行化；整批空闲时保持零 LLM 调用；
   接真实能力后逐段验证，所有 goroutine 有所有者与退出路径。
4. 接时间线 / 分类 / 媒体绑定、资源处理器、store、事件及 UI；写后等事件重拉。
   单独交付 Media.EncodeVideo / 有界缓存并与 recording 协调公共端口和编码决策。
5. 与 recording / providers 跑 G-loop；与 daily / weekly 以 cards 查询做独立集成；
   14 天和真实日期边界按 G-stability 另记。

## 验收、阻塞与回退

完成要求：时间线真实用户闭环、行为 / 事务 / 资源 / 双侧契约通过，G-loop 通过；
长时间稳定性另列状态，7 天不代替 14 天和 DST。
fake 能证明确定性逻辑，不能证明 LLM 文本一致、真实截图或网络质量。
上游未就绪时可做 time、cards、解析及聚合；大规模 UI 受 G-host 限制。

待决：解析失败的用户提示（禁止静默丢弃）、统一重试体验归 timeline 产品；
视频编码协同 recording，性能标准按现有风险 M-2 验证；新交互先补 01 / 05。
回退：停止新增分析并取消所属任务，保留捕获、批次和原有卡片；
事务改写失败不提交，不以删除卡片重建的方式回退。schema 回退遵循 data 的备份恢复策略。

## 验证记录

- **跨 4 点时间片（2026-09-23）**：匿名 03:30–04:30 卡片夹具先复现前一日计成 60 分钟；
  修复后 `CardsForDay` 在相邻两日均可查到同一原始卡片，`GetTimelineDay` 每日只展示并
  统计 30 分钟，周明细按 4 点分段，范围总量取交集；另覆盖纽约 DST 回拨、半小时与 45 分钟时区。
  `go test ./internal/storage ./internal/app ./internal/insight`
  通过；真实历史库与长时间跨日运行尚未单独复核。
- **存储与分析（2026-09-12—21）**：`go test ./internal/storage/ ./internal/analysis/ ./internal/app/`、`go vet`、`CGO_ENABLED=0 go build ./...` 及迁移夹具覆盖卡片事务改写、分类、批次重试、范围边界、单卡重写、appSites 回填和评分。具体算法与约束见 [03 §3.5](../03-data-model.md#35-时钟串派生) 和 [04 §4.3](../04-data-flow.md#43-分析流水线)。
- **前端（2026-09-11—21）**：typecheck、单元测试与 build 覆盖日 / 周视图、编辑、重处理状态、图标、分类本地化和详情页审阅 / 评分；Vite 预览使用匿名夹具。评分迁移为 v18，`card_ratings` 仅保存 `up` / `down`，再次点击撤销；审阅和评分都不改写卡片。
- **原生与闭环**：历史记录包括 macOS 真实像素及分段 smoke；真实 Provider、Wails 时间线与长期观察由用户于 2026-09-22 确认验收，未附逐项运行记录。搜索尚未实现，见 [09 §9.1](../09-roadmap.md#91-模块总表)。
