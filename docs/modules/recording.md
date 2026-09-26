# recording — 常驻录制

## 用户结果与范围

用户授权后可开启、关闭或定时暂停记录；关窗后继续离散截图，状态栏可查看状态并重开窗口。
屏蔽应用既从截图排除，前台命中时又生成脱敏占位帧；睡眠、锁屏、屏保、退出的分段安全收尾。
包含间隔 / 分辨率、屏蔽名单与原生应用选择器、自启和 Dock 设置。
负责 U6/8/10、F-C1–9、F-S5/8、F-L1–3；不包含 AI 分析、时间线页面或 Windows 发布承诺。

公共依据：[04 §4.1](../04-data-flow.md#41-捕获流水线)、
[05 §5.7](../05-interface-contract.md#57-b4platform-端口契约)、
[06](../06-native-integration.md)、[截图 v2 实现与调用](../decisions/recording-screen-capture-v2.md)、
[Windows 截图实现与限制](../decisions/recording-screen-capture-windows.md)、
[图片存储流水线](../decisions/recording-image-storage.md)、
[开机自启方案](../decisions/recording-launch-at-login.md)、
[07 §7.2](../07-privacy-security.md#72-捕获侧的两层保护)。
实机矩阵：[08 §8.6.2 MC](../08-testing-strategy.md#862-mc真实-macos-捕获矩阵)、
[§8.6.3 WC](../08-testing-strategy.md#863-wc真实-windows-捕获矩阵)。

## 当前状态与证据

2026-09-26 Dock 重新启用报错与反馈布局修复（本次增量）：用户报告关闭后正常、重新启用
弹出截断正文、英文占位复选框和空按钮。macOS 26.0.1 / arm64、基于 `ff573bf` 的修复工作树：
匿名 AppKit 宿主实测重复设置当前激活策略返回 false、实际策略保持目标值；旧桥将其误判为 -2。
桥现在只在实际策略不同才设置，并以主线程读回值判断成功；仍不匹配才报失败。
直接显示 `NSAlert.window` 前补 `layout()`，保留非阻塞与原有按钮动作；
[Apple NSAlert 文档](https://developer.apple.com/documentation/appkit/nsalert/layout())说明其用于立即布局。

夹具先行：新增重复 accessory 断言在旧桥失败；修策略后，渲染控件断言在旧弹窗失败。
修复后 `bash native/darwin/system-smoke.sh` 的独立可执行程序 / 匿名 `.app` 均通过：可见窗口
regular → accessory → regular、主 / 工作线程重复设置、宿主先恢复 regular、正文不裁切、
恰好一个按钮及点击关闭 / System 清理。Go 匿名库夹具覆盖 false → true → false → true 的
设置保存 / 读回与策略，同时保持前台状态。既有夹具期望未放宽。
`native/darwin/build.sh` universal 重建、
`go test -a ./internal/platform/darwin -run '^TestSystem' -count=1`、
`CGO_ENABLED=0 go test ./internal/app -run 'Test(DockPreference|UpdateSettingsAppliesDockPreference|FailedActivationPolicy|ExplicitReopen)' -count=1`
及 `./scripts/gate.sh` 通过（169 项前端测试、三平台无 cgo 构建、文档 0 问题；
Windows 安装器 4 项通过 / 5 项限 Windows 跳过）。`gofmt -l .` 无输出。

范围：recording 宿主 / preferences Dock 消费者；属于 G-host 增量回归，不扩大正式分发身份验收。
用户未注意报错时图标是否恢复，故尚不能确定其那次报错必然来自重复设置；本次修复后真实
Wails 开关 / 重启读回和持续捕获待回归，运行中的已安装应用未替换。所有原生夹具不读用户库或屏幕。
回退：整体回退本修复的桥、提示布局、夹具与契约，并重新构建原生库；不修改数据库或偏好键。

2026-09-26 菜单栏 / Dock 增量：macOS Dock 开关已在启动与设置持久化后应用，重开尊重偏好，
恢复入口不可用保留 regular，失败策略不缓存成功并可重试。状态栏首行与五类图标区分启动、
录制、暂停、空闲和警告；只读 / 不可用禁用操作，系统阻塞不允许手动恢复，定时暂停显示恢复
时刻。所有菜单动作消费错误并显示脱敏、本地化非阻塞提示，不阻塞系统事件泵；普通退出收尾
失败默认保留应用，可明确选择仍然退出。原生相同快照不重建菜单，暂停时长项平铺。
主应用 / 编辑 / 窗口菜单 20 项标题保留 selector 与快捷键，Cmd+Q 明确“留在后台继续记录”。
原生在 Wails 启动完成后重应用已下发策略，避免启动 regular 覆盖已保存的 Dock 关闭偏好。
Dock、菜单与错误文案全九种语言覆盖；recorder 的暂停元数据补齐既有 DTO，并在停止 / 到期时清除。

Go 夹具覆盖八类状态、暂停 / 锁屏交错、过期状态事件、设置持久化及策略失败重试；原生独立
smoke 覆盖工作线程同步策略与异步字符串拷贝、状态栏去重、20 项主菜单本地化与非阻塞提示
显示 / 关闭 / 生命周期清理。状态栏 ABI 3 新增 icon 字段，macOS 静态库已匹配；Windows
桥同步字段并需匹配 DLL 重建，本次自动化记录未在 Windows 主机运行原生通知区验证；已实现功能另经 09-26 用户确认验收。
本轮真实 Wails Dock 开关、语言切换、捕获连续性与已实现关机 / 注销路径于 09-26 用户确认已验收（无逐项记录），
步骤见 [08 增量回归](../08-testing-strategy.md#861-l2-用例)；匿名原生宿主不替代 G-host。

本增量验证：`./scripts/gate.sh` 通过（Go build / internal 单测 / vet、Linux / Darwin / Windows
无 cgo 核心构建、169 项前端测试 / typecheck / build、54 篇文档链接、Windows 安装器夹具
4 项通过 / 5 项限 Windows 执行跳过）。
`bash native/darwin/system-smoke.sh` 通过（含启动策略被宿主重置后的重应用）；app / recorder /
Darwin System 的定向 `-race` 通过。门禁已重新生成绑定并构建匹配的 universal 原生 archive；
先前全 app 的 race 扩扫在旧 `recordingEmitter` 测试夹具中报告竞争，未将全包 race 记为通过。
强制重新链接的 `go test -a ./internal/platform/darwin -run 'TestSystem' -count=1` 通过。

2026-09-26：macOS 宿主控制加固：新增状态栏已应用可用性查询，入口不可用时软退出保留
Dock；激活策略同步返回 AppKit 成功 / 拒绝并可重试；普通真退出遇分段收尾失败保留进程，
系统关机 / 注销独立放行且禁用授权自重启；System 关闭后拒绝晚到回调，终态关机事件优先入队。
`CGO_ENABLED=0 go test ./internal/app ./internal/platform/darwin` 与
`bash native/darwin/system-smoke.sh` 通过，后者在独立匿名宿主验证 regular / accessory、
状态栏安装 / 隐藏 / 移除、合成关机通知与观察者移除；不读取用户库或屏幕。
`native/darwin/build.sh` universal 构建、`go test -a ./internal/platform/darwin -run 'TestSystem' -count=1`
与 `./scripts/gate.sh`（含三平台无 cgo 构建、168 项前端测试）通过。本切片生命周期与 System
关闭夹具的定向 race 扫描通过；扩大到整个 app 包的 race 扫描在既有 `recordingEmitter`
无锁测试收集器中失败（chat / timeline 测试），不记为全包 race 通过。
真实注销 / 关机、策略与关窗持续捕获的已实现功能于 09-26 用户确认已验收；未附逐项步骤，
不倒填为上述匿名 smoke 的真实宿主证据。

2026-09-26：macOS 解码 ABI 增加每次调用的 `autoreleasepool`，保留 reader 取消和独立
C 输出缓冲的所有权。新增匿名 1080p 多段 / 并发 / 缩略图 / 错误恢复夹具，验证返回缓冲不会
被后续调用改写。`native/darwin/build.sh`、强制重新链接的原生测试和 `./scripts/gate.sh` 通过。
`DAYGO_NATIVE_MEMORY=1 go test -a ./internal/platform/darwin -run '^TestNativeDecodeMemoryPlateau$' -count=1 -v`
在三个独立进程中，300→600 次解码的静置 footprint 分别为 71.1→70.8、59.5→59.5、
59.7→57.4 MiB，均未超过 5 MiB 增量上限；修改前同一夹具为 77.2→94.8 MiB，失败。
测试内 Go GC 仅用于隔离原生保留，不是生产释放策略。长期真实应用观察于 09-26 用户确认已验收；未附 24 小时 / 14 天逐项记录或新增测量值。

2026-09-23：macOS HEVC 单帧读取补充资源收尾。`SegmentReader.decodeFrame` 在
`AVAssetReader.startReading()` 成功后，无论解码成功、帧缺失还是 JPEG 编码失败，均调用
`cancelReading()` 结束该次 CoreMedia 读取。此前每次读取只取一个样本，不会自然读到文件末尾，
存在解码工作线程滞留的风险。原生 universal 静态库构建和 macOS 分段读写 smoke 已通过，
另以同一进程连续 200 次单帧解码验证结果可读；
尚无同一进程连续 24–48 小时的线程数与 RSS 对照记录，因此不能将用户观察到的全部长期增长
归因于这一处；macOS 长期观察已于 2026-09-22 经用户实测验收（无逐项运行记录）。

> **验收状态（2026-09-26）**：本模块所有已实现能力（含近期增量、长期观察与已实现的真实安装升级）经用户确认已验收，未附逐项运行记录。未实现能力、待定设计与正式证书缺失保持原状态；历史命令的失败、跳过或未运行不改写为通过。统一记录见 [09 §9.1.1](../09-roadmap.md#911-本轮验收记录与证据边界)。

实现进度：部分实现。单元 / fake 契约已覆盖单次截图语义；macOS 原生单次截图与 cgo 适配已
落盘并完成一轮真实像素 smoke；Go recorder、pending capture 提交 / 恢复已落盘并通过 fake
生命周期测试。录制设置、主页开始控制、`recording:state` 前端同步和 macOS 状态栏的有限接入
已经落盘，用户已确认 dev 基本功能正常。**2026-09-12：启动自动录制已落盘**——
OnStartup 在状态栏安装后调用 `maybeAutoStartRecording`，三重防呆（capture owner /
屏幕授权 granted / 路由链主 provider 存在）全过才 `SetRecording(true)`，否则静默跳过；
已知偏差：无「停止后不自启」记忆（每次启动都录，设置项后续切片）、G-host 当时未跑
（退出即停，空窗由分析流水线 24h 未分批回看补齐），后已于 2026-09-22 经用户实测验收（无逐项运行记录）。production、隐私实机矩阵和长期观察
经用户确认已验收。
**2026-09-13：录制鲁棒性与崩溃恢复修复已落盘**——
① 崩溃恢复接线：启动时 `Captures().Reconcile` 在分析流水线之前运行，把已落盘但未提交的
pending intent 提交进 `screenshots`、把文件缺失的 intent 丢弃（此前该方法无调用点，崩溃
帧静默丢失）；`Reconcile` 的 recordings 根目录改为调用方显式传入，不再从 store 路径反推。
② 暂停竞态泄漏修复：capture 过程中被暂停的帧现在 `Abandon` 其 pending 行（此前文件删除
但行永久泄漏）。
③ 单帧失败容错：截图失败（含占位帧写失败）不再终止录制循环——适配器失败时 Abandon
intent，连续失败计数达到 3 次才放弃，成功即清零；初始 capture 同样容错。
④ 空闲采样仍未接入：`idle_seconds_at_capture` 恒为 NULL，空闲判定因此永不命中——
platform 端口缺 idle 查询能力，属待定设计，需要在 `System` 或 `Capture` 端口决策后
（docs/09 §9.8）补一个 `docs/decisions/` 记录再实现。

**2026-09-17：直接追加 HEVC 帧段优化落盘（Dayflow 方式）**——
落实 [HEVC 分段落盘决策](../decisions/recording-frame-segments-hevc.md) 的切片 A–D：
① 捕获端免 JPEG staging：Swift `SegmentWriter` 采用 VideoToolbox / `AVAssetWriter`
硬件编码器直接追加 HEVC 帧（质量 0.55、关键帧间隔 30、600 帧/600 秒滚动、分辨率变更滚动、
前台应用屏蔽时写入脱敏占位帧）；`SegmentCloser` 接口用于暂停与进程退出时的活跃段安全收尾。
② 读路径接入：Swift `SegmentReader` 采用 `AVAssetReader` 按段与帧序号随机访问解码（带 LRU
段缓存与 `maxPixelSize` 缩略图下采样），并保留旧 JPEG staging 文件的 legacy 直读回退；
Go 侧通过 darwin `platform.Media` 驱动，`/media/frame` 资源处理器与 analysis 流水线源已统一接入。
③ 存储与迁移 v15：`pending_captures` 增加 `frame_index`（迁移 v15），支持同一段文件内多帧
意图跟踪；`gen.go` 增加 v14 夹具，DB-2 迁移测试与崩溃对账测试通过；`Reconcile` 增加 `hasMoovAtom`
校验，未收尾的残破 MP4 自动丢弃不入库。
④ 整段清理：`cleanup.go` 改写为按 `segment_path` 整段软删除并物理删除段文件，豁免未收尾段与
活跃分析批次租用段。
⑤ 门禁：macOS 真实像素往返与段滚动 smoke、Go 单元 / 夹具测试、`CGO_ENABLED=0` 构建与
`./scripts/gate.sh` 全绿通过。

Windows 侧另有一份同 ABI 的 DXGI/WGC 实现（`internal/platform/windows` + `native/windows`），
已在一台 Windows 11 双屏机器完成原生与 Go cgo 的真实非黑 JPEG smoke；发布范围经决策记录推进（§9.8 #18），
完整 WC 隐私 / 显示器 / 资源矩阵由用户确认验收，未附逐项运行记录。Windows Store 已由 `LockFileEx` 接通，不再因锁实现缺失而
无法打开数据库。Windows System 也已接入睡眠/唤醒/锁屏/解锁、显示器枚举和通知区动作；这些
只有编译、回调夹具与有限启动证据，尚不能替代关窗持续捕获、真实系统事件和 24 小时资源矩阵。
以上不改变本模块的验收口径。

2026-09-20：Windows 平台差集实现已接线：Media Foundation HEVC/MP4 分段写入、Source Reader
按帧读取和 legacy JPEG 回退；`System` 增加屏保状态转换与 `WM_DISPLAYCHANGE`；隐私应用列表从
当前用户/机器、32/64 位 App Paths 与 Uninstall 注册表枚举，并统一经过现有 EXE 身份解析；
`SetActivationPolicy` 按 Windows 无进程级 Dock 策略的事实实现为幂等等价语义，窗口显示仍由
Wails/app 层管理。Go 平台测试、Windows `CGO_ENABLED=0` 交叉构建和全量 `gate.sh` 通过；原生
Windows 编译、HEVC 编解码 smoke、真实屏保/显示器通知与应用枚举结果经用户确认已验收；Windows 发布仍需独立授权。

**2026-09-20：macOS Dock 点击重开窗口修复已落盘**——原生 `System` 观察应用重新激活并通过
平台事件上送，app 层与状态栏“打开 Daygo”共用恢复激活策略及显示窗口的动作；Go 路由测试已覆盖。
真实 Dock 点击、accessory/regular 切换观感及关窗后长期捕获已于 2026-09-22 经用户实测验收（无逐项运行记录）。

**2026-09-21：原生界面文案接入 i18n**——此前 `PickApplication` 的 Windows 面板标题 /
`.exe` 过滤器名是硬编码英文，与状态栏文案走的两条通道不同。新增绑定
`SetNativeUiLabels(NativeUiLabelsDTO)`（[05 §5.5.1](../05-interface-contract.md#551-绑定方法目录)）：
前端在加载与语言切换时下发面板标题、过滤器名与更新弹窗拒绝安装的说明，后端按表面路由，
适配器仍不持有 locale。`PickApplication` 改为在调起面板时读取当前 bundle（
`applicationPickerOptions(goos, labels)` 是纯函数，macOS / Windows 两个分支都有单测）；三个
字段在后端各有 zh-CN 默认值，避免下发前渲染出无标题的原生面板。已验收部分：Go 单测
（存储、转发、默认值、两个平台分支）、契约清单、前端 typecheck 与 65 项前端单测、
`CGO_ENABLED=0` 构建。**经用户确认已验收**：Windows 上原生面板标题与过滤器名的实际渲染、
macOS 面板仍按决策不下发标题。系统授权框、钥匙串与 WinSparkle 的文案不由本应用提供，
不在本通道内（见 [delivery](delivery.md)）。
**2026-09-23：连续失败后保持重试**——recorder 遇连续捕获/提交失败仍保持 `capturing` 并按间隔重试，
保留最后一次失败供 `GetRecordingState.reason` 查询；成功捕获、重新启动或用户主动
停止时清除。原因只包含 Capture 稳定错误码及原生数值码、storage 错误类别，或通用文件/未知类别，
不包含错误文本、路径或屏幕内容。Windows Recorder 测试页显示该代码。该改动证明失败原因可见，
并不证明当前 Windows 真机停止录制的实际根因；仍需对应故障时的代码和 WC-8 长时间观察。
`internal/recorder` 提供可停止的 Go 状态机：`idle → starting → capturing`，支持 `paused`
与恢复；Capture 前写入 pending intent，完成后幂等提交 `screenshots`。`Backend` 已接入
`SetRecording`、`PauseRecording`、`ResumeRecording`，绑定首次调用时读取真实 settings 并装配
当前平台 Capture；后续设置更新会下发给运行中的 recorder。系统事件桥与 recorder 处理已有代码，
但真实权限请求和睡眠 / 锁屏 / 屏保矩阵经用户确认已验收。

截图测试页现在按平台切换：macOS 面板保留直接 ABI 单次 / 定时联调；Windows 面板通过正式
`SetRecording` / `GetRecordingState` 与 `recording:state` 驱动并观测共享 Go recorder，截图成功且
`screenshots` 提交完成后才更新 `lastFrameAtTs` 和本轮帧数。Windows 面板不会为测试清空或
绕过用户隐私设置；build 26100+ 的非空名单由 WGC `SetWindowExclusionList` 做画面排除，
更旧系统返回 `privacy_unsupported`。

## 能力与跨层职责

| 输入 | 可先推进 | 真实接入条件 |
|---|---|---|
| data: db-core / settings-store | 用 pending capture ID 和文件夹具验证提交、恢复与去重；状态机使用 repository fake | DB 门禁、连接层只读、捕获所有权验收 |
| preferences: settings-access / ui-bridge | 固定录制配置快照、绑定 DTO 和状态事件夹具 | 配置落库、事件重拉；UI 扩张需 G-host |
| delivery: 身份与分发探针 | 有限宿主、截图、编解码实验 | G-native 与本能力原生决策 |
| data: 清理能力 | 接入完整 JPEG 与后续 Media 分段元数据 | 真实长期录制前验证 staging 与整段清理集成 |

输出 host、capture、media-read 三项能力，按 09 §9.3 分别验收。
Go 负责录制意愿、暂停、恢复延迟、监管和配置；internal/app 编排生命周期、绑定和事件；
平台层负责 System/Capture/Media 与各自 fake，不写 SQL；screenshots repository 在
internal/storage。recording 协调 System 公共端口，daily 自行交付通知实现。
隐私名单的设置、UI 和捕获行为均归本模块。

## 实验与失败条件

| 实验 | 输入与操作 | 预期结果 | 失败条件 / 证据 |
|---|---|---|---|
| 宿主与 IT-14 | 限时一周探针；真实截图后关窗，观察至少 10 分钟，再从状态栏重开并切激活策略 | 进程与离散捕获持续，窗口可重开 | 心跳正常但截图停止仍失败；记录匿名帧计数 / 时间、进程状态 |
| MC-1–8 / IT-5/8/9 | 未授权、撤权、双屏、旋转 / HDR、屏蔽应用前台与后台窗口 | 授权显式请求；两层保护、目标屏及尺寸正确，无持续录屏指示 | 泄漏、过期帧或指示不符阻塞真实捕获；实机矩阵保存脱敏结果 |
| IT-1–4/10/11/13 | 匿名帧、提交前后崩溃、pending 恢复、双实例 | 单写入 / 单捕获、幂等提交、无重复行 | 丢帧、重复、错误只读或孤儿文件无法恢复会阻塞接入 |
| IT-6/7 与 MC-9/10 | user idle / user pause / capturing 下触发睡眠、锁屏、退出 | 各事件分别建模；唤醒 5 秒、解锁 0.5 秒；系统不恢复 idle | 状态混淆、过期截图入库或 pending 文件失控失败 |
| MC-12 与 08 长期断言 | 真实 24 小时分间隔实验，后续累计稳定性窗口 | 资源有界、无未解释缺口 | 编译通过不能代替长时间证据 |

## 实现切片与集成

1. 冻结实验输入与失败条件，完成宿主和捕获 / 分段候选实验；按 06 §6.6 落盘相关子决策。
   未决前只做有限探针，G-host 需要真实截图证明。
2. 补 fake System/Media 与 Capture 执行中取消、故障注入契约；实现 Go 录制状态机及可停止监管。
   产物是可无 GUI 运行的状态序列测试。
3. 使用 db-core 接 pending capture 与 screenshots repository；真实 Capture 跑同一套单次调用契约，
   验证“文件发布 → 幂等入库”及崩溃对账，再由 Media 能力决定是否转入分段。
4. 实现录制 / 授权绑定与事件、状态栏控制及设置分区，store 负责重拉；拒绝授权、无所有权、
   适配器故障均有可见状态。Go recorder 状态是真实来源，不用“希望录制”冒充实际 capturing。
5. 逐条运行 IT / MC 实机矩阵；与 timeline 接入后累计 G-loop 与 G-stability 证据。

## 验收、阻塞与回退

完成要求：用户闭环与 IT-1–14 中本模块路径、相关 MC 门禁通过，真实 JPEG 与后续媒体产物可读、
可恢复，隐私双保护和状态栏可操作。fake 仅证明契约，不证明像素、身份、耗电。
G-host/G-native 失败限制原生接入与大规模 UI；核心状态机、fixture 和其他模块仍可推进。G-host 已于 2026-09-22 经用户实测验收（无逐项运行记录），大规模 UI 扩张解锁；现有身份下真实安装升级于 09-26 用户确认已验收；G-native 的正式签名 / 公证材料仍缺，正式发布身份未验收。

待决：宿主 / 适配形态、截图真实门禁、系统事件、分段格式、解码、状态栏正式形态与长驻验收、
协议；负责人为 recording 工程，身份协同 delivery；均须在相应大规模实现前决定，见 09 §9.8。
回退：停止新增 Capture 调用，清理未提交 staging 文件并保留数据库；撤销未启用的接入改动可
恢复外壳。禁止以清空数据目录代替恢复。

## 验证记录

- **状态机回归（2026-09-23）**：连续 4 次捕获失败后自动恢复、睡眠期间定时暂停、停止时恢复定时器竞态的 Go 夹具通过；`./scripts/gate.sh` 通过。尚未在真实 Windows 捕获故障上复现与复核。
- **macOS 分段追加（2026-09-23）**：取消后的截图任务在写段前再次检查取消；每帧像素缓冲在
  `autoreleasepool` 内释放；段收尾等待限制为 10 秒并记录超时诊断。`native/darwin/build.sh`
  构建 arm64 / x86_64 通用静态库，`go test -a ./internal/platform/darwin -run TestNativeSegment -count=1`
  的追加、解码、滚动和重复读取 smoke 通过。尚未注入 HEVC 收尾卡死，也未完成 24–48 小时 RSS 对照。
- **macOS（2026-09-10—17）**：Capture fake 契约、Swift 通用静态库构建、真实单次像素 smoke、HEVC 段追加 / 解码 / 滚动 smoke、迁移夹具及 `./scripts/gate.sh` 通过。Go recorder 的暂停、失败容错和 pending 恢复有单元测试。
- **Windows（2026-09-11—20）**：Windows 11 双屏机器上完成原生与 Go cgo 的非黑 JPEG smoke；`LockFileEx`、设置页应用选择和通知区完成有限验证。Media Foundation 分段与系统事件已有代码和 Go 测试；完整矩阵由用户于 2026-09-22 确认验收，未附逐项运行记录。
- **用户闭环**：隐私、授权、G-host 和长期观察由用户于 2026-09-22 确认验收，未附逐项运行记录。未实现的空闲采样与 Linux Capture / System 不在本次验收范围；边界见 [09 §9.1](../09-roadmap.md#91-模块总表)。
