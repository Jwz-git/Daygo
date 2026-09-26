# 08 测试策略

> 原则：**验证强度与风险匹配。** 纯函数跑单元测试；存储变更跑夹具与迁移测试；
> 跨界变更跑双侧契约测试；捕获与生命周期变更必须在真实 macOS 上集成测试。

## 8.1 没有参考实现，所以夹具就是规范

这个项目没有可以逐条对照的既有实现，因此"正确"不能靠比较得出，只能靠**先写下期望**。
这带来一条纪律：

> **对每一条有风险的行为，先写夹具（输入 + 期望输出），再写实现。**
> 夹具是规范；实现是对规范的一次尝试。

**夹具放在使用它的包旁边**：`internal/<pkg>/testdata/`，由 `go test` 的标准约定加载，
不集中到仓库根目录的单一 `testdata/`。已落盘的例子是
[`internal/storage/testdata/`](../internal/storage/testdata/)：三个匿名二进制夹具加一个
`//go:build ignore` 的生成器 `gen.go`——生成器入库，产物也入库，因为“由上一版本写出的库”
无法由本版本的 DDL 重建（DB-2）。`.gitignore` 用 `!**/testdata/**` 把它们从“忽略一切 .db”
中救回来，新增夹具目录沿用同一形状即可。

夹具形状：

```jsonc
// internal/analysis/testdata/idle/fully_idle_15min.json
{
  "description": "15 分钟批次，全部帧空闲 > 60s，覆盖率 0.98",
  "input": {
    "screenshots": [
      {"id": 1, "capturedAt": 1788920400, "idleSecondsAtCapture": 120,
       "segmentPath": "seg.mp4", "frameIndex": 0, "isDeleted": false}
    ]
  },
  "expected": {
    "assessment": {
      "classifierVersion": "idle_v1",
      "coverageRatio": 0.98,
      "coveredSeconds": 882,
      "batchDurationSeconds": 900,
      "qualifiedIdleRatio": 1.0
    }
  }
}
```

**改夹具的期望值必须是一次显式决定**，写进提交说明。"测试挂了就改期望"是本策略要防的
主要失效模式。

## 模块验收与证据

每个 [模块执行册](09-roadmap.md#91-模块总表) 先记录有风险行为的固定输入、期望及失败条件，
再实现最小切片。实现进度与单元 / fake 契约 / 真实集成 / 长期观察状态分开。
接口 fake 与消费者可并行推进；负责方独立验收具体能力即可接入，不等完整模块。
公共端口、DTO、事件、schema 的变更由能力负责人协调，同提交更新规范与双侧测试。

证据至少包含日期、commit、环境、命令或人工步骤、匿名输入、预期 / 实际结果及限制。
页面骨架、通过类型检查、历史编译探针和 mock 数据不构成原生或用户闭环通过。
当前自动化基线与缺口见 [09 当前代码证据](09-roadmap.md#当前代码证据)；
下列 testdata、CI 和目标测试条目不能视为全部已落盘或已运行。

## 8.2 三层结构

| 层 | 回答的问题 | 运行位置 | 频率 |
|----|-----------|----------|------|
| 单元与行为 | 计算结果对吗？ | 无头 CI，任意平台 | 每次 commit |
| 契约 | 跨界的形状与语义没漂移吧？ | 无头 CI，任意平台 | 每次 commit |
| 集成 | 整条链路在真实 Mac 上跑得起来吗？ | macOS runner + 手动 | 每个 PR / 每夜 |

前两层在设计上不依赖 CGO——这正是"Go Core 保持 `CGO_ENABLED=0` 可构建"这条规则的具体收益。

## 8.3 行为测试

按"差异在生产中不被发现的可能性"排序。序号越小越优先。

| 优先级 | 行为 | 危险原因 |
|-------:|------|----------|
| **1** | 时钟串派生（[03 §3.5](03-data-model.md#35-时钟串派生)） | 四条相互作用的启发式；偏差会静默把卡片放到错误的日期——数据看似存在却找不到 |
| **2** | 凌晨 4 点逻辑日边界 | 每个按日查询都依赖它。边界错一次，整天看起来是空的 |
| **3** | 分批规则（[04 §4.3.1](04-data-flow.md#431-分批规则)） | 丢弃末批的差一错误会造成批次重复处理或永久停滞 |
| **4** | 空闲判定（[04 §4.4](04-data-flow.md#44-空闲判定)） | 判错要么为空闲时间付 LLM 费用，要么把真实活动标成 Idle |
| **5** | LLM 输出解析与修复 | 回归看起来像"provider 不稳定"，很难归因 |
| **6** | `ReplaceCardsInRange` 的重叠与改写吸收范围（含 System 回退卡，[03 §3.5](03-data-model.md#35-时钟串派生)） | 错删用户可见数据 |
| **7** | 每周合计与占比 | 可见但不具破坏性 |
| **8** | 失败批次分组（60 秒容差合并） | 影响重试入口的可用性 |
| **9** | provider 回退的粘性 | 语义细微，容易在重构中丢失 |

### 8.3.1 基于属性的测试

夹具只覆盖想到的情况。三个领域需要生成输入：

```go
// 任何在窗口内解析的时钟串，都必须落在该窗口向两侧各放宽一天的范围内。
// 覆盖"三天中取最近"的逻辑。
func TestResolveClockWithinWindow(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        anchor := rapid.Int64Range(0, 2_000_000_000).Draw(t, "anchor")
        hour   := rapid.IntRange(0, 23).Draw(t, "hour")
        minute := rapid.IntRange(0, 59).Draw(t, "minute")
        // 断言 |resolved - anchor| <= 12h + 1min
    })
}

// 逻辑日归属必须是全函数且稳定：每个时刻恰好属于一个逻辑日，
// 相邻时刻绝不跳过某一天。
func TestDayBoundaryTotality(t *testing.T)

// 分批必须是划分：每个输入帧恰好出现在一个批次或被丢弃的尾批中，
// 不会两者都在，也不会两者都不在。
func TestBatchingPartitions(t *testing.T)
```

三项都必须在多个 `TZ` 下运行，至少包括：`UTC`、`America/Los_Angeles`（DST）、
`Asia/Kolkata`（半小时偏移）、`Australia/Lord_Howe`（半小时 DST）、`Pacific/Chatham`。
常见时区下不会出问题，恰恰是这些边角会。

### 8.3.2 确定性的 LLM 测试

LLM 的**输出**不确定，所以不比较端到端文本。LLM 输出的**解析**完全确定，而缺陷正在这里。

`llm_calls` 只允许保存 attempt 元数据，不能成为 payload 来源。解析器始终使用人工构造且
验证不可逆匿名化的响应夹具，绝不从本机调用记录或真实服务提取用户 payload：

```text
internal/ai/testdata/llmresponses/
├── openai/transcribe/{ok,malformed_json,fenced_block,truncated}.json
├── openai/cards/{ok,prose_preamble,trailing_comma,wrong_types}.json
├── openai_responses/{transcribe,cards}/{ok,...}.json
└── anthropic/{transcribe,cards}/{ok,...}.json
```

三个协议各有一套响应形状（`openai` / `openai_responses` / `anthropic`），
解析回归必须分别覆盖，不能用其中一个的夹具代表另外两个。

每遇到一种新的畸形形态，就**加一个匿名夹具**；保存的是可复现形态，不是用户活动正文。

## 8.4 数据与迁移测试

Windows 录制目录变更需使用匿名分段夹具验证：复制取消、目标满盘或权限不足、提交前后强制退出、复制校验、旧文件清理重试、旧数据库备份的恢复门禁、只读实例拒绝迁移和录制状态恢复。真实 Windows 验收还需跨盘迁移、拔盘后启动与重连、MP4 收尾和历史帧回放；这些不能由纯 Go 夹具替代。见[目录迁移决策](decisions/recording-directory-windows.md)。

| ID | 测试 | 断言 |
|----|------|------|
| DB-1 | schema 幂等 | 对全新库执行迁移两次，第二次产生零 DDL 变更 |
| DB-2 | 迁移链 | 每个版本都有"旧库 → 新库"夹具，逐版本升级后结构与数据均符合期望 |
| DB-3 | 读遍每张表 | 每个只读 repository 方法在所有夹具上返回非错误 |
| DB-4 | 往返完整性 | 打开、全读、关闭后 `PRAGMA integrity_check` 仍为 `ok` |
| DB-5 | metadata JSON 解码 | 每个 `timeline_cards.metadata` 都能解码，且 `distractions`、`appSites` 往返一致 |
| DB-6 | PRAGMA 一致性 | 断言 `journal_mode=WAL`、`synchronous=NORMAL`、`busy_timeout=5000` 生效（查询回读） |
| DB-7 | 损坏分类 | 截断的库触发备份恢复；只读或磁盘已满的环境故障**只告警不删文件** |
| DB-8 | 并发访问 | 一个写入实例 + 一个只读实例持续 1 小时：零 `SQLITE_BUSY` 风暴、零损坏 |
| DB-9 | 清理边界 | 设小上限并超量填充；`recordings/` 收敛，且**活跃分段绝不被删除** |

夹具库至少三个变体：典型安装、边界数据（跨午夜卡片、DST 当天、整天空闲、脱敏帧）、空库。

## 8.5 契约测试

见 [05 §5.10.3](05-interface-contract.md#5103-接口测试门禁) 的完整表。要点：

- **DTO 形状**用黄金 JSON 快照锁定字段名、可空性、枚举取值；
- **错误码**遍历绑定方法的错误路径，断言全部是 `*apperr.Error` 且 code 在封闭表内；
- **事件名**在 Go 与前端各有一份常量，测试断言两份一致；
- **platform 端口**用同一套 `platformtest` 跑 fake 与真实适配层：`Suite`（出图完整性、
  不覆盖已有文件、调用前取消、非法请求）加 `SuitePermission` / `SuitePrivacy` /
  `SuiteNoDisplay` 三个需要驱动 OS 状态的套件，逐条断言见
  [05 §5.7.4](05-interface-contract.md#574-fake-实现与契约测试)。
  fake 四套全绿；真实能力经用户确认验收（见 09 §9.1.1），但仓库中没有两平台
  四套真实契约逐项运行记录，不能将用户确认记为套件自动化全绿。

## 8.6 集成测试

| 层 | 平台 | 适配层 | provider | 运行时机 |
|----|------|--------|----------|----------|
| **L1 核心循环** | 任意 | `platform/fake` | stub | 每次 commit |
| **L2 适配层循环** | macOS | 真实 | stub | 每个 PR |
| **L3 完整循环** | macOS | 真实 | 真实本地模型 | 每夜 |
| **L4 自用** | macOS | 真实 | 用户自己的 | 安全录制与真实分析接入后持续；**已于 2026-09-22 经用户实测验收（无逐项运行记录）** |

L1 价值最高：`platform/fake` 提供合成帧，stub provider 返回预设观测，整条流水线——分批、
空闲判定、卡片改写、日期归属、时间线构建——能在 Linux 上一秒内无头跑完，没有授权弹窗，
也没有 LLM 费用。

L3 用本地模型而不是云端 provider：本地、免费、足够可复现，适合每夜运行。

Vite 浏览器测试可在 URL 中显式选择匿名测试数据：`testData=on` 覆盖有数据状态，
`testData=off` 覆盖无 Wails bridge、无测试替身的不可用 / 空状态；参数可位于 hash 路由
查询中（如 `#/timeline?testData=off`）。选择在当前页面的 SPA 路由切换期间保持，另开标签页
可并行测试另一状态。省略参数沿用开发默认 `on`，生产构建无条件关闭。时间线、日报、周报、
设置、Provider 与 Chat 必须共用该开关，禁止各自绕过。

### 8.6.1 L2 用例

| ID | 测试 | 断言 |
|----|------|------|
| IT-1 | 捕获落库 | 启动捕获，等 5 个间隔后停止；JPEG 数量、时间与 screenshot 行一致 |
| IT-2 | 原子交付 | 成功、编码失败、取消和同名冲突；只出现完整 final 或无 final，不覆盖已有文件 |
| IT-3 | 配置切换 | 捕获中改分辨率；下一次调用使用新请求，前一张结果不被改写 |
| IT-4 | 帧解码 | 解码每个已提交 JPEG；尺寸与对应 `CaptureRequest` / `CaptureResult` 一致 |
| IT-5 | 隐私遮蔽 | 屏蔽应用前台时 Capture 返回 blocked 且无真实图；Go 写占位图并标记 `redacted` |
| IT-6 | 睡眠唤醒 | 睡眠后暂停调用；唤醒 5 秒后恢复，过期截图不入库 |
| IT-7 | 锁屏解锁 | 观察到暂停与 0.5 秒后恢复，且用户偏好未被改写 |
| IT-8 | 主显示器切换 | 修改系统主显示器；下一次 Capture 使用新的主显示器，无 native 跨调用状态 |
| IT-9 | 授权撤销 | 捕获中撤销授权；后续调用失败、发通知，**不覆盖用户偏好** |
| IT-10 | 适配层崩溃 | 终止在途适配调用；Go 保留 pending，恢复后对账，不接纳未知文件 |
| IT-11 | Go 崩溃 | 文件发布前后分别 `kill -9` Go；启动对账恰好补齐或重试，不产生重复行 |
| IT-12 | 清理 | 小上限 + 超量填充；`recordings/` 收敛，活跃分段未被删 |
| IT-13 | 并发锁 | 启两个实例；恰好一个捕获，另一个只读 |
| IT-14 | 窗口关闭 | 关闭最后一个窗口；进程存活，捕获继续，状态栏可重开窗口 |

IT-14 属于 G-host，验证整个项目的宿主前提：真实 macOS 上关窗后至少 10 分钟持续离散捕获、
状态栏可重开，并验证激活策略切换。心跳存活仅是前置探针。**该门禁已于 2026-09-22 经用户实测验收（无逐项运行记录）。**

2026-09-26 菜单栏 / Dock 增量回归：先跑 `bash native/darwin/system-smoke.sh`（独立匿名
AppKit 宿主，不捕获、不接用户库），核对主 / 工作线程策略调用、状态栏可用性与快照去重、
非阻塞操作反馈、主菜单标题 / selector / 快捷键保持及合成关机通知。它不替代以下真实 Wails 步骤：

- Dock 开关开 / 关分别保存、重启、关窗、Cmd+Q、状态栏重开；核对窗口可见性、Dock / Cmd+Tab
  与保存偏好一致。状态栏不可用夹具须保留 regular；原生策略失败后显式重开须可重试。
- 定时暂停后锁屏，验证不可手动恢复；到期仍锁定时清除恢复时刻、保持系统暂停，解锁后恢复。
  只读实例显示禁用录制控制，捕获重试显示警告，操作失败反馈不得包含原始错误或路径。
- 菜单展开与九种语言切换，核对状态首行、暂停项、主应用 / 编辑 / 窗口菜单及快捷键。
- 注入分段收尾失败后状态栏真退出：默认保留应用，明确选择仍然退出才能终止；真实关机 / 注销
  不被软退出拦截且不触发授权自重启。使用匿名数据运行，再完成 IT-14 十分钟持续捕获回归。

上述已实现真实回归于 09-26 用户确认已验收，未附逐项记录；这是新增确认，不沿用历史 G-host
覆盖增量。共享状态栏 ABI 已升为 3；Windows DLL / 通知区也在已实现功能确认范围内，
本机无 cgo 构建仍不等于 Windows 原生运行记录。

测试按能力归属，见 [09 §9.7](09-roadmap.md#97-需求接口与测试归属)：recording 主责
IT-1–11/14，data 主责 IT-12/13；跨界场景共同验证。进程内 / 外适配的故障注入须随选型记录
等价观察方法，保留异常退出 / 重放语义，不假定已有独立适配进程。

### 8.6.2 MC：真实 macOS 捕获矩阵

IT 用例验证“整条链路跑得起来”，MC 用例验证“这台真机上的截图原语行为正确”。
两者不可互相替代：fake 与契约测试通过不构成任何一条 MC 通过。全部 MC 结果按
[09 §9.6](09-roadmap.md#96-集成检查点与证据) 的格式记录（日期、commit、系统版本、机型、
脱敏观察），命中隐私项失败时直接阻塞真实捕获接入。

| ID | 场景 | 必须观察到的结果 |
|----|------|------------------|
| MC-1 | 以 1 / 10 / 60 秒间隔连续调用 | 每次调用最多一张图；**系统屏幕录制指示器不常亮**；无持续 capture session |
| MC-2 | 双显示器，光标移到副屏 | 始终只截系统主显示器；不产生显示器选择或切换状态 |
| MC-3 | Retina、旋转、SDR / HDR | 输出尺寸与 `CaptureResult` 一致，色彩无明显偏差 |
| MC-4 | 未授权屏幕录制 | `permission_denied`，无文件，不自动循环弹窗 |
| MC-5 | 捕获过程中撤销授权 | 后续调用固定返回 `permission_denied`；**用户保存的“希望录制”偏好未被改写** |
| MC-6 | 屏蔽应用处于前台 | `blocked`，未调用系统截图 API，最终路径与临时文件都不存在 |
| MC-7 | 屏蔽应用在后台但窗口可见 | 图像中不出现其内容（`excludingApplications` 生效） |
| MC-8 | 快速切换前台、多空间、全屏与系统窗口 | 不出现屏蔽应用内容；无法可靠判定时返回 `privacy_unsupported` 而不是降级截图 |
| MC-9 | 睡眠 / 唤醒 | Go 停止与恢复调用；native 无残留会话；唤醒后 5 秒内的过期结果不入库 |
| MC-10 | 锁屏 / 解锁 / 屏保 | 同上，恢复延迟 0.5 秒；`idle` 不被系统事件改成 `capturing` |
| MC-11 | 开发签名 / Release 签名 / 升级后签名 | TCC 身份稳定，同签名升级不触发新的授权提示（[风险 C-3](10-risks.md#c-3身份与授权不稳定)） |
| MC-12 | 连续 24 小时分间隔调用 | 内存、线程、文件描述符与系统对象无增长；失败可按调用计数 |

MC-6–MC-8 是 [07 §7.2](07-privacy-security.md#72-捕获侧的两层保护) 两层保护的实机证据，
缺一条就不能宣称隐私双保护已验收。**MC 实机矩阵与隐私双保护已于 2026-09-22 经用户实测验收（无逐项运行记录）。**

### 8.6.3 WC：真实 Windows 捕获矩阵

Windows 适配器已落盘，WC 功能范围经用户确认已验收（无逐项记录）；正式签名材料仍缺
（[决策记录](decisions/recording-screen-capture-windows.md)，[09 §9.8 第 18 项](09-roadmap.md#98-待定设计清单)）。
已保留的原生 smoke 证明 DXGI 能生成可解码非黑 JPEG；完整功能验收另记为用户确认，
以下矩阵保留为复核标准，不将未记录命令倒填为通过。

| ID | 场景 | 必须观察到的结果 |
|----|------|------------------|
| WC-1 | 空屏蔽名单下单次调用 | 主监视器出图，尺寸 / 字节数与 `CaptureResult` 一致，可解码且非全黑 |
| WC-2 | 屏蔽名单非空、前台命中 | `blocked`，无文件 |
| WC-3 | 屏蔽名单非空、前台未命中 | build 26100+：WGC 画面排除后正常出图且屏蔽内容不可见；更旧系统：`privacy_unsupported`，无文件，**不得**降级为“只检查前台”（见 [Windows 决策记录](decisions/recording-screen-capture-windows.md)） |
| WC-4 | 前台为传统 Win32（无 AUMID）应用 | `privacy_unsupported`；记录该限制而不是放宽判定 |
| WC-5 | 目标路径已存在 | `io`，已有文件字节不变 |
| WC-6 | 多监视器、缩放（DPI）、旋转 | 只截主监视器，方向与尺寸正确 |
| WC-7 | 受保护内容（`SetWindowDisplayAffinity`）、独占全屏、驱动返回空帧 | 要么正确出图，要么明确失败；**GDI 回退不得绕过内容保护** |
| WC-8 | 连续 24 小时分间隔调用 | 资源无增长；COM / D3D 对象无泄漏 |

2026-09-11 的 Windows 11（NT 10.0.26200、NVIDIA RTX 4060 Laptop GPU、双显示器）记录：
WC-1 通过一次原生 smoke 与一次 Go cgo smoke。首次 `AcquireNextFrame` 可能只有鼠标更新
（`AccumulatedFrames=0`、`LastPresentTime=0`）且纹理全零；实现会在同一 timeout 预算内继续等待，
随后取得非零 BGRA 桌面帧并输出 1280×720 JPEG。当天的非空屏蔽名单失败关闭 smoke（`privacy_unsupported`）
已被 2026-09-13 的 WGC 画面排除证据替代（build 26100+ 非空名单改走 WGC，排除图像已证明后台窗口
从画面消失且底层窗口可见，见 [Windows 决策记录](decisions/recording-screen-capture-windows.md)）；
这仍不是 WC-2/3/4 的完整隐私验收。WC-5–8 未运行。

WC-3 与 WC-4 一起决定一个产品事实：**build 26100 以下的 Windows 配置了屏蔽应用就拿不到画面**
（失败关闭）。完整 WC 矩阵验收前，Windows 不进入发布构建。

### 8.6.4 WD：真实 Windows 分发矩阵

WD 与 WC 独立：WC 证明捕获安全，WD 证明最终安装产物没有漏文件、错签或破坏数据。打包入口
`scripts/package-windows.ps1` 已实现，但尚未在真实 Windows 上执行，因此下表当前均未通过：

| ID | 场景 | 必须观察到的结果 |
|----|------|------------------|
| WD-1 | SDK 26100、VS 2022、MinGW、NSIS 的干净检出打包 | 产出安装器和 `windows-package.json`；manifest 的 commit、大小、SHA-256 与产物一致 |
| WD-2 | Authenticode 构建 | 构建目录 EXE/DLL、安装器及安装后的 EXE/DLL 均通过 `signtool verify /pa`；证明内层是签名后的最终字节 |
| WD-3 | machine / user 交互安装 | 安装范围、快捷方式、卸载注册表项正确；启动时 DLL 可加载，通知区可重开，正式 recorder 提交一帧 |
| WD-4 | `/S` 静默安装与卸载 | exit code 可判定；二进制、快捷方式和卸载项移除，用户数据库、录制与 Credential Manager 密钥保留 |
| WD-5 | 从前一版本覆盖升级 | schema、匿名数据、设置、凭据与捕获锁语义保留；中断后可恢复，不靠删除数据库 |
| WD-6 | 缺 SDK 26100 / DLL 或产物被篡改 | 打包或哈希/签名检查失败，绝不生成看似成功但隐私能力缺失或启动即缺 DLL 的发布候选 |

### 8.6.5 长时间断言

录制到时间线的真实闭环先做连续 7 天自用（G-loop）：无未解释缺口、失败可见可操作。**该闭环已于 2026-09-22 经用户实测验收（无逐项运行记录）。**
下表是独立的 G-stability 证据，14 天、跨一次真实 DST、跨周一分别记录；7 天不能把它们标为通过。**长期观察已于 2026-09-22 经用户实测验收（无逐项运行记录）。**
MC-12 / WC-8 的 24 小时观察是本表的前置条件，不是它的替代。
只有安全录制与真实分析接入后才开始累计相应观察窗口：

| 检查 | 窗口 | 断言 |
|------|-----:|------|
| 捕获连续性 | 14 天 | 除已记录的睡眠/锁屏窗口外，相邻帧间隔不超过 `interval × 3` |
| 日期翻转 | 14 天 | 每张卡片的 `day` 等于按 4 点边界算出的值；**特别验证 03:30–04:30 创建的卡片** |
| DST 转换 | 跨一次转换 | 没有重复或缺失的小时；没有 `endTs <= startTs` 的卡片 |
| 周边界 | 跨一个周一 | 周范围无重叠、无间隙地划分 |
| 批次活性 | 14 天 | 没有批次保持 `processing` 超过 30 分钟 |
| 磁盘收敛 | 14 天 | `recordings/` 保持在配置上限的 5% 以内 |
| 内存稳定 | 14 天 | 第一小时后 RSS 增长低于 10%——用于捕捉帧缓存泄漏 |

### 长期内存对照流程（2026-09-26）

四项优化分开验收：逐帧 autoreleasepool、图标载荷预算、256 像素帧条、隐藏时媒体释放。
短期原生复现入口（仅合成帧、独立临时目录，不用真实库 / 截图或 Provider）：

```bash
native/darwin/build.sh
DAYGO_NATIVE_MEMORY=1 go test -a ./internal/platform/darwin -run '^TestNativeDecodeMemoryPlateau$' -count=1 -v
```

600 次读取分别在 300 / 600 次后静置 3 秒并回收 Go 堆；每次启动三个独立进程，
每个进程增量不超过 5 MiB。该门禁不代表 24 小时 / 14 天通过，不在生产定时触发 GC。

真实 UI 对照使用同一机器、配置、原生工具链及匿名隔离资料，依次跑未修改版本与候选版本，
不在真实用户库运行测试 / 迁移。基准版本为 `d39c16b`，候选版本以实际 commit 记录。
选定同一张包含至少 14 帧的匿名卡片；固定回放倍率 40x，每轮播放 60 秒、打开帧条并拖动
至 25% / 50% / 75%、暂停后隐藏 10 分钟，再从状态栏重开。每次重开检查选卡、倍率、进度、
展开状态与未保存草稿；另覆盖 Cmd+Q、普通失焦、真退出收尾和 4 点逻辑日翻转。
图标场景固定相同匿名网站 / 应用列表，分别访问超过条目上限和载荷预算的集合，再回看被淘汰项。
后台录制场景固定截图配置与 Provider，排除已记录的睡眠 / 锁屏时段检查捕获和分析连续性。
真实捕获 / Provider 闭环另记门禁，不把匿名媒体或 fake 捕获当作实测通过。

macOS 读数入口（以下 PID 要替换为该次测试实例的真实 PID；每个进程逐项记录，不相加历史峰值）：

```bash
python3 scripts/sample-memory.py --pid main=123 --pid web=124 --pid gpu=125 --pid network=126 --duration 86400 --interval 300 --output build/diagnostics/memory-before.csv
```

候选版本使用另一份 `memory-after.csv`；14 天将 duration 设为 1209600。
采样仅记录时间、标签、PID、RSS 和 footprint，不收集像素、窗口标题、路径或 LLM 内容。
采样固定 PID 的启动时间身份；进程重启 / 缺失 / PID 被复用会标为不可用，需补充新的 PID 记录，不能把缺失当作内存归零。
RSS 与 footprint 分开比较；记录系统压力，舍弃第一小时预热，以相同操作状态的稳定窗口中位数
比较基线与候选值。14 天将预热后的第一个完整小时与末一个同状态完整小时对照，RSS 增长
低于 10%；状态不一致或样本不可用时记为证据不足，不能据此宣称通过。缓存账目由预算夹具
核验，不以此采样器的进程读数推算缓存占用。

本轮已实现窗口 / 捕获增量与长期观察于 09-26 用户确认已验收；未附十分钟、24 小时 A/B
或 14 天 RSS 的逐项记录与数值，不能宣称已量化内存下降或满足某个数值阈值。
确认范围见 [09 §9.1.1](09-roadmap.md#911-本轮验收记录与证据边界)，以上流程用于复核。

## 8.7 有意不测试的内容

明确写出来，避免虚假信心：

| 不测 | 原因 | 补偿控制 |
|------|------|----------|
| LLM 输出文本 | 本质不确定 | 断言结构；解析走夹具 |
| 像素级 UI 一致性 | v1 追求功能与体验，不追求某个基线的像素复刻 | 功能模块验收时人工走查 |
| 编码器字节级一致性 | 输出随芯片与系统版本变化 | 改为断言结构与可解码性 |
| CI 中的完整更新流程 | 需要签名、公证与托管 | 每次发布前用已发布 build 手动验收 |
| 真实 provider 的限流行为 | 成本高且不稳定 | 用 stub HTTP 状态码测错误路径 |

第一行是本策略坦诚承认的限制：可以证明分批、判定、解析、存储和展示的结果一致，
**无法证明 LLM 给出相同文本**。这可以接受——产品从未依赖这个属性。

## 8.8 CI 门禁

```bash
# 无头，任意平台，每次 commit
CGO_ENABLED=0 go build ./...
CGO_ENABLED=0 go test ./internal/...
go vet ./...
gofmt -l .            # 必须无输出

# 前端
npm --prefix frontend ci
npm --prefix frontend run test:unit
npm --prefix frontend run typecheck
npm --prefix frontend run build
```

**干净检出的依赖安装、绑定生成与前端检查有顺序依赖，Go 命令也依赖引导。** `internal/app` 导入 `frontend`，后者的
`go:embed all:dist` 在 `dist` 不存在时匹配不到任何文件，**整个模块无法编译**——包括上面
的 `go build` 与 `go test`。而 `dist` 与 `wailsjs` 都是生成产物、不入库
（`05 §5.5.5` 规则 3），两者又互相依赖：生成绑定需要可编译的 Go 树，可编译又需要 `dist`。

所以在干净环境（新 clone、CI runner）上，**必须先跑一次引导**再执行上述任何命令。
`scripts/gate.sh` 已内置该顺序，并包含 Linux / Darwin / Windows 的无 cgo 核心交叉构建、
前端单测与 Python 文档 / 安装器夹具检查：

```bash
./scripts/gate.sh
```

引导逻辑本身在 `scripts/bootstrap-frontend.sh`，`scripts/dev.sh` 与 `gate.sh` 共用；
顺序为「占位 `dist` → 生成绑定 → 真实 bundle」。两条规则对每个平台都成立，因此 Windows 的
`scripts/dev.ps1` 也内置同一顺序（不同 shell 无法共用脚本，只能各写一份，改一处要同时改另一处）：

1. **占位 `dist` 只在缺失时写。** 无条件写会毁掉真实 bundle 的 `index.html` 却留下它的
   `assets/`，应用随后提供的是一张空白页。
2. **绑定无条件重新生成。** 绑定一旦过期，`vue-tsc` 报的是「缺少某个成员」而不是「绑定陈旧」，
   指向的是前端文件；按存在性判断会让这个状态一直留着。

2026-09-12 已在 Windows 11 amd64 / PowerShell 7 上执行合并后的 `scripts/dev.ps1`：依赖同步、
无条件 Wails bindings 生成、真实 bundle 判断、Go 1.25 `nodwarf5` 局部 workaround、开发 EXE
编译及 WebView2 启动全部通过；运行前后 `frontend/package-lock.json` SHA-256 未变化。

`gate.sh` 最后还会跑 `scripts/check-docs.py` 和 `scripts/windows-installer/test_installer.py`。
有 `python3` 才执行，否则跳过；缺 NSIS 时安装器编译跳过，非 Windows 上的执行用例也跳过，
跳过不能写成通过。`gofmt -l .` 必须无输出，不能只凭脚本退出码判断格式通过。
文档脚本检查 markdown 链接与小节锚点是否存在、
有没有没被任何文档链接到的孤立文档。它只保证文档**内部自洽**；文档与代码是否一致仍然
靠“同一个 commit 内修正文档”这条纪律，不靠脚本。

**Linux 上必须全绿。** 这条门禁反向约束了所有接口设计：任何让核心包无法在无 macOS
环境编译或测试的设计都是错的。
