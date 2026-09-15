# timeline — 自动时间线

## 用户结果与范围

用户看到由捕获自动产生的按逻辑日活动卡片，可展开帧条、搜索、改标题 / 分类、软删除，
重试失败批次。失败可见，空闲批次不调用 LLM。
负责 U1/2/3、F-A1–6、F-V1–3、F-S7；包括分类管理、媒体资源及既有 timelapse 需求。
不包含每日 / 每周页面、Chat；未有交互与绑定的 review ratings 只跟踪设计，不自动扩展范围。

依据：[03 §3.5](../03-data-model.md#35-时钟串派生)、
[04 §4.3](../04-data-flow.md#43-分析流水线)、
[05 时间线绑定](../05-interface-contract.md#时间线)、[08 行为测试](../08-testing-strategy.md#83-行为测试)。

## 当前状态与证据

实现进度与验证状态以 [09 §9.1](../09-roadmap.md#91-模块总表) timeline 行为准。当前能力快照：

- 存储：cards / categories / batches / observations repository（`ReplaceCardsInRange`
  单事务改写、时钟串三日锚点派生、尝试上限、软删除）；迁移链含各版旧库夹具。
- 分析：两阶段流水线（帧分组转录 → 卡片生成 / 融合，Dayflow 式单卡窗口 +
  `activityPoints`）、分批器、空闲判定、失败分类与自动重排（5 次上限）、
  请求级超时、时区统一为 store `Location()`、融合分类闸门
  （跨分类的模型融合被夹紧回批次窗口，前卡保留）。
- 绑定与前端：`GetTimelineDay`（卡片 / 分类 / 合计 / 失败分组一次带回）、卡片写操作
  （改分类 / 标题 / 摘要 / 软删除）、`RetryBatches` / `DeleteBatches`、分类整体覆盖
  （重命名同事务改写卡片）、失败 / 处理中状态、当前日 15 秒实时跟随与 4 点边界自动
  重拉、帧回放（`GetCardMedia` + `/media/frame`）、周视图（hover 展开、日历选择）、
  卡片审查流。Windows 时区回退（`ZoneName` 注册表回退）已补齐，只影响新读取的页面。
- 开发便利：Vite 开发服务在绑定缺失时提供匿名只读样例（`frontend/dev-fixtures/`），
  页面明确标记"仅开发"；production bundle 不含其 payload。

**已知偏差（provisional）**：帧读取经 app 层 `stagingFrameSource` 而非 `platform.Media`
（#7/#8 未定）；卡片阶段互斥是全局而非按重叠范围。

**已知未修（需先决策再动）**：auth 批次 UI 标志说"不会自动重试"但 `RequeueFailed`
仍会重排（语义需决策）；Retry-After 无抖动（多组同限流时刻齐重试）；转录组 20 图上限
与低图片数网关的错配待配置化；跨 4AM 边界卡片在日视图与聚合中的口径冲突（双计 / 隐形
时段）与用户编辑被相邻批次回滚，需先对 docs/03 §3.5 明确语义归属；`ReprocessDay`
仅保留 docs/05 设计条目，无 Go 方法。

逐日实现与验证细节见下方[验证记录](#验证记录)；本节只维护"当前是什么状态"。

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

2026-09-15：融合分类闸门落地。此前合并决策完全由 prompt 承载，模型对任何相邻活动都倾向
融合（用户反馈"相邻就融合"）。现在卡片阶段对声明的融合（输出 start 早于批次窗口）做确定性
校验：将被吸收的前卡分类必须全部与输出卡一致，`System` 前卡豁免（分类本就未知）；不一致时
夹紧回批次窗口起点、丢弃窗口前 `activityPoints`、前卡保留。prompt 同步告知规则（闸门在 Go
侧强制，不依赖模型自律）。夹具为 pipeline「跨分类融合被闸门拒绝」（Coding 前卡 +
Communication 融合声明 → 两卡并存、点过滤），既有「同分类融合吸收前卡」「System 前卡吸收」
「idle 快路径融合」夹具不回归。契约同步 docs/03 §3.5、docs/04 §4.3.4。真实 LLM 下的融合
质量未验证。

2026-09-15：周视图三批前端修复（长标题单行溢出——flex 冻结选择器误命中无图标卡标题；
clamp 测量顺序——先释放 `-webkit-line-clamp` 再读 `scrollHeight`；hover 展开动画去抖
与幅度收敛——宽度瞬时跳变、上限 80px、超上限卡不反向缩小）经 Vite + mock 绑定浏览器
实测与 `vue-tsc` / production build / check-docs 验证。同日 `ReplaceCardsInRange`
吸收 System 前卡的缺陷修复：重叠谓词删除 System 例外（System 卡唯一实际写入来源是
未知分类回退，保留导致融合后并列占段）；契约同步改写 docs/03 §3.5、docs/05、AGENTS.md；
夹具为 storage「吸收他批 System 卡」（原「保留」期望显式反转）、pipeline「LLM 融合
吸收 System 前卡」（回退旧代码确认失败）与「idle 快路径融合吸收前张 Idle 卡」。

2026-09-14：批失败重试逻辑审查修复（attempt 级请求超时防挂死、`batch:failed` 事件
attempts 差一、failureKind 本地错误归 `internal`、转录失败 note 不带分段路径、
mid-flight 取消不计失败）经 `go test ./internal/ai/... ./internal/analysis/...
./internal/app/...`、`go vet`、`CGO_ENABLED=0 go build` 验证；夹具含挂死 attempt
超时重试、取消不计数、failureKind 全类目表。同日 Windows `ZoneName` 注册表回退与
当前日 15 秒实时跟随 / 4 点边界自动重拉落盘，前端不推导逻辑日。

2026-09-13：卡片生成缺陷修复批次通过 Go 单元测试（`go test ./internal/analysis/
./internal/storage/ ./internal/timeutil/`）。夹具覆盖：`10:21AM` 粘着时钟整批路径
（否则模型偏差导致永久失败循环）、批次尝试上限逐周期拒绝（配合 v8 → v9 迁移夹具
证明旧库保数据且 attempts 从 0 起）、observations 重写幂等、UTF-8 截断不切断多字节
rune、apps 元数据解析。失败面板排除 skipped_short 的语义变更已同步
`TestFailedBatchesInRange` 期望（显式决定：skipped_short 是正常终态）。

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
在当时的开发阶段，真实 cards 绑定尚未交付，因此开发夹具中的
日期导航保持禁用；这些改动不构成真实编辑、重试或聚合完成证据。

2026-09-11：按既有公共契约补齐卡片改标题 / 改分类（分别提交，避免伪装成原子更新）、软删除
二次确认、失败范围重试、整日重处理、时间线复制和后端视频 URL 播放路径。所有写操作要求
`features` 含 `timeline`、实例持有写锁且对应绑定存在；写后不乐观更新，等待
`timeline:updated` 重拉。浏览器开发夹具只用于检查禁用态、详情层、短卡片和复制反馈，不能
验证真实写入、事件顺序、媒体解码或分析恢复。

2026-09-14（失败重试入口，前端）：修复手动重试按钮不可点——前端曾把 `retryable`（仅表示
"是否会自动重排"，`auth` / `invalid_request` / `no_provider` 或 attempts 达上限时为 false）
误用为手动重试的禁用条件，而 Go `RetryBatches` 刻意无视失败类型与 attempt 上限并重置
attempts。现汇总面板不再按 `retryable` 过滤失败条目、详情按钮只受写锁与绑定探测约束；
`retryable` 仅用于提示文案（docs/05 §5.5.2 注释同步）。同日移除整日重处理前端入口
（按钮、store action、API wrapper、i18n 文案与 settings 描述），后端 `ReprocessDay`
本就未交付，设计条目保留在 docs/05。

2026-09-12（查询与卡片写操作绑定，Go）：`go test ./internal/app/`、`go vet`、
`CGO_ENABLED=0 go build ./...` 通过。空日 / 有卡日（metadata 解析、合计排 System、
Idle 单列）、非法 day、未知分类、卡片不存在、只读实例拒绝、写后事件合并（同 day
三次写一条 `timeline:updated`）、失败 60 秒容差分组均有断言。`wails dev` 真机端到端
（真实库写入 → 事件刷新）未运行。
