# chat — 应用内对话式 Agent

> 公共执行规则和门禁见 [09](../09-roadmap.md)，字段级契约唯一出处是
> [05 §5.12](../05-interface-contract.md#512-chat应用内对话式-agent设计准备未实现)。

## 用户结果与范围

用户在应用内用自然语言询问自己的时间线、日报、周报、分类与搜索结果，并在沙箱
授权范围内让 agent 完成增删改查：改卡片分类 / 标题、删除卡片、增删改分类、设定目标。

Chat **不借助外部 CLI、agent.sock 或 MCP**：它是宿主内功能（UI → B1 绑定 → chat 服务），
直接调用与绑定层同源的服务路径。工具读面与 CLI 读命令同源、写面不超出 agent.sock 的
六个操作，因此三条通道（CLI / 外部 agent / Chat）共享同一套查询与写入语义，不出现第四套。

非目标：不做通用助手或自由工具执行（工具集封闭）；不执行任意 SQL、不访问文件系统与
shell；不暴露原始帧、分段路径、密钥或 LLM payload；token 级流式输出为候选（多会话已定）；
不做团队 / 远程视角。**v1 不交付**，本册目前是设计准备 + 纯对话切片实现。

依据：[05 §5.12](../05-interface-contract.md#512-chat应用内对话式-agent设计准备未实现)、
[07 §7.5](../07-privacy-security.md#75-本地攻击面)、
[02](../02-architecture.md#21-模块图)（chat 服务）、[01 §1.7](../01-product-requirements.md#17-待决的产品问题)、
[decisions/chat-session-model](../decisions/chat-session-model.md)。

## 当前状态与证据

实现进度：**纯对话切片已实现（含绑定与 UI）**（decisions/chat-session-model）。已落地：
多会话模型（`chat_conversations` / `chat_messages`，迁移 v4）、`internal/chat` 服务（回合
状态机、全局记忆注入、会话级 provider 必选、失败 / 取消落库）、每会话单在途回合、
32 KiB 消息上限、`internal/app` 会话作用域绑定与 `chat:updated` 事件、前端 store / 视图
（对话列表在右侧、全局指令与列表并列页签、进入即新会话草稿、对话有标题后才入列表）。
**尚未实现**：工具循环与 `chat.editMode` 门禁（agent 切片）、`llm_calls` purpose=`chat`
审计行、消息留存策略（待定）。

## 能力与跨层职责

| 输入能力 / 契约 | 负责模块 | 可独立推进 / fake 可证明什么 | 真实接入前置条件 |
|------|------|------|------|
| provider-client（`internal/ai` 统一 Generate、重试 / 回退、结构化输出） | providers | 回合状态机用脚本化 fake provider 做匿名夹具单测 | 已达成（协议客户端）；chat 是否复用 `providers.routing` 是候选 |
| cards / time / insight 读查询 | timeline | 工具读面在匿名卡片库上的查询契约测试 | cards repository 存储层已达成；insight 聚合未实现 |
| 日记 / 目标 repository | daily | `goal_set` 等写工具的夹具库协议测试 | daily 表与 repository 落盘 |
| 写入服务路径 | timeline / daily | 同源断言：绑定层与 chat 工具执行同一实现，副作用、事件、审计一致 | 绑定层写方法实现后 |
| settings-access（`chat.editMode` 门禁） | preferences | 门禁拒绝路径的协议测试（服务端独立校验） | 设置变更事件已接 |
| db-core | data | chat 消息持久化走统一迁移链 | 已达成 |

输出能力：无。chat 是终端功能，不向其他模块输出能力。职责指定：chat 服务在
`internal/chat`，通过**消费者侧接口**使用读查询与写操作；工具执行器是唯一效果出口，
所有 SQL 仍在 `internal/storage`；UI 归前端 chat 视图与 store，经 `chat:updated` 重拉。

## 实验与失败条件

| 实验 / 风险 | 输入与操作 | 预期结果 | 失败条件 / 证据 |
|------|------|------|------|
| 沙箱门禁 | `editMode=readonly`（默认）与只读实例下，夹具对话诱导模型调用写工具 | 写工具被服务端拒绝，错误作为工具结果回给模型，回合继续或正常收束 | 门禁靠客户端自律、readonly 下产生任何写入 |
| 工具参数校验 | 畸形参数、未知工具、非法 `day` / 越界 `limit` | 每项返回封闭错误给模型，回合不崩溃、不循环放大，计入诊断计数 | 静默吞错、无界重试 |
| 预算与取消 | 超长对话、模型连续调用工具、超大工具结果、中途取消 | 达到上限（8 次工具调用 / 64 KiB 结果 / 120 s）即终止回合，状态可见；取消传播到在途 HTTP | 无限循环、内存放大、取消后仍有副作用 |
| 提示注入 | LLM 回复中包含"指令"，要求越权写入或泄露隐私字段 | 输出一律视为数据；工具参数仍走 schema 校验与门禁；分类名不在现有列表即拒绝 | 任何绕过校验的写入、隐私字段出现在工具输出 |
| 同源断言 | 同一输入分别经绑定层与 chat 工具执行 | 校验、事件、审计完全一致 | 任一路径绕过校验或漏发事件 |
| 真实闭环 | 用户配置真实 provider，连续数天使用问答与受控编辑 | 查询正确、编辑后 UI 经事件刷新、`agent-writes.log` 可查 | fixture 协议测试不构成闭环证据 |

## 实现切片与集成

1. **夹具与常量先行**：固定工具清单、参数 JSON Schema、门禁与预算常量（05 §5.12）；
   沙箱拒绝与参数校验的匿名夹具。
2. chat 服务：回合状态机（结构化输出驱动的工具循环）、`llm_calls` purpose=`chat`、
   取消传播；脚本化 fake provider 的单元测试。
3. 持久化：chat 表迁移与 repository（**已落盘**：多会话模型，迁移 v4，切片 5）。
4. 绑定与事件：`SendChatMessage` / `CancelChatTurn` / `GetChatMessages`（会话作用域签名）、
   `chat:updated`；绑定清单反射测试与 05 §5.2.1 同步。
5. UI：chat 视图与 store、`chat.editMode` 设置项、空态 / 错误态 / i18n；受 G-host 约束。
   **纯对话部分已先落**：多会话列表、全局记忆（`chat.memory`）、会话级 provider 选择。
6. 真实闭环与诊断计数：工具错误、预算终止、取消计入 data 的诊断框架。

每个切片独立可验证；读工具依赖 cards / insight 聚合，写工具依赖对应 repository 落盘，
不要求 timeline / daily 的 UI 完成。**纯对话先行**（无工具循环）已由
decisions/chat-session-model 定为当前实现路径；工具循环与门禁按上列切片 1–2 顺序随后。

## 验收、阻塞与回退

完成要求：沙箱门禁、参数校验、预算与取消全部有自动化证据；写操作同源断言通过；
真实用户连续使用问答与受控编辑的闭环验收。UI 扩张与其他界面同样受 G-host 约束。

待决（[09 §9.8](../09-roadmap.md#98-待定设计清单) #23）：~~会话模型~~（已定：多会话）、
token 级流式输出（已定：原子消息）、消息留存策略、审计来源标记（与 agent 模块共通）、
~~chat 的 provider 路由~~（已定：会话级必选，新会话默认路由链首位）。

回退：`chat.editMode` 设为 `readonly` 即收回全部写能力；chat 整体是增量功能，移除不影响
捕获与分析；会话数据独立于业务表，清除不伤及时间线。

## 验证记录

| 日期 / commit / 环境 | 命令或人工步骤 / 输入 | 期望与实际结果 | 限制 / 下一步 |
|------|------|------|------|
| 2026-09-12（本册建立） | — | 未运行 | 仅设计准备；切片 1 前无代码可验证 |
| 2026-09-12（UI 占位，Vite 预览） | `npm run typecheck`、`npm run build`、Playwright 访问 `#/chat` | 通过；侧栏入口激活态、禁用输入区、深浅主题、中英文与 420px 窄宽度均符合预期 | 占位无功能；切片 1 起补 Go 侧 |
| 2026-09-12（纯对话服务，Go） | `go test ./internal/chat/`、`go vet`、`CGO_ENABLED=0 go build ./...`、`GOOS=linux` 交叉构建 | 通过；happy path、全局记忆注入、历史拼装、会话级 provider 固定（无回退）、无供应商失败态、密钥不泄漏断言、跨会话并发、空/超长消息拒绝、删除会话级联 | 绑定层与前端未接；取消中途回合仅单测路径；`llm_calls` 审计未落 |
| 2026-09-12（provider 必选改版，Go） | `go test ./internal/chat/` 全量 | 通过；新会话默认路由链首位、清空 pin 后发送落「尚未选择供应商」失败消息（不回退链） | — |
| 2026-09-12（UI 重做，Vite 预览） | `npm run typecheck`、`npm run build`、Playwright `#/chat` | 通过；对话列表移至右侧、全局指令与列表并列页签、进入即新草稿（无空态）、首条消息后入列表、供应商默认无空位、深浅主题、中英文、420px 窄宽无横向溢出 | dev 固定回复非真实供应商；`wails dev` 真机端到端未运行 |
