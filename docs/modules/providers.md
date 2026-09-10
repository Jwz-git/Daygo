# providers — AI 接入

## 用户结果与范围

用户可配置 Provider 的名称、协议、地址、模型与密钥，选择主 / 备用服务，测试连接，
重启后仍能判断密钥是否已配置。负责 U7、F-S1–4；支持 openai（Chat Completions）、
openai_responses、anthropic 三种协议。
本模块交付可供 timeline / daily 消费的客户端，不拥有分批、卡片、摘要内容或批次状态。

公共依据：[05 Provider 绑定](../05-interface-contract.md#provider)、
[05 服务契约](../05-interface-contract.md#564-ai--analysis)、
[07 密钥](../07-privacy-security.md#73-密钥)、[04 重试](../04-data-flow.md#433-重试与回退)。

## 当前状态与证据

实现进度：部分实现。前端类型检查通过；`internal/ai` 协议客户端、重试 / 回退 / 取消、
连接探针与 factory 已落地并通过匿名 TLS fixture（见验证记录）。Secrets、Provider
repository、真实网络集成与 Wails 绑定未验收。
[store](../../frontend/src/stores/providers.ts) 与
[设置界面](../../frontend/src/views/Settings/ProvidersSection.vue) 已存在，
无密钥配置存 localStorage；密钥仅驻留内存。数据库、Secrets fake / 原生
及 Provider Go 绑定尚未实现。当前 hasSecret 不证明钥匙串持久化。

## 能力与跨层职责

| 输入 | 可先推进 | 真实接入条件 |
|---|---|---|
| data: db-core；preferences: settings-access | 匿名配置、路由和 repository fixture | Provider 表与路由落库，迁移 / 事务测试 |
| Secrets 端口；delivery 身份证据 | 内存 fake 验证写入 / 删除 / 缺失 / 失败 | 原生钥匙串及同签名重启 / 升级验证 |
| ui-bridge 与用户配置的服务 | DTO、HTTP 测试服务器、错误 / 重试夹具 | G-host、生成绑定；实际网络调用前用户明确配置目的地 |

输出 provider-client，包含文本 / 内存图片输入、JSON Schema 结构化输出、三种原生协议适配
（openai / openai_responses / anthropic，经 `internal/ai` factory 统一构造）、路由、取消、
重试与回退装饰器，以及内嵌匿名图片的连接探针（`ai.TestConnection`：固定文本 + PNG +
严格 schema，单次调用，验证文本 / 图片 / 结构化输出三种能力）。图片仅接受
JPEG / PNG / WebP，最多 20 张、单张 5 MiB、原始总量 20 MiB；调用记录只存 attempt 元数据，
不存 endpoint、正文、图片、密钥或费用。
协议客户端归 internal/ai；上层任务通过消费者接口调用，不导入另一服务的内部实现。
providers repository 在 internal/storage；Secrets.Get 只供 Go 客户端取密钥，
任何绑定均不返回密钥。settings-access 由 preferences 维护，本模块拥有 providers.routing、
llm.outputLanguage 的字段规则和设置交互；批次内粘性由 timeline 集成验证。

## 实验与失败条件

| 实验 | 输入与操作 | 预期结果 | 失败条件 / 证据 |
|---|---|---|---|
| 配置往返 | 匿名配置、主备相同 / 不存在、空密钥与显式删除 | 规范化后落库，空密钥保持不变，删除只能显式触发 | 重启丢配置、错误路由、误清密钥失败 |
| 密钥边界 | 测试专用临时密钥写入 / 删除，重启并检查 hasSecret | 值只在钥匙串 / Go 客户端；UI 仅布尔值 | DTO、日志、错误、localStorage 出现密钥立即阻塞 |
| 协议 / 重试 | 匿名 HTTP 服务器返回成功、限流、超时、错误体；取消任务 | 请求符合各协议，错误脱敏，重试有界并传播取消 | 重试失控、请求目的地错误、后台任务无法结束失败 |
| 真实连接 | 用户指定 Provider 与模型，显式运行 TestProvider | 一次实际测试调用，结果及耗时可见，不回显 payload | fake 成功不可替代此项；网络失败不谎报可用 |
| 身份 | 与 delivery 运行重启 / 同签名升级矩阵 | 密钥归属与读取稳定，授权行为符合记录 | 仅编译成功不解除 G-native |

## 实现切片与集成

1. 以固定配置、文本 / 匿名图片、schema、响应和错误夹具定义协议 / 路由 / 密钥行为；
   补 Secrets fake 契约。
2. 实现三种协议客户端、结构化输出、本地校验、装饰器、连接探针与取消，TLS HTTP fixture
   证明请求及错误路径；输出解析所需样本使用人工匿名夹具交给 timeline，避免两套重试规则。
3. 接入 Provider repository、settings-access 和真实 Secrets，完成同签名身份实验；
   提供 provider-client 给 timeline / daily，不等这两个模块完成。
4. 接完整 Provider 绑定、事件与 store；按功能迁移无密钥旧配置，确认持久化再切换数据源。
   旧内存密钥不批量写盘，用户通过显式保存完成钥匙串接入。
5. 验证真实连接与设置重启闭环，补空态、失败态、加载态和两种语言。

## 验收、阻塞与回退

完成要求：配置 / 路由 / 密钥全链路真实可用、三种协议夹具通过、真实配置服务连接通过、
无密钥泄漏；未配置时不发请求，测试连接成功前草稿配置不生效。fake 不证明外部服务或
系统钥匙串可用。
Secrets / 身份阻塞只限制真实密钥功能；HTTP fixture 与消费者开发可继续。

待决：钥匙串方式由 providers 工程在原生实现前决定；重试用户体验由 timeline 产品负责、
providers 协作，在策略 / UI 接入前统一，见 09 §9.8。
回退：禁用未验证调用路径、恢复原 store 接入，保留旧无密钥记录与新库；
不把密钥退回 localStorage，不在回退时删除用户已有钥匙串条目。

## 验证记录

2026-09-10：前端类型检查通过，见 [基线](../09-roadmap.md#当前代码证据)。
2026-09-10：`internal/ai` 落地统一 Provider 接口、三种协议客户端（openai Chat Completions /
openai Responses / anthropic Messages）、重试 / 回退 / 取消、脱敏 attempt 观测、
JSON 提取与 schema 校验、内嵌 PNG 连接探针与 factory。`go test ./internal/ai/...`、
`go vet ./internal/ai/...`、`CGO_ENABLED=0 go build ./...` 通过，全部使用匿名 TLS fixture。
Secrets、Provider repository、Wails 绑定、真实服务连接及升级身份验证未运行；
后续记录匿名夹具、commit 与环境。
