# agent MCP 传输与进程模型：stdio 子进程（daygo mcp）

> **状态：方案（切片 1 落盘，基础实现进行中）。** 本文收敛 [09 §9.8 #22](../09-roadmap.md#98-待定设计清单)
> 与 [05 §5.9.3](../05-interface-contract.md#593-mcp-服务器) 的传输待定表，是
> [agent 执行册](../modules/agent.md)「切片 1 决策先行」的落盘。已定约束（读写同源、六写操作、
> `schema_version` 信封、隐私边界、`CGO_ENABLED=0` 可测）不因本决策改变。真实 MCP 客户端多日闭环
> 仍属 G 级验收，本文不代表其已完成。

## 1. 决策

- **传输 = stdio。进程模型 = 主二进制子命令 `daygo mcp`。** MCP 客户端（Claude Desktop、Claude Code
  等）把 `daygo mcp` 作为子进程拉起，走 stdin/stdout 上的 JSON-RPC 2.0（换行分隔）。
- **读走只读 DB。** 子进程用 `SQLITE_OPEN_READONLY` + `PRAGMA query_only` 直连数据库
  （`storage.OpenReadOnly`），路径解析同 CLI：`DAYGO_DB` 覆盖，否则应用支持目录下的 `daygo.sqlite`。
- **写走 `agent.sock`。** 六个写操作一律经 [agent bridge](../05-interface-contract.md#592-agent-bridge写入通道)
  的 0600 Unix socket 交给写者实例，与绑定层同一条服务路径；MCP 子进程**不开第二条直连数据库的写路径**。
- **权限模型沿用现有面**：无网络端口、无鉴权握手；`agent.sock` 的文件权限（0600）即访问控制，
  `system.agentEditsEnabled` 是写开关（服务端独立校验）。
- **CLI 入口形态一并定：** 读命令与 `mcp` 均为 `daygo` 主二进制子命令（见 [agent 执行册待决](../modules/agent.md)
  中「CLI 入口形态」）。不再产出独立的 `daygo-cli` 可执行。

## 2. 候选与取舍

| 候选 | 结论 | 理由 |
|---|---|---|
| **stdio 子进程（`daygo mcp`）** | **选定** | 与 CLI 同构：同一份只读读路径、同一条 `agent.sock` 写路径。客户端负责进程生命周期，崩溃隔离天然（子进程死了不影响宿主 daemon）。无监听端口、无鉴权设计，本地攻击面最小——契合 [07](../07-privacy-security.md) 的「屏幕数据不离设备 / 最小本地攻击面」。 |
| 宿主内 Streamable HTTP | 否决（本次） | 读写可直达服务层，但需要本地回环监听、端口选择与鉴权设计，扩大攻击面；且要求宿主 daemon 必须在运行，与「客户端按需拉起工具源」的 MCP 常见用法不符。保留为回退候选，未删除该路径的可行性。 |

判据（隐私敏感、无自有后端、常驻 daemon 已持锁）下 stdio 明显占优：它不新增任何监听面，且把读写复用到
已有的两条通道上，避免出现第四套查询/写入语义。

## 3. 证据与门禁

- **握手探针即本次实现。** `internal/mcp` 的 stdio 服务器实现了 `initialize` / `tools/list` /
  `tools/call` 最小面，配合 fixture DB 完成「拉起 → 握手 → 一读（timeline/card/…）→ 一写（经 bridge）」，
  即 [agent 执行册实验表](../modules/agent.md#实验与失败条件)「MCP 传输决策实验」的 stdio 侧最小探针。
  HTTP 侧未实现，本决策据架构与安全判据选定，不据 HTTP 基准。
- **未验证（G 级）**：真实 Claude Desktop / Claude Code 多日闭环（读正确、写后 UI 刷新、无越权）
  仍未运行；fixture 协议测试不构成闭环证据。握手延迟等量化对比只在将来重新评估 HTTP 时才需要。
- **`CGO_ENABLED=0` 可测**：MCP 服务代码与只读读路径均为纯 Go，Linux 下可编译可测试
  （[05 §5.10.3](../05-interface-contract.md#5103-接口测试门禁) CI 门禁）。

## 4. 边界与不变量

- MCP 读面与 CLI 读命令**同源**（[05 §5.9.1](../05-interface-contract.md#591-daygo-cli)）；写面恰为
  [§5.9.2](../05-interface-contract.md#592-agent-bridge写入通道) 六操作，不新增绑定层没有的写能力。
- 工具输出复用 `schema_version`（初始 1）与 §5.9.1 的 JSON 规则（键序稳定、时间
  `yyyy-MM-dd'T'HH:mm:ssZZZZZ`、空值省略）。
- `today` / `yesterday` 别名只在入口层解析；MCP 客户端**不得自行推算逻辑日**（4 点边界客户端自算必错一天）。
- 工具不暴露原始帧、分段路径、LLM payload、密钥、屏幕内容（[07](../07-privacy-security.md) 边界对 MCP 生效）。
- LLM / 客户端传入的「分类名」等一律视为数据，服务端按现有校验拒绝越权，不据此创建分类。

## 5. 审计来源标记

`agent.sock` 每次成功写入追加 `agent-writes.log`。来源标记区分 UI / CLI / MCP / chat：本次在 bridge 请求
里携带 `source` 字段（默认 `agent.sock`，MCP 客户端写入标 `mcp`），写入审计行。字段为可选、封闭取值，
未提供即记为通用 `agent.sock`——与 [05 §5.9.3 审计归属](../05-interface-contract.md#593-mcp-服务器)
的待定项一并定。

## 6. 回退

- 停用 `system.agentEditsEnabled` 即关闭全部外部写入（含 MCP 写）；读面不受影响。
- MCP / CLI 是纯增量面：移除 `daygo mcp` 子命令与 `internal/mcp` 不影响捕获、分析与绑定层。
- 若将来 stdio 被证明不足（如需要远程或多客户端并发直达服务层），HTTP 候选路径仍保留，可另立决策记录
  切换，届时需补齐两候选的握手/鉴权/崩溃隔离量化对比。
