# 06 原生集成

> **实现方式整体待定设计。** 本文只回答两个问题：Daygo 需要平台提供**哪些能力**，
> 以及 Go 侧的**端口长什么样**。用什么技术实现这些能力、适配层以什么形态存在，
> 按负责模块的能力决策记录处理（[09 §9.8](09-roadmap.md#98-待定设计清单)）。
>
> 在决策落盘前，**不得**按某一种候选方案大规模实现，也不得删除其它候选路径。
>
> 22 项能力中已有一部分落地真实实现（单次截图分 macOS 与 Windows 两套，帧解码 / 探测与
> Secrets 亦已实现），其余仍为待定设计；逐项状态见 [§6.7](#67-平台实现状态)。

## 6.1 为什么单独隔离这一层

Go 能做完这个产品的绝大部分：分批、调度、解析、存储、派生视图、provider 调用。真正
必须依赖操作系统的能力是一份**有限且可枚举**的清单。把它们收拢到一个端口后面，收益有三：

1. `CGO_ENABLED=0 go test ./internal/...` 能在 Linux CI 上无头跑完整条流水线；
2. 业务代码永远不需要知道适配层是进程内还是进程外；
3. 适配方案换了，改的是一个包，不是整个代码库。

代价是每次跨界都有一次调用开销。对捕获（每 10 秒一次）无所谓；对帧解码（一屏几百张
缩略图）必须靠批量接口摊销，见 §6.4。

## 6.2 需要的平台能力清单

这是**完整**清单。不在表内的能力，说明 Go 侧应该自己做。

| # | 能力 | 用于 | 端口方法 | 实现 |
|---|------|------|----------|------|
| 1 | 查询屏幕录制授权状态 | 捕获前预检 | `System.ScreenRecordingPermission` | 待定设计 |
| 2 | 触发授权申请 | 引导流程 | `System.RequestScreenRecordingPermission` | 待定设计 |
| 3 | 跳转到系统设置的指定面板 | 授权被拒后的引导 | `System.OpenSystemSettings` | 待定设计 |
| 4 | 枚举显示器 | 选择捕获目标 | `System.Displays` | 待定设计 |
| 5 | 截取当前主显示器的一帧 | 捕获 | `Capture.Capture` | 有限实现，待实机矩阵 |
| 6 | 从捕获中排除指定应用 | 隐私屏蔽 | `CaptureRequest.BlockedApplicationIDs` | 有限实现，待双保护验收 |
| 7 | 把单帧原子编码为 JPEG | staging | `Capture.Capture` | 有限实现，待恢复接入 |
| 8 | 从分段解出单帧为 JPEG | 缩略图、帧条 | `Media.DecodeFrame(s)` | 待定设计 |
| 9 | 把多帧合成为 mp4 | timelapse | `Media.EncodeVideo` | 待定设计 |
| 10 | 探测分段的帧数与尺寸、可读性 | 崩溃恢复 | `Media.ProbeSegment` | 待定设计 |
| 11 | 读取系统空闲秒数 | 空闲判定 | `System`（内部） | 待定设计 |
| 12 | 解析调用时的系统主显示器 | 捕获目标 | `Capture.Capture`（内部） | 有限实现，待多屏验收 |
| 13 | 最前方可见应用标识 | 隐私屏蔽判定 | `Capture` 内部 / `System.FrontmostApplication` | 有限实现，待实机矩阵 |
| 14 | 已安装应用列表 | 隐私名单选择器 | `System.InstalledApplications` | 两平台均已实现；Windows 从当前用户/机器、32/64 位 App Paths 与 Uninstall 注册表枚举，去重规则见 [应用身份解析](decisions/recording-application-picker.md) |
| 15 | 睡眠 / 唤醒 / 锁屏 / 解锁 / 屏保事件 | 捕获状态机 | `System.Events` | macOS System ABI 已实现，待实机验证 |
| 16 | 显示器配置变化事件 | 刷新捕获目标 | `System.Events` | macOS System ABI 已实现，待实机验证 |
| 17 | 开机自启开关 | 设置 | `System.{,Set}LaunchAtLogin` | 待定设计 |
| 18 | 激活策略切换（是否占 Dock） | 后台 Agent 语义 | `System.SetActivationPolicy` | 待定设计 |
| 19 | 状态栏项与其菜单 | 无窗口时的入口 | `System.SetStatusItem` | macOS 与 Windows ABI 均已实现并接入；两平台的完整宿主/长驻矩阵分别验收 |
| 20 | 本地通知 | 日记提醒 | `System.ScheduleNotification` | 待定设计 |
| 21 | 系统钥匙串读写删 | provider 密钥 | `Secrets` | macOS / Windows 已实现；Linux Secret Service 已落盘，待真机验收 |
| 22 | 自动更新 | 版本分发 | `Updater` | 待定设计 |

端口的完整 Go 签名见 [05 §5.7](05-interface-contract.md#57-b4platform-端口契约)。

### 6.2.1 关于捕获方式的一条产品约束

**捕获必须是离散截图，不是连续屏幕录制流。** 连续录制会让系统的屏幕录制指示器常亮，
从根本上改变这个应用给用户的观感。这是**产品决策，编码在能力选择里**——任何把第 5 项
换成"持续录制流"的适配实现都改变了产品，不只是改变了实现。

## 6.3 适配层的职责边界

适配层**承担**：

- 上表 22 项能力的具体实现；
- 单次 Capture 的临时 JPEG 编码与排他原子发布；调用返回后不保留帧缓冲、事件日志或 recorder
  状态；
- 与 Go 之间的协议编解码（若形态是进程外）。

适配层**明确不承担**：

- **不打开、不写 SQLite。** Go 是唯一写入方；Go 负责 pending 记录、幂等提交和启动对账。
- **不读设置为自己决策。** 每次 Capture 请求全量携带本次参数，因此适配层调用后无状态、
  在测试中可复现。
- **不发起网络请求。**
- **不含产品逻辑**：不分批、不做空闲判定、不生成卡片、不判断哪天属于哪个逻辑日。

这条边界的价值在于：它让"重启适配层"成为一个无需数据库协调的动作。

## 6.4 已知的工程约束

即使实现方式未定，下面几条对**任何**方案都成立，选型时必须一并评估：

| # | 约束 | 后果 |
|---|------|------|
| 1 | 屏幕录制授权与钥匙串访问由系统绑定到**代码身份** | 执行捕获的二进制若身份变化，用户会重新收到授权提示，且已存的密钥读不出来。见 [风险 C-3](10-risks.md#c-3身份与授权不稳定) |
| 2 | 未收尾的分段文件可能完全不可读 | 睡眠、锁屏、更新重启、宿主退出、关机路径都必须收尾或安全移交当前分段 |
| 3 | 唤醒后立即查询显示器列表可能得到过期结果 | 唤醒恢复延迟 5 秒，解锁 0.5 秒（[04 §4.1.5](04-data-flow.md#415-睡眠--唤醒--锁屏)） |
| 4 | 单帧解码的跨界开销会被缩略图条放大数百倍 | 必须提供 `DecodeFrames` 批量接口，并在 `internal/media` 做有界 LRU |
| 5 | 编码器可能静默卡死：接受帧但不产出数据 | 需要写入方状态检查，并把失败上报给 Go，而不是只打日志 |
| 6 | 若适配层是独立进程，签名与公证涉及嵌套代码签名 | 配置错误只在最终用户机器上复现，必须在原生形态决定前由 delivery 以有限探针验证完整签名 / 公证可行性（[风险 M-3](10-risks.md#m-3打包签名与安装内容漂移)） |

## 6.5 fake 适配层

`internal/platform/fake` 与真实适配层是**同等地位的实现**，不是测试脚手架：

- 它让分析流水线、AI 层、insight 构建器在没有 Mac 的环境里可测；
- 各功能随使用的能力交付 fake 与真实契约，无需先完成所有端口；
- **它和真实适配层必须通过同一套契约测试**（`platformtest.Suite`）。只有 fake 通过、
  真实适配层没跑同一套测试的接口，不算已验证。

Capture fake 需要能构造：正常 JPEG、授权拒绝、blocked、适配层不可用、取消 / 超时和目标路径
冲突。System / Media fake 再分别覆盖事件合并、分段收尾、解码失败和关闭语义。

## 6.6 选型时要回答的问题

九项问题全部保留，由下列负责模块分别记录证据和阻塞条件。允许按能力做子决策；
没有相关证据就不能宣称该能力已决定 / 已验收。宿主形态确定前须先回答其生命周期、
身份 / 分发与更新可行性问题，不能把风险全部推迟到安装更新功能完成时。

| # | 问题 | 负责模块 | 验证时机 |
|---|------|----------|----------|
| 1 | 关闭最后一个窗口后进程能否存活并继续捕获？ | recording | G-host；真实离散截图持续至少 10 分钟 |
| 2 | 状态栏项能否在无窗口状态下工作，并重新打开窗口？ | recording | G-host |
| 3 | 激活策略能否在运行时切换？ | recording | G-host |
| 4 | 屏幕录制授权与钥匙串访问在该形态下是否稳定，升级后是否会重新提示？ | recording / providers，delivery 协调身份 | 原生形态确定前做身份探针，真实捕获 / 密钥接入前验收 |
| 5 | 若为进程外形态：签名、公证、Gatekeeper 是否在干净机器上通过？ | delivery | 原生形态确定前做分发探针；进程内同样须验证发行身份 |
| 6 | 适配层崩溃后能否自动恢复，且不丢帧？ | recording，data 提供写库去重 | capture / media-read 真实接入前 |
| 7 | 帧解码的跨界开销在 200 张缩略图的一屏下是否可接受？ | recording，timeline 提供资源 / 缓存接入 | media-read 及真实帧条验收前 |
| 8 | 自动更新链路在该形态下如何工作？ | delivery，recording 提供收尾协议 | 原生形态确定前验证可行性；Updater 完成时验证真实升级 |
| 9 | 崩溃报告能否关联两侧（若为两个进程）？ | delivery，data 提供匿名诊断 | 边界诊断设计前明确关联方式，崩溃上报接入前验收 |

回答写入 `docs/decisions/<module>-<topic>.md` 并在执行册链接；形态总记录由 recording
在决策时建立 `recording-native-adapter.md`，汇总以上子项的证据与限制，
同步本文和 [05 §5.8](05-interface-contract.md#58-b5平台适配边界待定设计)。
当前总记录尚未落盘。设备、签名身份或发布授权缺失时记录对应实验阻塞，不伪造通过。

候选技术、Go 数据契约、C ABI 与实验矩阵见
[recording 屏幕捕获](decisions/recording-screen-capture.md)。编译探针不替代上述真实机器证据。

## 6.7 平台实现状态

macOS 是当前主线，Windows 与 Linux 是已排期的一等发布目标。Windows 现在有一份**完成有限真机
smoke** 的截图实现，发布范围与其余能力逐项经决策记录推进（[09 §9.8 第 18 项](09-roadmap.md#98-待定设计清单)）。
把状态写在这里，是因为“仓库里有 Windows 代码”和“Windows 已验收可发布”是两件事，不写下来就会被混淆。

| 能力 | macOS | Windows | 说明 |
|---|---|---|---|
| 单次截图（第 5 / 7 / 12 项） | 有限实现，已跑通真机 smoke | 有限实现，真机非黑 JPEG smoke 通过 | macOS 用 ScreenCaptureKit，Windows 优先 DXGI Desktop Duplication；GDI 只在有效桌面更新仍为全零时回退 |
| 隐私屏蔽（第 6 / 13 项） | 前台兜底 + 画面排除，两层齐备 | build 26100+：前台兜底 + WGC `SetWindowExclusionList`；更旧系统失败关闭 | Windows 11 24H2（26100）是明确最低门禁；名单非空时改走 WGC，并等待对应 configuration iteration 后才接收帧 |
| 光标（`ShowsCursor`） | 生效 | **忽略**（Desktop Duplication 不含指针） | 实现与 ABI 语义之间的已知缺口 |
| 屏幕录制授权（第 1–3 项） | TCC 查询 / 请求 / 设置入口已实现 | 系统无对应 TCC，查询报告 `granted`、请求为 no-op | macOS 的正式签名升级身份仍属 G-native；Windows 不伪造授权弹框 |
| 实例锁（写入锁 / 捕获所有者锁） | `flock` 已实现 | `LockFileEx` 已实现并通过跨进程 smoke | 两平台共享 `storage.Open`、只读降级与 `ErrLockBusy` 语义；见 [data 实例锁](decisions/data-locking.md) |
| 应用身份解析（第 14 项前置） | 有限实现：Wails `.app` picker + 独立 ABI 2.x（身份 + 名称 + 图标 + 按 Bundle ID 回查） | 有限实现：Explorer `.exe` picker + 同一 ABI；路径哈希 ID、名称、PNG 图标和回查 | Windows 路径不进入 Wails DTO；回查优先内存、运行进程与 App Paths / Uninstall 注册表 |
| 应用枚举（第 14 项） | `InstalledApplications` 已实现（含 Go cgo smoke） | `InstalledApplications` 已实现，并在一台 Windows 11 机器核对过枚举结果 | 枚举结果只有本机证据；Windows 设置页网格的视觉与交互未验收，枚举不可用时仍回落到 Explorer `.exe` picker |
| 系统事件（第 15 / 16 项） | System ABI 已实现（睡眠 / 唤醒 / 锁屏 / 解锁 / 屏保 / 显示器变化） | 睡眠 / 唤醒 / 锁屏 / 解锁 ABI 已实现；显示器可枚举 | Windows 编译与回调夹具已过，睡眠/锁屏恢复延迟及长期事件矩阵仍需实机 |
| 状态栏（第 19 项） | `SetStatusItem` ABI 已实现并接入 | 通知区图标、菜单、左键重开及 open/toggle/pause/quit 动作已接入 | Windows 回调夹具通过；关窗后持续捕获、Explorer 重启恢复及完整交互 smoke 尚未验收 |
| 帧解码 / 段探测（第 8、10 项） | 原生段读取（`frameDecode` / `segmentProbe`） | 纯 Go `mediafile` | 两平台都经 `platform.Media` 真实实现；Windows / Linux 走 [`internal/platform/mediafile`](../internal/platform/mediafile/mediafile.go)：JPEG 单帧解码 + 探测，非 JPEG 多帧段报 `Readable=false` |
| 视频编码（第 9 项） | 未实现 | 未实现 | 两平台 `EncodeVideo` 均返回错误，待 M2 编码决策 |
| 自动更新（第 22 项） | Sparkle 2.10.0 适配器（仅发行 tag 构建） | WinSparkle 0.9.4 动态适配器 | 两端共用签名 appcast、设置 UI、安装前 owner 门禁与 recorder 收尾；依赖版本 / SHA 固定，发布 workflow 已落盘；源码与 fake 契约通过，真实签名构建和旧版 → 新版升级尚未验收 |
| 其余各项（第 4、11、17、18、20 项） | 待定设计 | 待定设计 | 端口已冻结，实现均未开始 |

构建接线：`cmd/daygo/wails.json` 的 `preBuildHooks` 在对应平台上调用
`native/darwin/build.sh` 或 `native/windows/build.ps1`；产物分别是
`build/native/darwin/universal/libdaygo_capture.a` 与
`build/native/windows/amd64/libdaygo_capture.a`；Windows 26100 隐私路径另生成并随 EXE 放置
`daygo_windows_native.dll`。截图与应用身份分别使用
[`native/include/daygo_capture.h`](../native/include/daygo_capture.h) 和
[`native/include/daygo_application.h`](../native/include/daygo_application.h) 两份独立 ABI；
Swift 编译通过 `daygo_native.h` 同时导入，截图请求布局未改变。

Windows 的开发构建允许缺少 SDK 26100 时跳过可选 WGC helper，非空隐私名单随即失败关闭；
发行打包则要求 `build/bin/daygo_windows_native.dll` 必须存在，否则 `package-windows.ps1` 直接失败。
NSIS 使用仓库内模板同时封装 EXE 与该 DLL，不能退回 Wails 默认的“只装 EXE”模板。

实现细节与限制：[macOS 截图 v2](decisions/recording-screen-capture-v2.md)、
[macOS 应用选择与身份 ABI](decisions/recording-application-picker.md)、
[Windows 截图](decisions/recording-screen-capture-windows.md)；
真机验收矩阵：[08 §8.6.2 MC](08-testing-strategy.md#862-mc真实-macos-捕获矩阵)、
[§8.6.3 WC](08-testing-strategy.md#863-wc真实-windows-捕获矩阵)。
