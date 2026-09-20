# timeline — 自动时间线

## 用户结果与范围

用户看到由捕获自动产生的按逻辑日活动卡片，可展开帧条、搜索、改标题 / 分类、软删除，
重试失败批次、按卡片所属批次重处理，并对卡片记录专注 / 中性 / 分心判断。失败可见，空闲批次不调用 LLM。
负责 U1/2/3、F-A1–6、F-V1–3、F-S7；包括分类管理、媒体资源及既有 timelapse 需求。
不包含每日 / 每周页面、Chat；卡片审阅判断属于 timeline，其他尚无交互与绑定的目标数据只跟踪设计，
不自动扩展范围。

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
  （改分类 / 标题 / 摘要 / 软删除）、`RetryBatches` / `DeleteBatches` / `ReprocessDay` /
  `ReprocessCard`（后者按卡片来源批次重排，同批次卡片会共同重建）、分类整体覆盖
  （重命名同事务改写卡片）、分类管理向导（未改动的默认分类按界面语言显示，编辑并保存即
  改写为本地文案）、失败 / 处理中状态、当前日 15 秒实时跟随与 4 点边界自动
  重拉、帧回放（`GetCardMedia` + `/media/frame`）、周视图（hover 展开、日历选择）、
  卡片审查流；审阅判断已持久化，支持在详情页读取 / 修改并刷新当日统计。Windows 时区回退
  （`ZoneName` 注册表回退）已补齐，只影响新读取的页面。
- 开发便利：Vite 开发服务在绑定缺失时提供匿名只读样例（`frontend/dev-fixtures/`），
  页面明确标记"仅开发"；production bundle 不含其 payload。

**已知偏差（provisional）**：帧读取经 app 层 `stagingFrameSource` 而非 `platform.Media`
（#7/#8 未定）；卡片阶段互斥是全局而非按重叠范围。

**已知未修（需先决策再动）**：auth 批次 UI 标志说"不会自动重试"但 `RequeueFailed`
仍会重排（语义需决策）；Retry-After 无抖动（多组同限流时刻齐重试）；转录组 20 图上限
与低图片数网关的错配待配置化；跨 4AM 边界卡片在日视图与聚合中的口径冲突（双计 / 隐形
时段）与用户编辑被相邻批次回滚，需先对 docs/03 §3.5 明确语义归属。

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

2026-09-16（分类管理的本地化）：分类管理向导此前直接渲染 `categories` 原文，中文界面下六个默认
分类的标题与描述是英文——种子文案按设计是给模型匹配的数据，缺的是显示层本地化。现在
`categoryLabel.ts` 在原有名称表之外增加出厂描述表与 `categoryDetails()`：只对**仍保持种子原文**
的行显示本地化文案，用户改写过的行原样显示。向导的只读行与编辑框预填同一份文案，因此
「打开编辑 → 保存 → 完成」即把该分类从种子文案改写为用户文案（走既有重命名事务，历史卡片同步
改写）；没打开过编辑的行原样回传，不会因一次浏览被批量改写。夹具：`categoryLabel.test.ts` 补四条
分支（未改动 / 描述被改写 / 已改名 / 自定义）；新增 `categoryDefaults.test.ts` 从
`internal/storage/migrate.go` 解析 v12 种子与前端描述表逐字对账（改动一侧字符串确认失败），并断言
两份 bundle 都有键、en 等于种子原文、zh 不是英文原文。浏览器验证（Vite 独立预览 + 注入绑定桩、
分类取迁移种子）：中文界面行显示「工作」等加中文描述，编辑框预填同一文案，`SaveCategories` 收到的
载荷里只有被打开并保存的那一行变成中文，其余五行保持英文 / 用户原文；en 界面逐字等于种子文案。
`npm --prefix frontend run test:unit`（50 项）、`typecheck`、production build 与
`scripts/check-docs.py` 通过；契约同步 docs/03 §3.3.3。真实 Wails 窗口与真实库上的保存、卡片改写
未复核。

2026-09-20：修复批次因末帧仍在活跃录制分段而反复失败（`frameDecode: failed with status -3`，即 `DG_CAPTURE_E_UNSUPPORTED`）的问题。这是 `-7`（残缺分段缺 moov）之外的第二条时序失效：分段冻结（600 帧 / 600 秒 / 尺寸变化）与批次封口（间隔或目标时长）是两条独立边界，因空闲间隔提前封口或跨满 15 分钟的批次，其末帧可能仍落在录制器正在写、尚未收尾的分段里，此刻解码必然失败；批次失败进 `FailureRetryCooldown`（10 分钟）冷却，等分段轮换收尾后自动重试才成功——即用户观察到的"重试几次就好"。区别于 `-3` 之外的活跃分段既不缺 moov 也非损坏，无法用 `Readable`/moov 探测区分，故改用纯 Go 信号：`Recorder.ActiveSegmentPath()` 暴露当前正在写入的分段路径（暂停 / 停止 / 收尾后清空），分析调度器 `Config.ActiveSegment` 读取它；`processBatch` 在置 `processing` 前先判定批次任一帧是否属于活跃分段，若是则返回 `errBatchDeferred`，批次**保持 `pending`、不计尝试、不触发 `batch:failed`**，下一轮分段收尾后自然处理。夹具 `TestPipelineDefersBatchInActiveSegment`（活跃时推迟、清空后成功、零 provider 调用、零尝试计数）；`go test ./internal/analysis/... ./internal/recorder/...`、`go vet`、`CGO_ENABLED=0 go build ./internal/...` 全通；契约同步 docs/04 §4.3.2。真实 macOS 长期录制闭环未复核。

2026-09-18：修复卡片重新生成时下方相邻卡片底色被错误渲染为彩色的问题，并增强录制分段收尾与未完成分段对齐恢复。此前时间线使用像素矩形几何重叠（`boxesOverlap`）来判定卡片是否处于重新生成状态（`regenerating`），导致被 `MIN_CARD_HEIGHT` 撑到 34px 的短卡片或批次在垂直像素上压入下方相邻卡片，使其错误带上 `is-regenerating` 渐变彩底。现增加 `cardIntersectsRanges` 纯函数改由真实时间戳交集（时间重合度大于 0）精确判断卡片是否属于重分析批次，下方相邻卡片保持正常底色不变。同时在 `SegmentWriter.swift` 添加 `atexit` 自动收尾勾子并在 Wails 与系统信号中断时触发 `backend.shutdown()` 保证录制分段写入 moov atom，并在 `Reconcile` 启动时自动检查已提交分段，将缺失 moov atom 的残缺分段置为 `is_deleted = 1`，彻底杜绝 `frameDecode: failed with status -7` 拖垮整个分析批次。夹具：`timelineCoverage.test.ts` 与 `captures_test.go` 分别新增测试；`./scripts/gate.sh` 门禁全通。

2026-09-16：修复「重新分析这一天」后时间线上「生成中」区块与旧卡片重叠。`ReprocessDay` 把当天
终态批次改回 `pending` 时**不删除已有卡片**，于是 `processingRanges` 与 `cards` 同时覆盖同一
窗口：日轨道把区块画成整行绝对定位盒（z-index 2），卡片（z-index 3）落在同一矩形上，两层叠在
一起；周栅格同样。现在按「一个窗口只有一个主人」处理——被卡片覆盖的窗口不再画区块，改由该卡片
进入重新分析态（`is-regenerating` 渐变底 + `aria-busy`），未被覆盖的窗口仍显示区块，所以首次
分析的空窗体验不变。规则落在 `layout.ts` 的 `boxesOverlap` / `coveredBy` / `uncoveredBy`，
日轨道与周栅格共用同一组纯函数。判据用**实际绘制的盒子**而非时间戳：4 分钟卡片被
`MIN_CARD_HEIGHT` 撑到 34px 后会压到下一个窗口，只看时间戳会漏。夹具
`frontend/tests/timelineCoverage.test.ts`（7 项，含周列同规则用例）；把 `weekLayout` 退回旧行为
确认该用例失败。浏览器验证：vite 夹具临时注入 processingRanges（覆盖卡片、部分重叠、空窗三种），
全日 69 张卡与区块矩形零相交，3 张被覆盖卡片带 `is-regenerating` 与 `aria-busy`，随后夹具已还原。
`./scripts/gate.sh` 通过。真实 reprocess 与真实 LLM 下的观感未复核。

2026-09-16：修复卡片 appSites 在真实链路上丢失。卡片阶段把模型输出的扁平列表
（`["Code","github.com"]`）原样写进 `timeline_cards.metadata`，绑定层却按 docs/05 §5.5.2 的
`{primary, secondary}` 对象解析，`json.Unmarshal` 的类型错误让 `parseCardMetadata` 整体返回空值
——appSites、distractions、activityPoints 三项在真实数据上一起丢失，前端 `AppSiteIcon` 因此从未
渲染。此前两处夹具各自自洽掩盖了这条缝：DB-5 与 binding 用例都用对象形状，analysis 用例用扁平
列表，没有任何夹具跨过生产者→消费者。现在由 `appSitesFromList` 在生产者侧完成映射；
`dropPreWindowPoints` 的解码结构同步改形状（该函数回写整个 metadata，字段解不开就会静默不再
过滤窗口前时间点）。夹具：pipeline happy path 断言存储字节是契约对象形状，新增 binding
`TestCardMetadataWrittenByAnalysisPipelineParses` 用生产者实测字节回灌绑定层，DB-5 夹具改为
对象形状——期望值变更是一次显式决定，不是"测试挂了就改期望"。`go test ./...`、`go vet ./...`、
`CGO_ENABLED=0 go build ./...` 通过。真实 LLM 与真实录制数据下的图标显示仍未验证。

2026-09-15：修复 Windows 窗口重新获得焦点时的时间线加载闪屏。此前标题栏点击触发
`window.focus` 后使用全量 `load()`，会暂时将已有时间轴替换为首次加载面板；现在窗口焦点与
可见性恢复均使用 silent refresh，保留当前页面并在后台重拉。延迟后端夹具验证请求未完成时
store 仍保持 `loading=false` 和既有页面状态；`npm --prefix frontend run test:unit`（35 项）、
`npm --prefix frontend run typecheck` 与 production build 通过。尚未在 Wails 窗口中做人工复核。

2026-09-15：融合分类闸门落地。此前合并决策完全由 prompt 承载，模型对任何相邻活动都倾向
融合（用户反馈"相邻就融合"）。现在卡片阶段对声明的融合（输出 start 早于批次窗口）做确定性
校验：将被吸收的前卡分类必须全部与输出卡一致，`System` 前卡豁免（分类本就未知）；不一致时
夹紧回批次窗口起点、丢弃窗口前 `activityPoints`、前卡保留。prompt 同步告知规则（闸门在 Go
侧强制，不依赖模型自律）。夹具为 pipeline「跨分类融合被闸门拒绝」（Coding 前卡 +
Communication 融合声明 → 两卡并存、点过滤），既有「同分类融合吸收前卡」「System 前卡吸收」
「idle 快路径融合」夹具不回归。契约同步 docs/03 §3.5、docs/04 §4.3.4。真实 LLM 下的融合
质量未验证。

2026-09-18：卡片生成速率限制与步频防护（TPM 限制防击穿）：转录引入等距帧采样（`DefaultSampledFrames = 15`，对齐 Dayflow 原版实现），单批请求由多次图片分组聚合为单次请求，输入 Token 消耗降低 >80%；调度器加入批次间步频控制（`DefaultBatchPacing = 10s`）与单 worker 串行化，遇 rate_limited 即刻熔断本轮排队；无 Retry-After 的 429 错误加入 long backoff 退避机制（15s 起算，封顶 30s），防止快速重试耗尽 attempt。

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
