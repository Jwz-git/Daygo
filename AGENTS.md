# Daygo — Go + Wails 3 + Vue 3 Agent 工程指南

> **以可验证性为最高约束，优先输出能复核、能被推翻、能执行的回答。**

## 1. 判断与执行原则

- **准确性优先于迎合**：解决问题优先于形式完整；用户前提有误时指出反例、隐藏假设与边界，只在新证据、更可靠来源、更完整推理或可复现结果出现时修正结论。
- **区分证据层级**：明确区分观察事实、候选解释、因果判断与行动建议；不把经验、猜测或未运行代码写成确定事实，不从相关性推出因果，也不从个案推出普遍结论。
- **诚实标定精度**：有依据时明确判断并说明前提、反例和验证方式；证据不足时降低结论精度，说明能确定什么、不能确定什么、缺什么及如何验证。
- **变更前先核验**：涉及版本、接口、脚本、配置或模型能力时优先核验当前代码、可复现实验与官方文档；无法核验时标注范围与时间，来源冲突时分别记录。
- **最小可执行推进**：先识别目标与硬约束，默认给最小可执行方案；仅在缺失信息显著影响结果或带来风险时提问，否则明确假设并执行。复杂任务拆成可检查阶段（输入、操作、输出、完成标准）。
- **零编造**：不编造资料、引用、版本号、接口、测试结果或实现行为；不以免责声明代替分析。输出先给结论，再给依据、边界、验证方式与下一步。

---

## 2. 项目定位与当前状态

Daygo 是一个本地优先的常驻桌面后台 Agent：按间隔离散截取当前系统主显示器，分批交给用户配置的 LLM，将活动整理为时间线、每日复盘、每周回顾与计划提醒。

- **新项目边界**：不承接任何既有产品的数据、身份或对外契约。
- **技术基线**：日常开发分支 `test` 使用 Go 1.26.10（以 [`go.mod`](go.mod) 为准）、Wails `v3.1.0` 与 Vue 3 + TypeScript；`main` 为稳定分支。
- **平台定位**：macOS 为当前主线开发平台；Windows（x64）与 Linux 为已排期的一等发布目标。
  - Windows 已实现 DXGI/WGC 截图、分段编码降级、应用身份、系统事件、通知区、Credential Manager、`LockFileEx`、录制目录迁移、原生通知与 NSIS 打包链路。
  - Linux 已验证 arm64 宿主构建（GTK4 / WebKitGTK 6.0）与 `secret-tool` 密钥适配；Capture / System 仍为 `unsupported` 桩。
  - 各平台真机能力均须遵守 G-host / G-native 验收门禁，缺可复核记录时不得宣称「已支持」。
- **文档与事实权威**：
  - 实现与验证现状以 [`docs/09-roadmap.md §9.1`](docs/09-roadmap.md#91-模块总表) 为准，细节见 `docs/modules/<module>.md`；跨界接口以 [`docs/05-interface-contract.md`](docs/05-interface-contract.md) 为准。
  - `docs/01`–`10` 是设计规格，不代表全部目标已实现；文档与代码冲突时，**以当前代码和可复现实验为准，并在同一 commit 内同步修正文档**。
- **历史验收与证据边界**：
  - 2026-10-13 用户确认此前仍待验收的已实现能力（范围为 `test` 的 `bf49b34` 工作树及此前能力，含 09-26 后增量、真实 Wails / Provider / 外部客户端闭环、两平台通知、Windows 目录迁移、安装升级、长期观察及原待验收的 G-native 发布身份）全部验收完成，未附逐项运行记录；统一证据见 [`docs/09-roadmap.md §9.1.1`](docs/09-roadmap.md#911-本轮验收记录与证据边界)。
  - **历史验收不自动覆盖后续增量**；10-13 之后的新改动须独立验证。未实现功能与待定设计保持原状态；自动化失败 / 跳过 / 未运行记录不改写为通过；Developer ID / 公证 / Authenticode 材料与产物签名未在文档同步中独立核验。
- **待定设计（[`docs/09-roadmap.md §9.8`](docs/09-roadmap.md#98-待定设计清单)）**：
  - 屏幕捕获 API、钥匙串访问、HEVC 分段、自动更新链路等已在 `docs/decisions/` 落定；仍标「待定设计」的项（如平台适配最终形态、系统事件订阅等）在决策落盘前**不要按单一候选大规模实现**，也不要删除其它候选路径。
  - 业务代码只面向 `internal/platform` 端口接口编写；实现放入既有适配包（如 `darwin`、`windows`、`secrets`、`fake`），不把规划目录写成现状，不把可移植逻辑写进平台适配层。

---

## 3. 功能模块与门禁

模块是协作与验收单位，可并行推进；依赖精确到具体能力（[`docs/09-roadmap.md §9.3`](docs/09-roadmap.md#93-能力接入表)），共享能力独立验收即可接入。

| 模块 | 用户结果与职责 | 状态边界 |
|---|---|---|
| `recording` | 常驻录制、授权、暂停、系统事件、隐私屏蔽与分段恢复 | 部分实现 |
| `providers` | AI 配置（API Key / ChatGPT 订阅）、密钥、多模型回退链与试测 | 部分实现 |
| `timeline` | 自动时间线、分类、帧条回放、编辑、重处理与卡片合并 | 部分实现 |
| `daily` | 每日摘要、工作流、日记、目标与提醒通知 | 部分实现 |
| `weekly` | 周时长、专注分析、分类占比与 Token 图表（不等待 daily） | 部分实现 |
| `data` | 数据库基础、实例锁、备份恢复、磁盘上限与诊断 | 部分实现 |
| `preferences` | 外观、九语言 i18n、通用设置容器与前端基础接入 | 部分实现 |
| `delivery` | 签名身份、首次引导、安装器与 Sparkle / WinSparkle 更新 | 部分实现 |
| `plan` | 今日计划时间块、完成标记与开始 / 分心系统通知 | 部分实现 |
| `agent` | CLI 查询、`agent.sock` 受控写入与 stdio MCP | 部分实现（v1 不交付） |
| `chat` | 应用内自然语言问答与沙箱内受控编辑 | 部分实现（v1 不交付） |
| `play` | 由当日 / 当周卡片生成的拼岛休闲小游戏 | 仅策划案，未开始 |

所有权分工：数据库连接 / 迁移 / 锁归 `data`，业务 repository 随功能交付并统一放在 `internal/storage`；通用设置访问归 `preferences`，具体设置字段与语义归各自功能；时间语义由 `timeline` 维护。

### 门禁原则（详见 [`docs/09-roadmap.md §9.4`](docs/09-roadmap.md#94-全局门禁与阻塞范围)）

分别记录实现状态与验证状态；fake 契约通过不等于真实功能完成，生产绑定绝不返回测试数据。

- **G-core（持续硬门禁）**：Go Core 始终保持 `CGO_ENABLED=0` 可构建且能在 Linux 通过测试；SQLite 驱动问题须重新决策，不得自动引入违反该门禁的 cgo 后备。
- **G-host（硬门禁）**：真实 macOS 上关窗后 ≥10 分钟进程存活、离散捕获继续、状态栏可重开、激活策略可切换；已于 2026-09-22 经用户实测验收（无逐项记录），大规模 UI 扩张已解锁。
- **G-native**：原生选型、签名公证、授权 / 密钥身份、通知与更新可行性提前验证；原待验收项于 10-13 用户确认已验收（证书与签名材料未独立核验）。
- **G-data / G-loop / 长期观察**：隐私双保护、唯一写入 / 捕获者、连接层只读、分段恢复、7 天闭环及 14 天 / DST / 跨周观察已于 09-22 / 10-13 经用户确认验收（无逐项记录）；新改动仍需独立回归。

---

## 4. 架构与依赖方向

详见 [`docs/02-architecture.md`](docs/02-architecture.md)：

```text
Vue 3 + TypeScript
        ↓ 生成的 Wails v3 TypeScript 绑定
internal/app                唯一依赖 Wails 的层
        ↓
Go services                 analysis / ai / insight / chat / recorder / agent* / mcp
        ↓
Go foundation               storage / settings / domain / timeutil / favicon
        ↓ 端口接口
internal/platform           Capture / Media / System / Secrets / Updater / Notifications
        ↓
平台适配层                   唯一接触系统能力的地方（darwin / windows / linux / fake）
```

**不可违反的 6 条依赖规则**：

1. foundation 不依赖 service、app、Wails 或适配层实现。
2. service 通过消费者侧接口协作，不导入其他 service 的内部实现。
3. 只有 `internal/app` 可以依赖 Wails；Go Core 必须能无 GUI 测试。
4. 只有 `internal/platform` 接触平台适配层实现；业务服务不构造平台 IPC、不硬编码系统路径、不直接调系统 API。
5. `internal/storage` 是 Go 侧唯一允许包含 SQL 和打开业务数据库连接的包。
6. Vue 只通过生成的绑定与薄 wrapper 访问 Go，不直接接触数据库或文件系统。

不要仅为匹配架构图而搬动已验证代码；目录重构应独立提交并始终保持可构建。

---

## 5. 构建、测试与验证门禁

### 一键提交门禁

```bash
./scripts/gate.sh
```

- **干净检出引导顺序**：`internal/app` 导入 `frontend`，后者的 `go:embed all:dist` 要求 `frontend/dist` 存在，而 `vue-tsc` 又需要 `frontend/bindings`（生成绑定要求 Go 可编译）。因此必须先运行 `./scripts/bootstrap-frontend.sh`（占位 dist → 生成绑定 → 真实 bundle），`gate.sh` 会自动处理。
- **格式与脚本检查注意**：`gate.sh` 中 `gofmt -l .` 只打印未格式化文件而不返回非零退出码，**必须人工确认其无输出**；缺少 `python3` 时 `check-docs.py`、`test-entrypoints.py` 和 `test_installer.py` 会被跳过，不得将跳过报成通过。
- **纯文档改动**：至少运行 `python3 scripts/check-docs.py` 与 `git diff --check`，并核对涉及的代码 / 命令事实。

### 分层命令

```bash
# Go（纯 Go 核心门禁，须在 Linux 通过）
gofmt -w <changed-go-files>
CGO_ENABLED=0 go test ./internal/...
go vet ./...
CGO_ENABLED=0 go build ./...

# 前端（严格遵循 frontend/package-lock.json，不混用 pnpm/yarn）
npm --prefix frontend ci
npm --prefix frontend run test:unit
npm --prefix frontend run typecheck
npm --prefix frontend run build
```

### 原生库与桌面打包

```bash
# macOS：Swift → arm64 + x86_64 → lipo
native/darwin/build.sh        # → build/native/darwin/universal/libdaygo_capture.a

# Windows（PowerShell，需 MinGW-w64 + VS2022 C++ + Windows SDK 26100）
powershell -NoProfile -ExecutionPolicy Bypass -File native\windows\build.ps1
                              # → build/native/windows/amd64/libdaygo_capture.a + daygo_windows_native.dll
```

- 仅带 `daygo_desktop` 标签的 cgo 宿主需要原生库（由 `scripts/build-desktop.sh` 与 Windows 构建入口自动调用）；手动跑原生 smoke 时先重建原生库并加 `go run -a`，避免链接旧 archive。
- 修改共享原生 ABI 时同步 Go / Swift / C++ 两侧并重建静态库 / DLL；macOS 编译或合成 smoke 不能替代 Windows 真机验证。
- Windows 发布候选须在 Windows 主机执行 `scripts/package-windows.ps1 -Version <x.y.z> -RunSmoke`；脚本详情见 [`scripts/README.md`](scripts/README.md)。

### 验证强度与夹具先行

按风险匹配验证层级（[`docs/08-testing-strategy.md`](docs/08-testing-strategy.md)）：纯函数跑单测；存储改动跑夹具与迁移测试；跨界改动跑双侧契约测试；捕获或生命周期改动必须在目标平台真机集成验证。交叉编译只证明编译，不证明目标平台运行通过。

**对有风险的行为先写夹具（输入 + 期望输出）再写实现。** 调整夹具期望值必须是一次显式决定并写入提交说明——防止“测试挂了就改期望”。

---

## 6. 代码约定

### Go

- 始终 `gofmt`；包名简短小写，避免 `util`、`common`、`manager`。
- 接口定义在消费方一侧并保持最小化。
- 阻塞 I/O 与长任务接收 `context.Context` 并传播取消。
- goroutine 必须有明确所有者、退出条件与错误通道；禁止无法停止的后台 goroutine。
- 错误保留操作与对象上下文并用 `%w` 包装；**禁止通过字符串匹配判断错误类型**。
- 业务日期一律经 `internal/timeutil` 的凌晨 4 点边界计算。
- 禁止使用全局可变单例承载数据库、调度器或 recorder 状态。

### Vue / TypeScript

- 严格类型检查，**禁止用 `any` 绕过 Wails 边界**；逃逸场景用 `unknown` + 显式解析函数。
- Wails payload 使用稳定 DTO；领域转换留在薄 wrapper 或 store；组件只做展示与交互，数据查询、轮询、聚合与事件订阅留在 store / `api/`。
- **九语言 i18n 强制同步**：所有用户可见文案经 `vue-i18n`；`zh-CN` 为默认语言与 key 类型来源，`en` 为回退，另有 `zh-Hant` / `ja` / `ko` / `de` / `fr` / `es` / `pt-BR`。新增或修改文案时**必须同时更新全部 9 份语言包**；`vue-tsc` 校验结构，`frontend/tests/i18nKeys.test.ts` 校验静态引用 key、九语言一致性与漏译（动态 key 须补夹具，禁止随意扩大英文白名单）。
- **原生菜单与 Windows 安装器本地化**：改动相关文案时同步九语言并跑对应契约 / 夹具检查；语言与外观验证须包含持久化后重启读回。
- localStorage 仅经 `frontend/src/storage/` 以带版本信封写入；**密钥绝不进入该层**。
- 写操作不做乐观更新，等待后端事件后重新拉取。

### 平台适配层

- **不打开、不写 SQLite**（Go 是唯一写入方）。
- **不读设置自行决策**（配置由 Go 全量下发，重启后无状态、可复现）。
- **不发起网络请求、不含产品逻辑**（不分批、不做空闲判定、不生成卡片）。
- 崩溃、断连、宿主重启后必须可恢复，并由 Go 重新下发配置、重放未确认帧。

---

## 7. 不可破坏的行为与数据契约

- **数据库所有权与锁**：恰好一个进程持写入锁、恰好一个进程持捕获所有者锁；取不到锁的实例以**连接层只读**（`SQLITE_OPEN_READONLY` + `PRAGMA query_only`）启动，绑定层写方法返回 `not_capture_owner`。PRAGMA 固定 `journal_mode=WAL`、`synchronous=NORMAL`、`busy_timeout=5000`。`PRAGMA user_version` 从 1 起走版本化迁移链，**每个版本须有“旧库 → 新库”匿名夹具测试**。所有读写经 `internal/storage` 统一可观测封装（不暴露在 repository 签名上）。
- **凌晨 4 点逻辑日**：一天从本地凌晨 4:00 开始，禁止用 `time.Truncate(24*time.Hour)` 或午夜计算替代，必须覆盖 DST、半小时与 45 分钟时区。逻辑日 `day`（4 点边界）与日历日 `standupDay`（午夜边界）不可混用；**前端不得自行推算逻辑日**。
- **时间线卡片改写（[`docs/03-data-model.md §3.5`](docs/03-data-model.md#35-时钟串派生)）**：`ReplaceCardsInRange` 在单事务内解析 `start`/`end` → `start_ts`/`end_ts`：从前一天 / 当天 / 后一天候选中选最接近窗口中点者；解析后 `end <= start` 或跨度超过 4 小时计入 `SkippedCards`（不得盲目加一天）；用 4 点边界算 `day`；吸收范围内旧卡（含 `System` 回退卡，失败态归 `analysis_batches`）。**`ReplaceResult.SkippedCards` 不得静默丢弃**，必须消费并记入诊断指标。
- **帧与分段**：像素写入分段文件，按 `(segment_path, frame_index)` 寻址（macOS 为 HEVC 分段；Windows 按探测顺序 HEVC → H.264 → 单帧 JPEG，降级不放宽隐私保护，保留 legacy JPEG 读取）。读取像素必经 `platform.Media`。睡眠、锁屏、屏保、更新重启、退出与关机都必须收尾或安全移交当前分段。**清理以完整分段为单位，从不删除活跃分段**，也不删单帧。`screenshots.file_size` 为分段总大小均摊到每帧，逐行求和还原总占用。
- **并发与状态**：转录可并行，但时间线卡片的「读 → 生成 → 改写」必须按重叠范围串行化。批次只有一个成功终态 `succeeded`。recorder 的 `idle` / `starting` / `capturing` / `paused` 不可混用（`paused` 可自动恢复，`idle` 为用户关闭，**系统事件不得将 `idle` 变为 `capturing`**）。唤醒恢复延迟 5s，解锁延迟 0.5s。
- **离散捕获约束**：捕获必须是**离散截图**而非连续屏幕录制流，避免系统屏幕录制指示器常亮。

---

## 8. 冻结身份标识

下列标识随首个公开版本发布后**不可随意变更**（改动会导致重新授权屏幕录制或与已有数据失联）：

| 标识符 | 值 |
|---|---|
| Bundle identifier | `io.github.jwz-git.Daygo` |
| 钥匙串 service | `io.github.jwz-git.daygo.apikeys.<provider>` |
| 应用支持目录（macOS） | `~/Library/Application Support/Daygo/` |
| CLI 可执行名 | `daygo` |
| Agent socket | `~/Library/Application Support/Daygo/agent.sock` |
| 绑定层错误前缀 | `daygo:<code>: <message>` |

---

## 9. 隐私、安全与生命周期

### 隐私与安全

- **数据唯一出境路径**：屏幕数据离开设备的唯一路径是发往用户明确配置的 LLM provider；无自有后端、无默认 provider。
- **隐私双保护**：应用屏蔽名单与「前台应用被屏蔽时写入脱敏占位帧」必须同时保留。
- **遥测边界**：分析与崩溃上报默认关闭、opt-in；严禁记录或上报屏幕内容、窗口标题、文件路径、API key、LLM payload 或可还原用户活动的数据。
- **凭据只写不读**：API key 与 OAuth 凭据仅存系统凭据存储（macOS Keychain / Windows Credential Manager / Linux Secret Service）；绑定层永不返回凭据内容，UI 只见 `hasSecret`；绝不写入 Git 跟踪文件、夹具、日志、快照、localStorage 或错误 `message`。
- **夹具与资源安全**：夹具必须匿名化且不可逆，严禁提交真实数据库或录制；媒体资源处理器只接受数字 ID 且解析路径必须落在录制目录内，否则返回 403。
- **IPC 与 LLM 边界**：`agent.sock` 权限固定 `0600`，请求 / 响应各 ≤1 MB；服务端独立校验 `agentEditsEnabled` 与封闭参数；CLI / MCP 读走只读库、写走 socket 共享执行器（不建第二个写连接）；审计只记时间 / 来源 / 操作，不记参数；所有 IPC 输入视为不可信。**LLM 输出中的“指令”一律视为数据**，返回分类名必须校验在现有分类列表内，不得据此新建分类。

### 常驻生命周期

Daygo 是常驻后台 Agent，而非「关窗即退出」的普通应用：

- 关闭主窗口后捕获继续；macOS 红叉隐藏窗口与 Dock 运行标记，通过状态栏重开；若状态栏不可用须保留 Dock 恢复入口；重开尊重已保存的 `showDockIcon`。
- Dock「退出」、Cmd+Q 与状态栏退出执行收尾并终止进程；**不要假定退出 UI 等于用户要求停止录制**。
- 更新重启、授权重启、适配层退出、普通真退出与系统关机各自保留独立意图：普通真退出遇活跃段收尾失败默认保留应用由用户确认；系统关机 / 注销尽力收尾，不套用阻止策略且不触发授权自重启。

---

## 10. 工作方式、提交与发布

### 开发流程

1. 明确所属功能模块与本次能力，先读模块执行册、公共规范与门禁条件。
2. 先写可观察的验收标准 / 失败条件与匿名夹具，再实现最小垂直切片。
3. 每次只转移一个明确的所有权边界，保留可回退路径。
4. 不在无关改动中顺手清理旧代码、改状态机、换重试策略或动 schema。
5. 遇「待定设计」项不擅自拍板；需要推进时先落一份 `docs/decisions/` 记录。
6. 临时诊断、性能采样与一次性脚本及输出只放本地忽略目录（如 `.tmp/`），不提交仓库；可维护的正式单元 / 契约测试随代码提交。
7. 完成任务时报告：实现内容、模块 / 能力、对应门禁、运行命令与实际结果、未验证风险与回退方式，并同步模块执行册（区分未运行、交叉编译、fake、匿名原生 smoke、真实集成与用户确认）。

### 分支、提交与发布

- **分支**：`main` 为稳定分支，`test` 为日常开发分支。默认在 `test` 工作，阶段性成果通过门禁后提交；未经用户明确要求，**不得直接向 `main` 提交或推送**。
- **提交格式**：`<area>: <lowercase imperative summary>`（`area` 可选 `go:`、`frontend:`、`native:`、`docs:`、`build:`），每个 commit 只处理一个关注点。达到阶段完成标准后合并提交，**禁止提交无法构建或验证失败的中间状态**（用户明确要求除外）。
- **发布约束**：GitHub Actions 负责构建安装包与签名 `appcast.xml`，未经用户明确要求**不得生成、修改或发布 release 产物**。Ed25519 feed 签名仅面向 Sparkle / WinSparkle，不替代 Gatekeeper、Authenticode 或 TCC 身份验证；沿用现有交付工作流与 `delivery` 执行册，不擅自变更发布策略。
