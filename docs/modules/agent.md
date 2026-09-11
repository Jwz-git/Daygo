# agent — 对外程序化接口（CLI / agent.sock / MCP）

> 公共执行规则和门禁见 [09](../09-roadmap.md)，字段级契约唯一出处是
> [05 §5.9](../05-interface-contract.md#59-b6对外接口推迟到-v11)。

## 用户结果与范围

用户与外部 agent 通过程序化接口使用 Daygo：`daygo` CLI 查询时间线 / 日报 / 周报 /
分类与搜索；受控写入通道（`agent.sock`）修改卡片、分类与目标；MCP 客户端
（Claude Desktop、Claude Code 等）把 Daygo 当作工具源完成同样的读与授权内写入。

负责 [01](../01-product-requirements.md) 的 agent 相关用户故事与 05 §5.9 全部契约；
包含所需的 Go 服务、`internal/storage` 查询复用、CLI 可执行、socket 服务与 MCP 实现，
不改变技术分层。

非目标：不提供自有后端或远程访问（屏幕数据不离设备的原则不变）；不暴露原始帧、
分段路径或 LLM payload；不引入绑定层与 §5.9.2 操作集之外的写能力；不做团队 /
多人视角。**v1 不交付**，本册目前是设计准备。

依据：[05 §5.9](../05-interface-contract.md#59-b6对外接口推迟到-v11)、
[03 §3.1](../03-data-model.md#31-磁盘布局)（agent.sock 布局与权限）、
[07](../07-privacy-security.md)、[02 B6 边界](../02-architecture.md)。

## 当前状态与证据

实现进度：未开始。仅有 05 §5.9 契约（CLI 命令面、agent.sock 帧格式、MCP 已定约束与
候选）与本执行册；无任何 Go / 前端代码。`system.agentEditsEnabled` 设置键已在
settings 落盘但无消费者。

## 能力与跨层职责

| 输入能力 / 契约 | 负责模块 | 可独立推进 / fake 可证明什么 | 真实接入前置条件 |
|---|---|---|---|
| cards / time（查询、`day` 边界） | timeline | 匿名卡片库上的 CLI 读命令与 JSON 输出契约测试 | cards repository 与查询契约通过（★ 已达成存储层） |
| 日记 / 目标 repository | daily | `goal_set` 等写操作在夹具库上的双端协议测试 | daily 表与 repository 落盘 |
| settings-access（`agentEditsEnabled` 门禁） | preferences | 门禁拒绝路径的协议测试 | 设置变更事件已接 |
| db-core（只读连接） | data | CLI 直连只读模式（`query_only` 回读） | 已达成 |
| 写入服务路径 | timeline / daily | 同一服务层被绑定层与 agent 复用的同源断言 | 绑定层写方法实现后 |

输出能力：`agent-writes.log` 审计（含来源标记，MCP 归属待定）；对外 JSON 的
`schema_version` 信封。所有 SQL 在 internal/storage；CLI 与 MCP 不打开第二个写连接，
写一律经 `agent.sock` → 绑定层同路径。密钥、屏幕内容不进入任何对外输出。

internal/agentbridge 拥有 socket 帧协议与门禁；CLI 可执行在 cmd/ 或独立入口（随
构建链决策）；MCP 包位置随 05 §5.9.3 传输决策定。

## 实验与失败条件

| 实验 / 风险 | 输入与操作 | 预期结果 | 失败条件 / 证据 |
|---|---|---|---|
| MCP 传输决策实验 | stdio 子进程（读只读 DB + 写走 socket）与宿主内 HTTP 两个最小探针：拉起、握手、一读一写 | 两候选的握手延迟、鉴权面、崩溃隔离可量化对比 | 无证据即不得标记已决定；决策记录缺候选或回退即失败 |
| CLI JSON 契约 | 匿名夹具库 + `--json` 全命令 + `diff` 门禁 | `schema_version` 恒为 1、键序与空值规则稳定、错误走 stderr | 快照漂移、合计含 System、时间格式不一致失败 |
| socket 协议 | 双端测试：六个操作、错误码封闭集、1 MB 上限、畸形帧、`edits_disabled` | 服务端独立拒绝越权与超限；崩溃后 socket 可重建 | 客户端侧检查充当门禁、权限非 0600 失败 |
| 同源断言 | 绑定层与 agent 写路径对同一输入的副作用与事件对比 | 校验、事件、审计完全一致 | 任一路径绕过校验或漏发事件失败 |
| 真实 MCP 客户端闭环 | Claude Desktop / Claude Code 配置 Daygo 为工具源，连续使用数天 | 读查询正确、写入后 UI 刷新、无越权操作 | fixture 协议测试不构成闭环证据 |

## 实现切片与集成

1. **决策先行**：MCP 传输与进程模型落 `docs/decisions/agent-mcp-transport.md`（候选、
   探针数据、边界与回退），收敛 05 §5.9.3 的待定表。
2. CLI 只读命令：查询复用 cards / insight 聚合，`--json` 契约与快照测试先行；
   退出码与错误输出形状按 05 §5.9.1。
3. agentbridge：帧协议、错误码、大小上限、0600 权限、`edits_disabled` 服务端校验、
   `agent-writes.log`；双端协议测试（05 §5.10.3 已列）。
4. MCP 服务器：按决策实现工具面，读命令与 CLI 同源、写操作与 bridge 同集；
   真实 MCP 客户端接入验证。
5. 诊断与观测：对外接口的慢查询、错误分类计入 data 的诊断框架。

每个切片独立可验证；不要求 daily / weekly 的 UI 完成，但写操作依赖对应 repository 落盘。

## 验收、阻塞与回退

完成要求：CLI / agent.sock / MCP 三面各自的契约测试通过，真实 MCP 客户端闭环验收，
写入同源断言通过。fake 协议测试不能证明真实客户端兼容性或 socket 长期稳定性。

待决：MCP 传输与进程模型（09 §9.8 #22，本册切片 1 前必须落决策）；CLI 入口形态
（主二进制子命令 vs 独立可执行，随构建链）；MCP 工具粒度与命名；审计来源标记。

回退：停用 `agentEditsEnabled` 即关闭全部外部写入；MCP / CLI 为纯增量面，移除不影响
捕获与分析；socket 删除重建无数据损失。决策记录保留候选与回退路径。

## 验证记录

| 日期 / commit / 环境 | 命令或人工步骤 / 输入 | 期望与实际结果 | 限制 / 下一步 |
|---|---|---|---|
| 2026-09-12（本册建立） | — | 未运行 | 仅契约与执行册；切片 1 前无代码可验证 |
