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

2026-10-05 配置模型 combobox 修复（基于 `a9dc930` 的 test 工作树，macOS arm64）：
单个服务的模型栏可手填或下拉选择；「获取模型」只更新候选，不自动将所有返回模型写入草稿。
修复原先全表单监听导致获取后候选立即清空，以及聚焦清空显示值、延迟 blur 导致立即保存旧值的问题。
模型选择 / 增删、服务名称与图片数编辑保留候选；协议 / 地址 / 密钥 / User-Agent 变化清空候选，
丢弃旧请求及卸载后的迟到结果。支持输入筛选、方向键 / Enter 选择、Escape 收起；九语言提示同步。
沿用 `ListProviderModels` 的草稿 / 已存密钥分支，地址或协议修改后不能把已存密钥发往新目的地。
夹具先于实现复现自动添加、空候选、聚焦清空与保存旧值；另覆盖空 / 失败列表可手填、来源失效、
重复请求闸门及迟到失败不覆盖新列表。真实 SFC 使用匿名 Vue host renderer / Wails 绑定夹具。
验证结果与限制在下方本次验证记录中单列；不变更后端 DTO、数据库、钥匙串或历史门禁验收。
回退：撤销本次提交即可，无数据迁移。

2026-10-03 Anthropic 请求修复（基于 `a53934f` 的 test 未提交工作树，macOS arm64）：
匿名 HTTP / TLS 夹具先于实现复现：`/v1` 与粘贴的 Messages 地址被重复追加版本路径；
转录使用的 `minimum: 0` 原样发出后被夹具拒绝；SDK 附带环境中的无关 Authorization；
结构化参数拒绝被误分类为 `invalid_request`。
Messages 和模型列表共用地址规范化，兼容无版本 / `/v1` / 完整请求地址，保留网关前缀，
不改已有 Provider 记录。Anthropic 的 wire Schema 只将不支持的数值 / 长度 / 数组约束移到
description，本地仍校验原始 Schema，负帧索引继续拒绝；关闭 SDK 环境 / profile 默认配置。
结构化参数拒绝按 `unsupported_feature` 返回，不泄漏错误正文、不改用无约束请求。
依据：[Anthropic 结构化输出与 Schema 限制](https://platform.claude.com/docs/en/build-with-claude/structured-outputs)
（2026-10-03 核验）及锁定的 `anthropic-sdk-go v1.71.0` 源码。
验证：定向 `CGO_ENABLED=0 go test ./internal/ai/... ./internal/app -run 'TestGenerateAccepts|TestGenerateIgnores|TestGenerateAdapts|TestGenerateClassifiesStructured|TestListModelsAnthropicAccepts|TestAnthropicSavedPasted|TestOutputSchemaPreserves' -count=1`
通过，覆盖地址、原始 Schema 校验、密钥隔离、错误脱敏与单次请求。
`./scripts/gate.sh` 通过：Go 内部测试（含实际 `transcribeOutput` 生产 Schema 夹具与既有
图文契约）/ vet / `CGO_ENABLED=0` 构建、三平台核心交叉构建、前端 249 项单测 / typecheck /
build、文档 58 篇 0 问题；Windows 安装器 9 项中 4 项通过、5 项因需 Windows 主机跳过，
不记为通过。`gofmt -l .` 无输出，`git diff --check` 通过。Wails 引导构建有既有
macOS deployment target / UserNotifications 可用性告警，构建成功，不等同原生回归通过。
限制：匿名服务器不证明真实 Anthropic / 中转服务可用，真实 Wails 与真实 Provider 回归未运行，
不提升历史 G-loop / G-native 验收范围。回退：撤销本次提交，无迁移、无用户数据变更。

2026-10-03 每 Provider User-Agent 覆盖（test 未提交工作树，macOS arm64）：每个 AI 服务可配
`User-Agent`，留空保持现状（openai 用 Go 默认，anthropic 用 SDK 默认）。`providers.user_agent`
（迁移 v21）与三个 DTO 的 `userAgent`；`internal/app` 校验（trim、≤512、仅可打印 ASCII、拒绝
CR/LF）；`internal/ai/factory` 以自定义 RoundTripper 覆盖三协议生成路径——Anthropic SDK 自带默认
UA，只能从更外层覆盖；`ai.ListModels` 同步携带。前端表单加「User-Agent」行（文本框 + 预设下拉：
Claude Code / Google Chrome），九语言同步。
夹具先于实现：v20→v21 迁移（新匿名夹具 `v20-plan-blocks.db`）、UA 往返与五类非法值、三协议
请求头 UA（含覆盖 SDK 默认）、`ListModels` UA、前端 store 往返与预设合法性。
验证：`./scripts/gate.sh` 通过（前端 227 项、`check-docs` 58 篇 0 问题；Windows 安装器夹具 5 项
因需 Windows 主机跳过）；`gofmt -l .` 无输出。浏览器实测预设填入、非法值拦截、保存成功，
浅 / 深色正常。未运行：真实网关按 UA 放行的端到端、Wails 窗口内交互与重启读回。
回退：撤销即可，新增列默认空串，无数据回滚。

2026-10-03 报告 Token 用量显示开关（test 分支未提交工作树，macOS arm64）：设置「AI 服务」分区新增
`llm.showTokenUsage`（默认关），决定日报 / 周报末尾的 Token 用量卡片是否渲染。默认值与规范化由
`internal/settings` 承担，DTO 走 `LLMSettingsDTO.showTokenUsage`；前端新增
`stores/tokenUsageVisibility.ts`（读 `GetSettings`、订阅 `settings:changed`、写经 `UpdateSettings`），
日报 / 周报只按它 `v-if`——关闭只影响渲染，卡片自身的读取、失败态与 `GetTokenUsage` 不变。
夹具先于实现：Go 键默认关 / 写入读回 / 空 patch 不改写、`SettingsDTO` JSON 含 `llm.showTokenUsage`；
前端 store 五项——默认关、读回 true、读失败保持隐藏、写回采用规范化值、写失败不翻转。
验证：`./scripts/gate.sh` 通过（Go 内部测试 / `go vet` / `CGO_ENABLED=0` 构建、三平台核心交叉构建、
前端 `test:unit` 223 项通过、typecheck、build、`check-docs` 58 篇 0 问题；Windows 安装器夹具 9 项
中 5 项因需 Windows 主机跳过，未记为通过），`gofmt -l` 无输出。匿名浏览器预览（vite 夹具 + Chrome
headless 1280×900）：夹具为开时日报卡片 1 个、设置行与说明文案正确；开关切到关后日报卡片 0 个，
返回设置仍为关，再切到开后周报卡片 1 个，pageerror 为 0。未运行：真实 Wails 窗口内的切换与重启读回。
键为新增项，旧库缺该行按默认关读取。回退：撤销本次提交即可，无数据迁移。

2026-09-27 默认输入增量（基于 `c1a054f` 工作树，Windows amd64）：进入模型测试与试用页时，预填随包内置的 Daygo 软件图标 PNG 和当前界面的本地化描述提示词（简体中文为「请描述这张图片的内容」）。图标以打包内联 data URL 同时用于预览和请求字节，无需联网加载；用户可移除或替换图片、编辑文字。仅点击发送才调用模型，重新进入页面恢复默认值；切换语言不覆盖正在编辑的文字。
验证：`npm --prefix frontend run test:unit` 186/186；`npm --prefix frontend run build`（含 vue-tsc）通过；检查构建产物内联图片与源 PNG 字节完全一致。新增匿名夹具先于实现覆盖 PNG 格式 / 大小 / 像素及九语言默认提示词。真实桌面发送未运行；回退仅需恢复页面默认值，不涉及数据迁移。


2026-09-27 测试与试用合并（`6c080c0` + `54b3044` 待提交合并工作树，Windows amd64）：
根据用户反馈，旧探针要求固定图像识别与严格 JSON，而试用只要求非空文字，两者判定不同。
产品设置入口统一为「模型测试与试用」，逐模型跳转携带 providerId / model，所有发送走 `TryProvider`。
移除表单草稿探针；先保存配置再从模型列表测试，不自动保存或自动发请求。
实际回复即本次测试结果，明确成功状态；输入或模型变化清空旧结果。九语言同步。
旧 Go 探针绑定保留兼容，没有产品页面入口；不把收到回复提升为图片理解 / 结构化能力认证。
用户确认现有试用能正常返回，但未提供模型、协议或逐项记录，合并后桌面闭环仍待实测。
验收夹具先于实现添加：设置页面不得调用旧探针，模型入口保留选择参数。
`npm --prefix frontend run test:unit`：185/185；`npm --prefix frontend run typecheck`：通过。
`npm --prefix frontend run build`：通过；本次路径 `git diff --check`：通过。
文档检查在匿名工作树快照运行（仅将不可读的 CLAUDE.md 链接物化为 AGENTS.md），55 个 Markdown、0 问题；原文件未改。
本次仅前端与文档变更，Go 行为未改。回退可单独恢复本次入口、文案和结果清除逻辑，不涉及数据迁移。


2026-09-26 新增「模型试用」页面（本条在当日历史用户验收之后，真实闭环待验收）：
设置页及各模型行可进入独立页面，选择已保存模型，上传 / 拖入 / 粘贴一张 PNG/JPEG 并预览，
编辑文字后显式发送；支持纯文字或单张图片。`TryProvider` 复用三协议客户端与系统密钥，
单次、无回退、无历史、非流式返回实际文本；展示模型 / 耗时和本地化失败，支持复制回复。
上限为 5 MiB / 2000 万像素 / 16000 字符 / 2048 输出 tokens / 30 秒。
内容只在内存中保留，卸载清空并忽略迟到结果；仅 `llm_calls` attempt 元数据落库，因此要求持锁读写实例。
回复以转义纯文本呈现；后续统一测试入口的行为以上述 09-27 记录为准。
契约和 DTO 见 [05 Provider](../05-interface-contract.md#provider)。

> **验收状态（2026-09-26）**：本模块所有已实现能力（含近期增量、长期观察与已实现的真实安装升级）经用户确认已验收，未附逐项运行记录。未实现能力、待定设计与正式证书缺失保持原状态；历史命令的失败、跳过或未运行不改写为通过。统一记录见 [09 §9.1.1](../09-roadmap.md#911-本轮验收记录与证据边界)。

实现进度：部分实现。Go 侧已落地：三协议客户端、重试 / 回退链（`ai.Chain`，循环降级）、
迁移 v4 的 `providers` 表与 `ProviderRepo`、Secrets 端口（macOS 经
`security` CLI、Windows 经 Credential Manager、Linux 经 Secret Service / `secret-tool`，以及 fake）、
Provider CRUD / 路由链 / 密钥绑定（主要在 `internal/app/providers.go`），以及
`providers_models.go` 的模型列表与 `provider_playground.go` 的模型测试与试用（`TryProvider`）。
旧连接探针 `TestProvider` / `TestProviderConnection`（含 `ai.TestConnection`）与无调用方的
`SetProviderSecret` 于 2026-10-02 移除。单供应商多模型已落地（v17：`providers.model` → `models` JSON 数组，
上限 20）：路由链条目改为「供应商 + 模型」对，`ai.Chain` 按 `providerID + "\x1f" + model`
复合键独立计数，钥匙串与 `llm_calls.provider_id` 仍用裸供应商 ID
（decisions/providers-multi-model）。设置层 `providers.routing` 为有序对链，兼容旧的裸 id
数组与 `{primary,secondary}` 形状（读取时折叠）。
前端 store 已以 Go 绑定为权威来源，写后重拉；表单支持多模型增删与逐模型测试，回退链编辑器
以单一有序列表编排「供应商 + 模型」对；旧 localStorage 记录只在后端列表为空时做一次性
无密钥迁移（单模型折为一元列表），成功后删除。`hasSecret` 仅由后端检查钥匙串后返回。
模型列表查询与每 Provider 图片上限（v11，per-provider、与模型无关）也已接入；10-05 模型列表
仅作为 combobox 候选，由用户逐项选择或手填，来源不变时保留候选。每 Provider 的
User-Agent 覆盖已落地（v21 `providers.user_agent`，空串 = Go/SDK 默认；`internal/ai/factory`
经 RoundTripper 对三协议统一注入，`ai.ListModels` 同步携带）。真实网络集成、完整 Wails 重启
闭环与升级身份验证经用户确认已验收。

## 能力与跨层职责

| 输入 | 可先推进 | 真实接入条件 |
|---|---|---|
| data: db-core；preferences: settings-access | 匿名配置、路由和 repository fixture | Provider 表与路由落库，迁移 / 事务测试 |
| Secrets 端口；delivery 身份证据 | 内存 fake 验证写入 / 删除 / 缺失 / 失败 | 原生钥匙串及同签名重启 / 升级验证 |
| ui-bridge 与用户配置的服务 | DTO、HTTP 测试服务器、错误 / 重试夹具 | G-host、生成绑定；实际网络调用前用户明确配置目的地 |

输出 provider-client，包含文本 / 内存图片输入、JSON Schema 结构化输出、三种原生协议适配
（openai / openai_responses / anthropic，经 `internal/ai` factory 统一构造）、路由、取消、
重试与回退装饰器。图片仅接受
JPEG / PNG / WebP，最多 5 张、单张 5 MiB、原始总量 20 MiB；调用记录只存 attempt 元数据，
不存 endpoint、正文、图片、密钥或费用。
协议客户端归 internal/ai；上层任务通过消费者接口调用，不导入另一服务的内部实现。
providers repository 在 internal/storage；Secrets.Get 只供 Go 客户端取密钥，
任何绑定均不返回密钥。settings-access 由 preferences 维护，本模块拥有 providers.routing
的字段规则和设置交互；回退链跨回合
行为由消费方（chat / 分析流水线）集成验证。模型输出语言已无独立设置：卡片 / 摘要与
chat 回复跟随 `appearance.language`（`llm.outputLanguage` 于 2026-10-03 移除）。

分析分组按回退链中最小图片上限限制请求规模。识别增强（`ai.GenerateRecognition` 与
`llm.recognitionEnhancementEnabled`）于 2026-10-02 移除：设置页开关可保存，但转录阶段从未调用
该函数，开关无实际效果（此前本节「已接入」的描述与代码不符）。

## 实验与失败条件

| 实验 | 输入与操作 | 预期结果 | 失败条件 / 证据 |
|---|---|---|---|
| 配置往返 | 匿名配置、链重复 / 不存在、空密钥与显式删除 | 规范化后落库，空密钥保持不变，删除只能显式触发 | 重启丢配置、错误路由、误清密钥失败 |
| 密钥边界 | 测试专用临时密钥写入 / 删除，重启并检查 hasSecret | 值只在钥匙串 / Go 客户端；UI 仅布尔值 | DTO、日志、错误、localStorage 出现密钥立即阻塞 |
| 协议 / 重试 | 匿名 HTTP 服务器返回成功、限流、超时、错误体；取消任务 | 请求符合各协议，错误脱敏，重试有界并传播取消 | 重试失控、请求目的地错误、后台任务无法结束失败 |
| 真实连接 | 用户指定 Provider 与模型，在模型测试与试用页显式运行 TryProvider | 一次实际测试调用，结果及耗时可见，不回显 payload | fake 成功不可替代此项；网络失败不谎报可用 |
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
5. 验证真实连接与设置重启闭环，补空态、失败态、加载态和九种语言。

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

- **2026-10-05 配置模型 combobox（`a9dc930` 的 test 工作树，macOS arm64）**：匿名输入为已存服务
  `https://example.invalid/v1`、自定义模型和返回的两项模型；期望获取仅更新候选、选择 / 手填保存
  精确对应用户输入、其他模型行继续可选、旧来源响应不覆盖当前列表。原代码的五组夹具失败，
  未改变行为期望；测试宿主补全 DOM / Teleport 操作，鼠标夹具跟随最终 click 事件选择。
  完整 `./scripts/gate.sh` 通过（Go 内部测试 / vet / `CGO_ENABLED=0` 构建、三平台核心交叉构建、
  前端 254 项单测 / typecheck / build、59 篇文档 0 问题；Windows 安装器 9 项中 4 项通过、
  5 项需 Windows 主机跳过）。最终 click 收尾与追加来源竞态夹具后，
  `npm --prefix frontend run test:unit` 255 项全通过，`npm --prefix frontend run build`
  （含 vue-tsc）通过；`python3 scripts/check-docs.py` 59 篇 0 问题，`git diff --check` 通过，
  `gofmt -l .` 无输出。Wails 引导有既有 macOS deployment target / UserNotifications 告警，
  构建成功，未作为原生回归证据。
  内置浏览器匿名开发夹具实测：获取保留两条既有模型；鼠标选 `dev-standard`，第二行输入
  `large` 后 ArrowDown / Enter 选 `dev-large`；第一行手填 `manual-fixture-model` 后立即保存，
  模型标签精确为 `manual-fixture-model` / `dev-large`。重新编辑可用已存密钥获取下拉候选；
  切换为 Anthropic 后候选为空且「获取模型」停用，不复用旧协议的已存密钥。
  前一轮预览与绑定生成引起的热更新重叠，内存夹具被重置，该轮不作为最终保存证据；构建后
  重载再完整复测。未运行真实 Wails / 外部 Provider；浏览器结果不提升真实闭环或 G-native。

- **2026-10-02 AI 服务设置页整理（`frontend-lab` 分支）**：版式改为与其它设置页一致的平铺行与细分隔线——标题行右侧放
  「模型测试与试用」「添加服务」；每个服务一行（名称、主 / 备标记、接口类型、地址、密钥状态、最多图片数），模型为可点的
  小标签直达该服务与模型的测试页（取代每个模型下的按钮）；编辑在该行原位展开、添加在列表顶部展开；表单顺序改为
  名称 → 接口类型 → 地址 → 密钥 → 模型 → 图片数；使用顺序与钥匙串说明改为同样的行式。功能补齐：编辑已保存且钥匙串
  有密钥的服务时，地址与接口类型未改、未输入新密钥即可「获取模型」，按既有 `ListProviderModels` 的 providerId 形式
  由 Go 读取已存密钥；地址或类型一旦改动即不再使用已存密钥，避免把密钥发往新地址。未改后端与契约。验证：typecheck、
  unit（208 项，含设置页仅暴露测试页入口的源码检查）、build 通过；内置浏览器匿名夹具下检查浅 / 深色版式，并操作编辑、
  删除确认、添加并保存；在内存中把夹具服务标为已存密钥，确认无需重填即可获取模型、改地址后按钮停用。限制：未在真实
  Wails 窗口与真实钥匙串下获取模型。

- **2026-09-29 Qwen3-VL 配置修复复测（`939a75b` 工作树，Windows amd64 + WSL Ubuntu）**：
  官方 `qwen3-vl:8b-instruct` 已下载，模型元数据确认 renderer / parser 为 `qwen3-vl-instruct`，
  能力不含 thinking。保留原 Thinking 模型；LiteLLM 外部别名仍为 `qwen3-vl-8b`，
  仅将其上游改为 `ollama_chat/qwen3-vl:8b-instruct`，保留 `num_ctx: 8192`。
  已验证其余 YAML 字段不变，修改前在配置所在目录生成权限 `0600` 的备份，重启服务生效。
  `CGO_ENABLED=0 DAYGO_ANONYMOUS_PROBE=1 DAYGO_PROBE_INSTRUCT_DIRECT=1 go test ./internal/analysis -run '^TestLocalAnonymousProbe$' -v -count=1`
  直连通过：转录 12.011 秒、卡片 4.727 秒。移除 `DAYGO_PROBE_INSTRUCT_DIRECT` 后同一测试经
  已保存 Provider / LiteLLM 别名再次通过：转录 9.018 秒、卡片 3.528 秒，各返回一项；
  两阶段均 `finish_reason=stop`、推理字段为空，严格 Schema、非空标题 / 摘要、既有分类和
  匿名 10:00–10:15 时间窗检查通过。Ollama `/api/ps` 确认实际上下文为 8192。
  输入为仓库匿名 PNG，转录结果映射到固定匿名窗口；真实截图、既有失败批次重试、Wails
  展示及长期稳定性未在本次复测，不将此匿名真实 Provider 验证提升为完整 G-loop 验收。
  Daygo 生产代码、输出预算和校验规则未改。回退：恢复修改前 YAML 并重启 LiteLLM；原模型保留，
  不回滚或删除用户数据库。文档在隔离快照中运行 `scripts/check-docs.py`（仅将不可读的
  CLAUDE.md 链接物化为 AGENTS.md 文本）：56 篇 Markdown、0 问题。限定文档路径的
  `git -c core.whitespace=cr-at-eol diff --check` 通过（保留 providers.md 已入库的 CRLF）。

- **2026-09-27—28 本地 Qwen3-VL / LiteLLM 排查（`058ea03` 工作树，Windows amd64 + WSL Ubuntu，Ollama 0.32.11）**：
  模型试用成功不等于分析可用。只读调用元数据显示转录失败为 `invalid_output`；匿名图标配合
  生产转录提示词 / Schema 复现 HTTP 200、空正文。上游 `ollama/qwen3-vl:8b` 改为
  `ollama_chat/qwen3-vl:8b` 后转录可以通过，但匿名卡片请求在 4096 上下文下出现
  2797 输入 + 1299 输出、`finish_reason=length`、正文为空。设 `num_ctx: 8192` 后实际加载值
  已核验；随后卡片仍耗尽 4096 输出预算，只有推理内容。模型元数据显示 renderer / parser 为
  `qwen3-vl-thinking`；`reasoning_effort: none` 实测仍返回空正文和推理字段，已撤回该尝试。
  因而不能把适配路径修改或扩大上下文单独记录为修复完成，也不能将推理字段当成最终卡片。
  后续验证目标为官方 `qwen3-vl:8b-instruct`，保留原模型和外部模型别名；结果见上方 09-29 记录。
  显式启用的 `TestLocalAnonymousProbe` 使用仓库内匿名 PNG、生产提示词和严格 Schema，
  只读配置与系统密钥，密钥仅在内存中用于已配置服务，不发送真实截图、不写用户数据库。
  此记录不提升 G-loop / 长期观察验收；Daygo 代码、输出预算和校验契约未修改。

- **2026-09-26—27 模型试用（基于 `ab8624b` 的 test 工作树，Windows amd64）**：新增匿名 HTTP 夹具覆盖三协议实际图文
  请求 / 原始文本返回、无 schema / 单次调用、非法模型零请求、MIME 不一致 / 非图片 / 超限输入拒绝、
  错误不回显正文。前端夹具覆盖重复发送闸门、清理后的迟到结果丢弃、上传大小 / 类型及回复 HTML 转义。
  本次实现使用 PNG/JPEG 子集；WebP、流式、持久化历史、在途取消和批量对比未实现。
  `CGO_ENABLED=0 go test ./internal/...`、`go vet ./...`、`CGO_ENABLED=0 go build ./...` 和
  `GOOS=linux/darwin/windows CGO_ENABLED=0 go build ./internal/...` 通过；交叉构建不代表目标平台运行。
  最终补充取消 / 像素上限 / 密钥回显夹具后，`CGO_ENABLED=0 go test ./internal/app ./internal/ai/...`
  再次通过。`npm --prefix frontend run test:unit` 174 项通过；typecheck / build 通过；
  `gofmt -l` 本次 Go 文件无输出；限定本次路径的 `git diff --check` 通过。
  浏览器预览以仓库内匿名 PNG 为输入，图片预览及文字编辑符合预期；无 Wails 时发送按钮禁用，
  没有用模拟成功替代真实回复。新增真实 Provider / Wails 真机闭环未运行，不继承历史验收。
  完整 `scripts/gate.sh` 的 Go / 前端 / 三平台构建段通过，末尾 Python 入口未完成，因此不记录为
  整体通过。初次五项 Agent 夹具因硬编码 `/tmp` 不存在失败，准备临时 `E:\tmp` 后原样全量通过，
  无修改这五项预期。直接 `python scripts/check-docs.py` 因现有 CLAUDE.md 链接不可读失败；
  `git archive HEAD` 的临时快照覆盖本次 docs，并仅在快照内将 CLAUDE.md 按 HEAD 目标 AGENTS.md
  展开为文本后，原检查器检查 54 个 Markdown、0 问题。用户工作区链接保持不变。
  `python scripts/windows-installer/test_installer.py`：2 项源契约通过，7 项因缺 makensis 跳过。
  本轮夹具设计修正：Anthropic 根端点预期路径从 `/messages` 改为 `/v1/messages`，依据现有 SDK
  适配和 `TestGenerateMapsMultimodalStructuredRequest`；取消夹具先读完 HTTP 请求体再观察断连，
  保持“宿主取消必须终止在途请求”的预期不变。
  **09-27 合并复核**：模型试用提交 `6c080c0` 与远端 `54b3044` 合并，两个文档插入冲突保留双方段落。
  合并工作树上原门禁的全部 Go 测试、vet、本机构建、三平台核心交叉构建、前端 184 项单测、
  typecheck / build 通过；Python 阶段仍未完成。文档临时快照改用 `git archive origin/test` 加本次
  docs 覆盖，55 个 Markdown、0 问题；完整脚本不标通过。浏览器 600px 视口验证单列，
  document scrollWidth 等于 viewport 600px。临时快照清理后，合并检出的 Go 文件仅因 CRLF 被 gofmt
  列出；恢复标准换行后 `gofmt -l .` 无输出，Git 中没有额外 Go 内容差异。
  回退：撤回本次页面、入口和 TryProvider 绑定；无 schema 迁移，不删除既有配置 / 密钥。

- **2026-09-23 回归修复**：默认 Anthropic 端点及草稿模型列表的端点规范化由 Go 夹具验证；`./scripts/gate.sh` 通过。真实 Provider 网络调用未在本次重跑。

- **Go 与存储（2026-09-10—20）**：`go test ./internal/ai/... ./internal/app/...`、`go vet`、`CGO_ENABLED=0 go build ./...` 及匿名 TLS 夹具覆盖三协议、连接探针、重试 / 回退、错误脱敏、路由与多模型迁移。macOS 钥匙串有一次真实 smoke；Windows 与 Linux 适配器已落盘。
- **前端**：typecheck、构建和匿名配置交互覆盖 Provider 表单、模型列表、逐模型连接测试与有序回退链。
- **真实闭环**：真实 Provider 网络、Wails 重启与升级身份由用户于 2026-09-22 确认验收，未附逐项运行记录。密钥不进入绑定、日志或 localStorage 的约束仍适用。
