# providers — AI 接入

## 用户结果与范围

用户可为一个 Provider 配置名称、协议、地址、**多个模型**与密钥，编排有序回退链
（条目为「供应商 + 模型」对，主 + 多备用），获取模型列表，逐模型测试连接，重启后仍能
判断密钥是否已配置。负责 U7、F-S1–4；支持 openai（Chat Completions）、openai_responses、
anthropic 三种协议。
本模块交付可供 timeline / daily / chat 消费的客户端，不拥有分批、卡片、摘要内容或批次状态。

公共依据：[05 Provider 绑定](../05-interface-contract.md#provider)、
[05 服务契约](../05-interface-contract.md#564-ai--analysis)、
[07 密钥](../07-privacy-security.md#73-密钥)、[04 重试](../04-data-flow.md#433-重试与回退)、
[decisions/providers-fallback-chain](../decisions/providers-fallback-chain.md)、
[decisions/providers-multi-model](../decisions/providers-multi-model.md)、
[decisions/providers-secrets-keychain](../decisions/providers-secrets-keychain.md)。

## 当前状态与证据

> **验收状态**：已实现能力于 2026-09-22 经用户确认已验收；无逐项运行记录。未实现能力见 [09 §9.1](../09-roadmap.md#91-模块总表)。

实现进度：部分实现。Go 侧已落地：三协议客户端、重试 / 回退链（`ai.Chain`，循环降级）、
连接探针、迁移 v4 的 `providers` 表与 `ProviderRepo`、Secrets 端口（macOS 经
`security` CLI、Windows 经 Credential Manager、Linux 经 Secret Service / `secret-tool`，以及 fake）、
Provider CRUD / 路由链 / 密钥 / `TestProvider(id, model)` 绑定
（主要在 `internal/app/providers.go`），以及分置于 `providers_models.go` 和 `provider_probe.go` 的
模型列表与草稿连接探针。单供应商多模型已落地（v17：`providers.model` → `models` JSON 数组，
上限 20）：路由链条目改为「供应商 + 模型」对，`ai.Chain` 按 `providerID + "\x1f" + model`
复合键独立计数，钥匙串与 `llm_calls.provider_id` 仍用裸供应商 ID
（decisions/providers-multi-model）。设置层 `providers.routing` 为有序对链，兼容旧的裸 id
数组与 `{primary,secondary}` 形状（读取时折叠）。
前端 store 已以 Go 绑定为权威来源，写后重拉；表单支持多模型增删与逐模型测试，回退链编辑器
以单一有序列表编排「供应商 + 模型」对；旧 localStorage 记录只在后端列表为空时做一次性
无密钥迁移（单模型折为一元列表），成功后删除。`hasSecret` 仅由后端检查钥匙串后返回。
模型列表查询与每 Provider 图片上限（v11，per-provider、与模型无关）也已接入。真实网络集成、
完整 Wails 重启闭环与升级身份验证经用户确认已验收。

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
llm.outputLanguage、llm.recognitionEnhancementEnabled 的字段规则和设置交互；回退链跨回合
行为由消费方（chat / 分析流水线）集成验证。

识别增强（`ai.GenerateRecognition`，由 `llm.recognitionEnhancementEnabled` 控制，默认关）：
开启时识别用途的每张图片在内存中切成 2×2 四张重叠分片（每片约半幅加交叉覆盖），四片
之后附上未改动的原图一起发送，分片仅存在于单次请求生命周期、返回后清零，不落盘不入库；
关闭时请求原样透传。timeline 转录阶段已通过 `GenerateRecognition` 接入该开关；卡片生成仍走
普通文本请求。分析分组同时按回退链中最小图片上限限制请求规模。

## 实验与失败条件

| 实验 | 输入与操作 | 预期结果 | 失败条件 / 证据 |
|---|---|---|---|
| 配置往返 | 匿名配置、链重复 / 不存在、空密钥与显式删除 | 规范化后落库，空密钥保持不变，删除只能显式触发 | 重启丢配置、错误路由、误清密钥失败 |
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
无密钥泄漏；未配置时不发请求，连接测试经绑定真实发起且结果如实展示（建议性，不阻塞
保存）。fake 不证明外部服务或
系统钥匙串可用。
Secrets / 身份阻塞只限制真实密钥功能；HTTP fixture 与消费者开发可继续。

待决：钥匙串方式由 providers 工程在原生实现前决定；重试用户体验由 timeline 产品负责、
providers 协作，在策略 / UI 接入前统一，见 09 §9.8。
回退：禁用未验证调用路径、恢复原 store 接入，保留旧无密钥记录与新库；
不把密钥退回 localStorage，不在回退时删除用户已有钥匙串条目。

## 验证记录

- **2026-09-23 回归修复**：默认 Anthropic 端点及草稿模型列表的端点规范化由 Go 夹具验证；`./scripts/gate.sh` 通过。真实 Provider 网络调用未在本次重跑。

- **Go 与存储（2026-09-10—20）**：`go test ./internal/ai/... ./internal/app/...`、`go vet`、`CGO_ENABLED=0 go build ./...` 及匿名 TLS 夹具覆盖三协议、连接探针、重试 / 回退、错误脱敏、路由与多模型迁移。macOS 钥匙串有一次真实 smoke；Windows 与 Linux 适配器已落盘。
- **前端**：typecheck、构建和匿名配置交互覆盖 Provider 表单、模型列表、逐模型连接测试与有序回退链。
- **真实闭环**：真实 Provider 网络、Wails 重启与升级身份由用户于 2026-09-22 确认验收，未附逐项运行记录。密钥不进入绑定、日志或 localStorage 的约束仍适用。
