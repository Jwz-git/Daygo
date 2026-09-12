# timeline — 自动时间线

## 用户结果与范围

用户看到由捕获自动产生的按逻辑日活动卡片，可展开帧条、搜索、改标题 / 分类、软删除，
重试失败批次或重处理一天。失败可见，空闲批次不调用 LLM。
负责 U1/2/3、F-A1–6、F-V1–3、F-S7；包括分类管理、媒体资源及既有 timelapse 需求。
不包含每日 / 每周页面、Chat；未有交互与绑定的 review ratings 只跟踪设计，不自动扩展范围。

依据：[03 §3.5](../03-data-model.md#35-时钟串派生)、
[04 §4.3](../04-data-flow.md#43-分析流水线)、
[05 时间线绑定](../05-interface-contract.md#时间线)、[08 行为测试](../08-testing-strategy.md#83-行为测试)。

## 当前状态与证据

实现进度：部分实现。已有时间函数单元通过；可接真实 DTO 的小时轨道、卡片、失败 / 处理中
状态和详情检查器已落盘。**2026-09-12：查询与卡片写操作绑定已落盘**——
`GetTimelineDay`（一次带回卡片 / 分类 / 合计 / 失败分组，metadata 宽容解析）、
`UpdateCardCategory`（未知分类拒绝、不自动创建）、`UpdateCardTitle`、`DeleteCard`（软删除），
写后发按 day 合并（200 ms）的 `timeline:updated`；只读实例返回 `not_capture_owner`；
`features` 含 `timeline`。
**2026-09-12：分析流水线初版已落盘**——`internal/analysis`
（分批器含差一间隔算术、空闲判定、两阶段转录/卡片流水线：帧分组转录为 observations
（模型只引用帧序号）、卡片阶段在互斥区内读→生成→改写（45 分钟滑动窗上下文、
未知分类归 System、窗外卡片丢弃）、`ReplaceCardsInRange` 单事务提交）、
storage v8（`batch_screenshots` / `observations` + v7 夹具）、`AnalysisRepo`
（未分批查询 / 批次状态机 / 收养 / 冷却重排 / observations 读写）、
`GetTimelineDay` 填充 `ProcessingRanges`；卡片提交自动触发既有 `timeline:updated`，
批次失败发既有 `batch:failed`。启动自动录制随本切片落盘（无 provider 不录）。
Go 单元覆盖：分批 / 空闲逐边界、六条流水线路径（正常 / 空闲零调用 / 失败 / 空 / 短批 /
取消保持 processing）、auto-start 三重防呆。
已知偏差：帧读取经 app 层 `stagingFrameSource` 而非 `platform.Media`（#7/#8 未定，
provisional）；卡片阶段全局互斥（比按重叠范围粗）；`wails dev` 真机端到端未运行。
失败批次重试（`RetryBatches`）、整日重处理（`ReprocessDay`）与视频 URL 仍无 Go 方法
（依赖媒体切片），前端按方法探测自动禁用对应入口。
[timeutil](../../internal/timeutil/timeutil.go)、[日期绑定](../../internal/app/backend.go)
和 [时间线前端切片](../../frontend/src/views/Timeline/TimelineView.vue) 已落盘。
**2026-09-12：cards 存储切片已落盘**——迁移 v2（`analysis_batches` / `timeline_cards` /
`categories`，含 System / Idle 种子与 v1 夹具）、`internal/domain` 卡片值类型、
`timeutil.ResolveClock`（三日锚点 + 跨午夜 + 五时区夹具）、`storage.CardRepo`
（查询 / 更新 / 软删除 / `ReplaceCardsInRange` 单事务改写，保留其它批次 System 卡片，
SkippedCards 计入诊断）。Go 单元覆盖时钟派生四规则、DST、并发重叠与分类重命名事务。
生产页在 `GetTimelineDay` 缺失时明确显示不可用，不返回 fixture 数据。Vite 开发服务会在绑定
缺失时从 `frontend/dev-fixtures/timeline.json` 提供一组匿名只读样例，并在页面上明确标记
“仅开发”；夹具位于 `src` 外且 production bundle 不包含其 payload 或请求路径。

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
聚合；卡片、批次、observations、分类、llm_calls 和 review ratings 的 schema / repository
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

2026-09-12：cards 存储切片通过 Go 单元测试（`go test ./internal/storage/
./internal/timeutil/`）与 `CGO_ENABLED=0` 的 macOS / Linux / Windows 构建。夹具覆盖：
v1 → v2 迁移保数据（DB-2）、`ResolveClock` 三日锚点 / 跨午夜 / meridiem 与 24 小时
格式 / DST（Lord Howe）/ 半小时与 45 分钟时区 / 畸形输入、`ReplaceCardsInRange`
派生与插入 / 近午夜跨日 / 跳过卡片计数（喂 `NoteSkippedCards`）/ 保留其它批次
System 卡片 / timelapse 路径回收 / 并发重叠最终一致（busy 视为合法重试信号）、
分类种子 / 整体覆盖 / 重名拒绝 / 重命名同事务改写卡片。这只证明存储层行为，
不证明绑定接入、分析流水线或真实闭环。

2026-09-10：已有 timeutil 和绑定 Go 测试通过，见 [基线](../09-roadmap.md#当前代码证据)。

2026-09-11：时间线前端切片通过 `vue-tsc --noEmit` 与 Vite production build；用一次性匿名
浏览器夹具人工复核浅 / 深主题、1440×900 双栏、窄窗折叠、小时刻度、短卡片、长空白、
失败 / 处理中区间、分类筛选与详情切换。production bundle 已检查不含夹具哨兵文本。
这只证明前端呈现与状态边界，不证明 cards、媒体、写操作、真实闭环或长期稳定性。

2026-09-11：前端视觉收敛为系统字体、中性窗口材质、低阴影和无位移悬浮反馈；删除逐项入场、
漂浮、放大、光晕与无限 shimmer。Vite 开发服务增加可删除的匿名时间线夹具，浏览器人工复核
浅 / 深主题、时间比例、详情选中和开发数据标记；production build 检查不含样例活动与 dev
fixture endpoint。这是开发验收便利设施，不构成生产数据或 G-loop 证据。

2026-09-11：时间轨道增加最小点击高度碰撞分栏与首次进入的相关时刻定位；日期选择写入路由并
交回 `GetDayContext` 解析，Timeline 与 Daily 切页时保留同一日期键。匿名时间线夹具补充相邻
4 分钟卡片及空闲卡片，用于复核碰撞与空闲摘要。真实 cards 绑定仍未交付，因此开发夹具中的
日期导航保持禁用；这些改动不构成真实编辑、重试或聚合完成证据。

2026-09-11：按既有公共契约补齐卡片改标题 / 改分类（分别提交，避免伪装成原子更新）、软删除
二次确认、失败范围重试、整日重处理、时间线复制和后端视频 URL 播放路径。所有写操作要求
`features` 含 `timeline`、实例持有写锁且对应绑定存在；写后不乐观更新，等待
`timeline:updated` 重拉。浏览器开发夹具只用于检查禁用态、详情层、短卡片和复制反馈，不能
验证真实写入、事件顺序、媒体解码或分析恢复。

2026-09-12（查询与卡片写操作绑定，Go）：`go test ./internal/app/`、`go vet`、
`CGO_ENABLED=0 go build ./...` 通过。空日 / 有卡日（metadata 解析、合计排 System、
Idle 单列）、非法 day、未知分类、卡片不存在、只读实例拒绝、写后事件合并（同 day
三次写一条 `timeline:updated`）、失败 60 秒容差分组均有断言。`wails dev` 真机端到端
（真实库写入 → 事件刷新）未运行。
