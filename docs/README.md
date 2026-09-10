# Daygo 设计文档

> **状态：设计中。** 本目录描述目标行为与接口。已有桌面外壳、前端设置、平台端口、
> Capture fake、macOS 单次截图有限实现、部分契约测试 / 绑定与时间函数；数据库、recorder 和
> 业务闭环尚未实现。当前状态与证据见 [09 §9.1](09-roadmap.md#91-模块总表)，不要把目标目录、
> 命令或行为描述成现状。
>
> macOS 单次截图已有实验实现，但真实隐私矩阵、宿主、系统授权流程、钥匙串、状态栏、自动更新
> 和视频编解码仍是 **待定设计**。汇总见
> [09 §9.8 待定设计清单](09-roadmap.md#98-待定设计清单)。

## 文档索引

| # | 文档 | 回答的问题 |
|---|------|-----------|
| 01 | [产品需求](01-product-requirements.md) | 做给谁、解决什么问题、v1 范围与非目标 |
| 02 | [架构设计](02-architecture.md) | 有哪些模块、依赖方向、目录结构 |
| 03 | [数据模型](03-data-model.md) | 表结构、逻辑日、帧与分段、设置项 |
| 04 | [数据流](04-data-flow.md) | 捕获 → 存储 → 分析 → 呈现的端到端流程与常量 |
| 05 | [接口契约](05-interface-contract.md) | 绑定方法、DTO、事件、错误码、版本规则 |
| 06 | [原生集成](06-native-integration.md) | 需要平台提供哪些能力、Go 侧端口形状 |
| 07 | [隐私与安全](07-privacy-security.md) | 数据边界、密钥、遥测、IPC 安全模型 |
| 08 | [测试策略](08-testing-strategy.md) | 各层验证方式与门禁 |
| 09 | [功能模块路线](09-roadmap.md) | 模块状态、能力接入、门禁、检查点与待定设计 |
| 10 | [风险登记](10-risks.md) | 已识别风险与缓解措施 |

## 开发入口

先在 [模块总表](09-roadmap.md#91-模块总表) 选择用户功能，再读该执行册和关联公共规范。
模块可并行开工，真实接入取决于具体能力；fake 验证与真实闭环验收分别记录。

| 模块执行册 | 用户结果 |
|---|---|
| [recording](modules/recording.md) | 常驻录制与隐私 |
| [providers](modules/providers.md) | AI 配置与连接 |
| [timeline](modules/timeline.md) | 自动时间线 |
| [daily](modules/daily.md) | 每日摘要、日记与目标 |
| [weekly](modules/weekly.md) | 每周复盘 |
| [data](modules/data.md) | 数据维护与诊断 |
| [preferences](modules/preferences.md) | 外观、语言与通用设置 |
| [delivery](modules/delivery.md) | 安装与安全更新 |

新增执行册沿用 [模板](modules/_template.md)。这些功能边界不改变 02 的技术分层。

原生能力的实验规格与子决策：

- [屏幕截屏 v2：macOS 实现与上层调用](decisions/recording-screen-capture-v2.md)——当前真实
  Swift / cgo 路径、调用不变量、错误处理、调试与 recorder 接入边界。
- [屏幕截屏：单次调用契约与原生 ABI](decisions/recording-screen-capture.md)——跨平台原始规格、
  Go `Capture` 契约、C ABI v1 与真实机器门禁。

工程约定（构建命令、代码风格、提交格式、依赖规则）见仓库根目录的
[AGENTS.md](../AGENTS.md)。文档与代码冲突时，**以可复现实验和当前代码为准，并在同一个
commit 内修正文档**。

---

## 1. 执行摘要

### 1.1 Daygo 是什么

Daygo 是一个 macOS 常驻后台 Agent。它按固定间隔截取当前活跃显示器，把帧按时间分批
交给用户配置的 LLM 理解，再把结果整理成可检索的每日时间线、站会摘要和每周复盘。

三条产品前提决定了整个架构：

1. **本地优先。** 录制、数据库和派生结果默认只存在于用户的 Mac 上。屏幕数据离开设备
   的唯一路径，是发送给用户明确配置的 AI provider。
2. **零手工输入。** 用户不启停计时器、不打标签。时间线完全由捕获与分析得到。
3. **常驻，不是普通桌面应用。** 关闭窗口后捕获必须继续。这一条对宿主框架的要求，
   是全项目风险最高的技术前提（[风险 C-1](10-risks.md#c-1宿主无法承载后台-agent)）。

### 1.2 技术栈与分层

```text
Vue 3 + TypeScript
        ↓ 生成的 Wails 绑定
internal/app                仅此层知道 Wails
        ↓
Go services                 analysis / ai / insight / chat
        ↓
Go foundation               storage / settings / domain / timeutil
        ↓ 接口
internal/platform           端口：Capture / Media / System / Secrets / Updater
        ↓ 实现待定设计
平台适配层                   唯一接触 macOS 能力的地方
```

Go 拥有全部可移植业务逻辑，并且是 SQLite 的唯一写入方。平台适配层的实现形态
（进程内桥接、独立辅助进程、原生宿主）**待定设计**，但它必须满足 [06](06-native-integration.md)
定义的端口契约——契约先冻结，实现后选择。

### 1.3 v1 范围

| 交付 | v1 | 说明 |
|------|:--:|------|
| 时间线（按日） | ✅ | 主界面 |
| 每日摘要 / 站会 | ✅ | |
| 每周复盘 | ✅ | 合计与分类占比；自定义图表推迟 |
| 设置（外观、语言、Provider） | ✅ | 已部分落盘 |
| 设置（存储、隐私、账户） | ✅ | 需要 Go 绑定 |
| 自然语言问答（Chat） | ❌ | 推迟 |
| 导出 Markdown | ❌ | 推迟到 v1.1 |
| CLI / MCP / Agent 写入通道 | ❌ | 接口已定义（[05 §5.9](05-interface-contract.md#59-b6对外接口推迟到-v11)），实现推迟 |

推迟项的接口形状仍然写进 [05](05-interface-contract.md)，这样 v1 的数据模型不会在
补做它们时被迫改动。

### 1.4 关键设计决策

| 决策 | 选择 | 原因 |
|------|------|------|
| 逻辑日边界 | **凌晨 4 点**，不是午夜 | 深夜工作属于"前一天"是用户的真实心智；见 [03 §3.2](03-data-model.md#32-凌晨-4-点逻辑日) |
| SQLite 驱动 | `modernc.org/sqlite`（纯 Go） | 让 Go Core 在无 macOS 环境可构建可测试，这是整套测试策略的前提 |
| CGO | Go Core 默认关闭 | 同上。平台适配层是否需要 CGO 取决于其实现形态（待定设计） |
| Schema 所有权 | Go Core 是唯一写入方 | 单一 schema owner、单一事务边界 |
| 设置存放 | SQLite 的 `app_settings` 表 | 设置与受其影响的数据可以在同一事务里改；不引入第二套持久化机制 |
| 密钥存放 | 系统钥匙串，只写不读 | 绑定层永不返回密钥内容，UI 只能看到"是否已配置" |
| 帧像素 | 不走 JSON，走 HTTP 资源 | 浏览器免费获得流式、缓存与懒加载 |
| 前端默认语言 | `zh-CN`，回退 `en` | key 结构以 zh-CN 为类型来源，缺 key 时类型检查直接失败 |

### 1.5 身份标识

下列值一旦随首个公开版本发布就**不可更改**——改动会让用户重新授权屏幕录制并与已有数据失联：

| 标识符 | 值 |
|--------|-----|
| Bundle identifier | `io.github.jwz-git.Daygo` |
| 钥匙串 service | `io.github.jwz-git.daygo.apikeys.<provider>` |
| 应用支持目录 | `~/Library/Application Support/Daygo/` |
| CLI 可执行名 | `daygo` |
| Agent socket | `~/Library/Application Support/Daygo/agent.sock` |
| 绑定层错误前缀 | `daygo:<code>: <message>` |

发布前它们仍可调整；发布后的任何变更都需要迁移方案与回滚路径。
