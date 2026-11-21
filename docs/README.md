# Daygo 设计文档

> 本目录定义 Daygo 的目标规格、跨界契约、模块执行册与架构决策记录（ADR）。
> **规格不等于已实现**：当前实现进度、验收证据与已知缺口以 [09 功能模块路线](09-roadmap.md) 为准（[§9.1 模块总表](09-roadmap.md#91-模块总表) · [§9.1.1 验收记录与证据边界](09-roadmap.md#911-本轮验收记录与证据边界) · [§9.1.2 当前不足与改进清单](09-roadmap.md#912-当前不足与改进清单) · [§9.8 待定设计清单](09-roadmap.md#98-待定设计清单)）。
> 文档与代码冲突时，**以当前代码和可复现实验为准，并在同一 commit 内同步修正文档**。

---

## 文档索引

| # | 文档 | 覆盖内容 |
|---|---|---|
| 01 | [产品需求](01-product-requirements.md) | 目标用户、核心问题、v1 交付范围与非目标 |
| 02 | [架构设计](02-architecture.md) | 分层架构、依赖方向与目录边界 |
| 03 | [数据模型](03-data-model.md) | SQLite 表结构、4 点逻辑日、分段存储与设置项 |
| 04 | [数据流](04-data-flow.md) | 捕获 → 存储 → 分析 → 呈现的端到端流程与常量 |
| 05 | [接口契约](05-interface-contract.md) | Wails 绑定、DTO、事件、错误码与平台端口 |
| 06 | [原生集成](06-native-integration.md) | 平台能力需求、Go 端口契约与各平台实现状态 |
| 07 | [隐私与安全](07-privacy-security.md) | 数据出境边界、凭据存储、遥测与 IPC 安全模型 |
| 08 | [测试策略](08-testing-strategy.md) | 各层验证方式、匿名夹具与提交门禁 |
| 09 | [功能模块路线](09-roadmap.md) | 模块现状、能力依赖、验收门禁、证据边界与待定项 |
| 10 | [风险登记](10-risks.md) | 已识别风险、触发条件与缓解措施 |

---

## 功能模块执行册

开始任务前，先在 [模块总表](09-roadmap.md#91-模块总表) 确认目标能力与门禁，再阅读对应执行册和公共规格。新增执行册沿用 [执行册模板](modules/_template.md)。

| 模块 | 职责与用户结果 | 交付说明 |
|---|---|---|
| [recording](modules/recording.md) | 常驻录制、授权、暂停、系统事件、隐私屏蔽与分段恢复 | v1 核心（部分实现） |
| [providers](modules/providers.md) | AI 服务配置（API Key / ChatGPT 订阅）、密钥、回退链与模型试用 | v1 核心（部分实现） |
| [timeline](modules/timeline.md) | 自动时间线、分类、原始帧回放、编辑、重处理与卡片合并 | v1 核心（部分实现） |
| [daily](modules/daily.md) | 每日工作流、站会摘要、日记、目标与日记提醒通知 | v1 核心（部分实现） |
| [weekly](modules/weekly.md) | 周工作流、热力图、分类占比、时间流向与 Token 图表 | v1 核心（部分实现） |
| [data](modules/data.md) | 数据库基础、实例锁、备份恢复、磁盘上限清理与诊断 | v1 核心（部分实现） |
| [preferences](modules/preferences.md) | 外观、九语言 i18n、通用设置容器与前端基础接入 | v1 核心（部分实现） |
| [delivery](modules/delivery.md) | 签名身份、首次引导、安装包与 Sparkle / WinSparkle 自动更新 | v1 核心（部分实现） |
| [plan](modules/plan.md) | 今日计划时间块、完成标记与开始 / 分心系统通知 | 部分实现（基础已合入 `test`） |
| [agent](modules/agent.md) | CLI 查询、`agent.sock` 受控写入与 stdio MCP | 部分实现（v1 不交付） |
| [chat](modules/chat.md) | 应用内自然语言问答与沙箱内受控编辑 | 部分实现（v1 不交付） |
| [play](modules/play.md) | 由当日 / 当周卡片生成的拼岛休闲小游戏 | 仅策划案，未开始 |

---

## 决策记录（ADR）

`docs/decisions/<module>-<topic>.md` 记录候选方案、实验证据、已定边界与回退路径；无验证证据不标「已决定」。

### 宿主、捕获与存储

| 决策 | 状态 | 摘要 |
|---|---|---|
| [Wails v3 GA 宿主迁移](decisions/wails-v3-migration.md) | 已落盘 | Wails v3.1.0 + Go 1.26.10 宿主迁移边界、验证与回退 |
| [常驻宿主退出模型](decisions/lifecycle-quit-model.md) | 已决定并实现 | 关窗留后台、真退出、更新 / 关机意图、Dock 偏好与状态栏恢复 |
| [屏幕截屏：单次调用契约与原生 ABI](decisions/recording-screen-capture.md) | 契约已冻结 | 跨平台原始规格、Go `Capture` 契约、C ABI v1 与真机门禁 |
| [屏幕截屏 v2：macOS 实现与上层调用](decisions/recording-screen-capture-v2.md) | 有限实现 | Swift / cgo 路径、调用不变量、错误处理与 recorder 接入边界 |
| [屏幕录制授权：弹框 + 跳设置 + 提示重启](decisions/recording-screen-recording-permission.md) | 有限实现 | preflight / request / 跳设置 ABI、前端闸门与重启生效 |
| [跨平台应用身份解析与 ABI](decisions/recording-application-picker.md) | 有限实现 | `.app` / `.exe` 选择器、独立 Go 端口、ABI 2.1 图标与平台 ID 回查 |
| [屏幕截屏（Windows）：DXGI/WGC 实现与限制](decisions/recording-screen-capture-windows.md) | 已实现并确认验收 | DXGI/WGC、应用身份、系统事件 / 通知区、安装升级与证据边界 |
| [屏幕截屏（Linux）：候选与决策进度](decisions/recording-screen-capture-linux.md) | 决策进行中 | X11 vs Wayland、离散截图约束、分发形态与 LC 门禁 |
| [开机自启](decisions/recording-launch-at-login.md) | 已决定并实现 | macOS `SMAppService`、Windows Run 键与配置意图边界 |
| [图片存储流水线](decisions/recording-image-storage.md) | 公共边界保留 | 像素 / SQLite 边界与整段清理；staging 方案已由分段方案取代 |
| [HEVC 帧分段](decisions/recording-frame-segments-hevc.md) | 已决定并有限实现 | macOS 直接追加、帧解码、迁移、按段清理与验收边界 |
| [Windows 分段编码：探测与降级](decisions/recording-windows-segment-codec.md) | 已决定并实现 | HEVC → H.264 → 单帧 JPEG 探测、首帧降级与单帧段契约 |
| [Windows 录制目录迁移](decisions/recording-directory-windows.md) | 已实现并确认验收 | 迁移 `recordings/` 到所选空目录，DB / 锁 / 密钥留在应用数据目录 |

### AI 服务与凭据

| 决策 | 状态 | 摘要 |
|---|---|---|
| [Gemini 原生协议与原始视频分段](decisions/providers-gemini-video.md) | 有限实现 | 合格的完整已收尾 MP4 优先直传，不满足时回退截图 |
| [ChatGPT 订阅](decisions/providers-chatgpt-subscription.md) | 已决定（增量验证单列） | 官方 OSS OAuth、套餐调用、系统凭据与设置 / 引导共用入口 |
| [providers 回退链](decisions/providers-fallback-chain.md) | 已决定 | 有序路由、粘性回退与重试语义 |
| [providers 单供应商多模型](decisions/providers-multi-model.md) | 已决定 | `models` JSON 列、路由「供应商 + 模型」对与复合计数键 |
| [providers 密钥：Keychain / Credential Manager](decisions/providers-secrets-keychain.md) | 已决定 | 系统凭据存储访问、service 命名与只写不读边界 |
| [providers 密钥：Linux Secret Service](decisions/providers-secrets-linux.md) | 已落盘并确认验收 | `secret-tool` 访问 Secret Service 的取舍与降级边界 |

### 时间线、复盘与通知

| 决策 | 状态 | 摘要 |
|---|---|---|
| [时间线人工输入保留](decisions/timeline-human-card-preservation.md) | 已决定并实现 | 自动改写保留人工原 ID / 区间与显式单卡替换边界 |
| [时间线短卡合并](decisions/timeline-short-single-card-merge.md) | 已决定并实现 | 首批与持续窗口的 15 分钟下限及合并边界 |
| [时间线输出预算与 token 截断](decisions/timeline-output-budget.md) | 已决定 | 协议默认输出额度与明确截断不原样重试 |
| [网站图标获取](decisions/timeline-favicon-fetch.md) | 已决定并实现 | 本地缓存、受限网络请求与载荷预算 |
| [weekly 周边界：周一起始](decisions/weekly-boundary-monday.md) | 已决定 | 周一 4 点对齐的统计语义与夹具 |
| [日记提醒通知](decisions/notifications-journal-reminder.md) | 已决定并实现 | Go 重复调度、一次性原生投递端口与文案下发 |
| [Windows 日记 / 计划通知](decisions/notifications-windows-toast.md) | 已决定并实现 | C++/WinRT、当前用户 AUMID / COM、一次性排程与取消 |

### 数据、分发与扩展交互

| 决策 | 状态 | 摘要 |
|---|---|---|
| [data 实例锁：flock / LockFileEx 锁文件](decisions/data-locking.md) | 已决定 | 写入锁与捕获所有者锁的跨平台实现与回退 |
| [data 备份保留份数：7 份](decisions/data-backup-retention.md) | 已决定 | `VACUUM INTO` 轮换策略与边界 |
| [data 损坏恢复](decisions/data-corruption-recovery.md) | 已决定 | 备份识别、恢复流程与只读降级 |
| [macOS 签名身份：本地自签名稳定证书](decisions/delivery-macos-signing-identity.md) | 有限实现 | TCC 绑定 DR、ad-hoc 失效根因与三条签名路径 |
| [macOS 自动更新](decisions/delivery-auto-update.md) | 已决定并实现 | Sparkle、Ed25519 appcast、安装前收尾与身份门禁 |
| [Windows 自动更新](decisions/delivery-auto-update-windows.md) | 已决定并实现 | WinSparkle、NSIS 与双平台共用 appcast |
| [chat 会话模型](decisions/chat-session-model.md) | 已决定 | 多会话、原子消息与会话级 provider 选择 |
| [agent MCP 传输：stdio 子进程](decisions/agent-mcp-transport.md) | 已决定（基础已落盘） | `daygo mcp` 子进程、读走只读库 / 写走 `agent.sock` 与来源审计 |
| [play 玩法选型：方块拼岛](decisions/play-game-style.md) | 提议，待确认 | 六类候选对比、软上限与无 streak 约束 |

> 链接、小节锚点与孤立文档由 [`scripts/check-docs.py`](../scripts/check-docs.py)（随 `scripts/gate.sh`）机械校验；文档与代码一致性由提交者在同一次 commit 内保证。工程规范见根目录 [AGENTS.md](../AGENTS.md)。

---

## 1. 执行摘要

### 1.1 Daygo 是什么

Daygo 是一个面向 macOS 与 Windows（并规划支持 Linux）的常驻后台 Agent：按间隔离散截取当前系统主显示器，分批交给用户配置的 LLM 分析，自动整理成时间线、每日站会摘要与每周复盘。

三条产品前提决定整体架构：

1. **本地优先**：录制、数据库与派生结果默认只存于用户本机；屏幕数据离开设备的唯一路径是发往用户明确配置的 AI provider。
2. **零手工记录**：无需手动启停计时器或打标签，时间线由后台捕获与分批分析生成。
3. **常驻而非普通窗口应用**：关闭主窗口后捕获必须继续，对宿主框架与生命周期提出核心约束（见 [风险 C-1](10-risks.md#c-1宿主无法承载后台-agent)）。

### 1.2 技术栈与分层

```text
Vue 3 + TypeScript
        ↓ 生成的 Wails v3 绑定
internal/app                仅此层知道 Wails
        ↓
Go services                 analysis / ai / insight / chat / recorder
        ↓
Go foundation               storage / settings / domain / timeutil
        ↓ 端口接口
internal/platform           Capture / Media / System / Secrets / Updater / Notifications
        ↓
平台适配层                   唯一接触系统能力的地方（darwin / windows / linux / fake）
```

Go 拥有全部可移植业务逻辑与唯一的 SQLite 写连接；平台适配层严格实现 [06](06-native-integration.md) 冻结的端口契约。

### 1.3 v1 范围

| 能力 | v1 交付 | 说明 |
|---|:--:|---|
| 自动时间线（日 / 周视图、帧回放、编辑与重处理） | ✅ | 主界面 |
| 每日回顾（工作流、站会摘要、日记、目标与提醒） | ✅ | |
| 每周回顾（周图表、专注分析与分类占比） | ✅ | |
| 设置（外观、九语言、AI Provider、存储、隐私与更新） | ✅ | |
| 计划时间块与开始 / 分心提醒 | ✅ | 随 `plan` 接入时间线检查器 |
| 自然语言问答（Chat） | ❌ | 已部分实现，v1 明确不交付 |
| 导出 Markdown | ❌ | 推迟到 v1.1 |
| CLI / MCP / `agent.sock` 写入通道 | ❌ | 基础实现已落盘（[05 §5.9](05-interface-contract.md#59-b6对外接口推迟到-v11)），交付推迟到后续版本 |

推迟项的接口形状提前定义在 [05](05-interface-contract.md)，确保后续补做时不破坏 v1 数据模型。

### 1.4 关键设计决策

| 决策项 | 选择 | 原因 |
|---|---|---|
| 捕获方式 | **离散截图**，非连续录制流 | 避免系统屏幕录制指示器常亮，见 [06 §6.2.1](06-native-integration.md#621-关于捕获方式的一条产品约束) |
| 捕获目标 | 调用时的**系统主显示器** | 保持截图原语单次、无状态；多屏仅记录主屏 |
| 逻辑日边界 | **本地凌晨 4 点** | 深夜工作归属前一天，见 [03 §3.2](03-data-model.md#32-凌晨-4-点逻辑日) |
| SQLite 驱动 | `modernc.org/sqlite`（纯 Go） | 保障 Go Core 在 `CGO_ENABLED=0` 与 Linux 下可构建可测试 |
| Schema 所有权 | `internal/storage` 为唯一写入方 | 单一事务边界与迁移链 |
| 设置持久化 | SQLite `app_settings` 表 | 设置与业务数据可在同一事务内更新 |
| 凭据存储 | 系统凭据存储，只写不读 | 绑定层永不返回密钥内容，UI 仅知 `hasSecret` |
| 帧像素传输 | HTTP 资源而非 JSON | 原生获得流式传输、缓存与懒加载 |
| 多语言策略 | `zh-CN` 为默认与类型源，`en` 回退，共 9 种语言 | `vue-tsc` 与单测在编译/测试期拦截缺键与漏译 |

### 1.5 身份标识

下列值随首个公开版本发布后**不可随意更改**——改动会让用户重新授权屏幕录制并与已有数据失联：

| 标识符 | 值 |
|---|---|
| Bundle identifier | `io.github.jwz-git.Daygo` |
| 钥匙串 service | `io.github.jwz-git.daygo.apikeys.<provider>` |
| 应用支持目录（macOS） | `~/Library/Application Support/Daygo/` |
| CLI 可执行名 | `daygo` |
| Agent socket | `~/Library/Application Support/Daygo/agent.sock` |
| 绑定层错误前缀 | `daygo:<code>: <message>` |
