# recording — 常驻录制

## 用户结果与范围

用户授权后可开启、关闭或定时暂停记录；关窗后继续离散截图，状态栏可查看状态并重开窗口。
屏蔽应用既从截图排除，前台命中时又生成脱敏占位帧；睡眠、锁屏、屏保、退出的分段安全收尾。
包含间隔 / 分辨率、屏蔽名单与已安装应用选择器、自启和 Dock 设置。
负责 U6/8/10、F-C1–9、F-S5/8、F-L1–3；不包含 AI 分析、时间线页面或 Windows 发布承诺。

公共依据：[04 §4.1](../04-data-flow.md#41-捕获流水线)、
[05 §5.7](../05-interface-contract.md#57-b4platform-端口契约)、
[06](../06-native-integration.md)、[截图 v2 实现与调用](../decisions/recording-screen-capture-v2.md)、
[Windows 截图实现与限制](../decisions/recording-screen-capture-windows.md)、
[图片存储流水线](../decisions/recording-image-storage.md)、
[07 §7.2](../07-privacy-security.md#72-捕获侧的两层保护)。
实机矩阵：[08 §8.6.2 MC](../08-testing-strategy.md#862-mc真实-macos-捕获矩阵)、
[§8.6.3 WC](../08-testing-strategy.md#863-wc真实-windows-捕获矩阵)。

## 当前状态与证据

实现进度：部分实现。单元 / fake 契约已覆盖单次截图语义；macOS 原生单次截图与 cgo 适配已
落盘并完成一轮真实像素 smoke，但应用装配、隐私实机矩阵和长期观察未验收。
Windows 侧另有一份同 ABI 的 DXGI 实现（`internal/platform/windows` + `native/windows`），
已在一台 Windows 11 双屏机器完成原生与 Go cgo 的真实非黑 JPEG smoke，但仍**不在发布范围**；
完整 WC 隐私/显示器/资源矩阵未完成。Windows Store 已由 `LockFileEx` 接通，不再因锁实现缺失而
无法打开数据库。以上不改变本模块的验收口径。
[Capture fake](../../internal/platform/fake/capture.go)、
[契约套件](../../internal/platform/platformtest/suite.go)、
`internal/app/capture_test_binding.go`、`capture_test_application_binding.go` 和
`frontend/src/views/CaptureTest/CaptureTestView.vue` 提供临时联调页面：可通过 Wails 原生面板
选择 `.app`，把 ScreenCaptureKit 使用的 Bundle ID 加入屏蔽名单，再配置截图参数单次或限时调用并
打开输出目录。路径不跨绑定且不持久化；该页面不接 recorder、正式 settings 或数据库。
Media、完整 System、recorder、storage pending 恢复和后台生命周期尚未实现。

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
G-host/G-native 失败限制原生接入与大规模 UI；核心状态机、fixture 和其他模块仍可推进。

待决：宿主 / 适配形态、截图真实门禁、系统事件、分段格式、解码、状态栏与协议；负责人为
recording 工程，身份协同 delivery；均须在相应大规模实现前决定，见 09 §9.8。
回退：停止新增 Capture 调用，清理未提交 staging 文件并保留数据库；撤销未启用的接入改动可
恢复外壳。禁止以清空数据目录代替恢复。

## 验证记录

2026-09-10：单次 Capture fake 契约测试通过；Swift arm64/x86_64 通用静态库构建通过；
darwin cgo、无 cgo 与 Linux 交叉编译门禁通过；合成图 JPEG 原子落盘验证为 32×18、777 bytes。
授权后的真实 cgo 调用从 1920×1080 主显示器生成并解码 1280×720 JPEG，返回宽高、字节数与
磁盘一致。隐私双保护、捕获指示、正式应用 TCC 身份、G-host 与长期观察仍未验收。

2026-09-11（当前工作树，Windows 11 NT 10.0.26200、NVIDIA RTX 4060 Laptop GPU、双显示器）：
原生 smoke 与 Go cgo smoke 均生成并解码 1280×720 非黑 JPEG。调试记录确认首个 pointer-only
全零帧被跳过，后续桌面更新由 DXGI 返回非零 BGRA，没有命中 GDI fallback。非空屏蔽名单返回
`privacy_unsupported` 且不生成文件，证明失败关闭而非隐私能力完整。仅 WC-1 有限通过；
目标冲突、多屏切换/旋转、受保护内容、光标与 24 小时资源矩阵未运行。

2026-09-11：临时 `CaptureTest` binding 使用真实 macOS `darwin.Capture` 完成 one-shot smoke，生成并
检查 JPEG 文件存在、非空且返回文件大小一致；fake binding 行为测试、Go 全量测试、前端 typecheck/build
和文档链接检查通过。Wails 原生窗口中的页面视觉检查受当前 headless 环境限制，已用 Vite 页面和无障碍
树确认路由、导航入口、配置控件与操作按钮渲染；定时与 Finder 长期观察仍未验收。

2026-09-11：新增独立 macOS 应用 Bundle ID ABI 与 `platform.ApplicationInspector`。对
`/System/Applications/Calculator.app` 的 Go → cgo → Swift smoke 返回 `Calculator` /
`com.apple.calculator`；arm64 + x86_64 universal archive、binding 行为测试和前端 typecheck 通过。
Wails 原生 `.app` 面板等待人工视觉验收；helper / XPC 和 MC 隐私矩阵未验收。

2026-09-11：应用身份改为与 `SCRunningApplication.bundleIdentifier` 一致的 Bundle ID，不再把
代码签名资源完整性作为屏蔽名单接入条件。资源被 Custom UI Style 修改的 VS Code smoke 返回
`Code` / `com.microsoft.VSCode`；无签名测试 bundle 的回归测试通过。前台兜底同步改用
`NSRunningApplication.bundleIdentifier`，避免与 ScreenCaptureKit 使用不同身份来源。
