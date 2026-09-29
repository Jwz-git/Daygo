# providers 单供应商多模型：JSON 列 + 路由「供应商 + 模型」对

> **最新功能验收（2026-09-26）**：本文涉及的所有已实现能力、长期观察与现有身份下真实安装升级，
> 均按本次用户确认记为已验收，未附逐项运行记录；未实现项、待定设计与正式证书缺失保留。
> 下文旧日期的失败 / 跳过 / 未运行结果是历史记录，不倒填为通过；统一范围见
> [09 §9.1.1](../09-roadmap.md#911-本轮验收记录与证据边界)。

> **状态：已决定（本轮实现范围内）。** 本文记录把「一个供应商恰好一个模型」放宽为
> 「一个供应商可配置多个模型、并在回退链里分别排序」的取舍；公共规范
> （[03 §3.3.5](../03-data-model.md#335-设置与-provider)、
> [05 §Provider](../05-interface-contract.md#provider)、
> [providers 回退链](providers-fallback-chain.md)）在实现同批 commit 内同步。

## 1. 决定

一个供应商（同 endpoint、同 key）持有一个**有序模型列表**，回退链引用
**「供应商 + 模型」对**——同一供应商的不同模型可分别排进链、独立排序与回退，分析流水线
按具体模型走。

- **存储：JSON 数组列。** `providers.model TEXT` → `providers.models TEXT NOT NULL
  DEFAULT '[]'`（v17，建新表→回填→改名）。模型无独立身份，只是附在供应商上的有序串
  列表，不建子表。每供应商模型上限 20。
- **路由条目 = `{providerId, model}` 对。** `providers.routing.chain` 从裸 id 列表变为对
  列表，上限仍 8 条，按 (providerId, model) 对去重。空 `model` = 该供应商的首个模型
  （`models[0]`），与 chat 现有 `model==""` 跟随约定一致。
- **`ai.Chain` 计数用复合键** `providerID + "\x1f" + model`：同一供应商两个模型是链上两环，
  必须独立计数，否则降级游标会互相干扰。钥匙串与 `llm_calls.provider_id` 仍用**裸供应商
  ID**，只有链计数键复合（见 [providers 回退链](providers-fallback-chain.md)）。
- **`TestProvider(id, model)` 增 model 参数**：供应商多模型后，卡片按模型逐个测试；空
  model 回退到首个模型，保持旧探针行为。
- **`max_images` 保持 per-provider**（与具体模型无关），不引入模型级图片上限。

## 2. 候选与取舍

### 2.1 选中：JSON 数组列 + 路由引用「供应商 + 模型」对

**优点。** 一次改动同时满足「一个 key 下配多个模型」和「不同模型在回退链里独立排序 /
回退」两个诉求；模型没有跨表身份，JSON 串列表是最轻的表达，读写都在 `internal/storage`
一处 marshal/unmarshal。链按对计数让「同 key 换模型」成为一等回退手段（如同一网关的
高配模型失败后自动降级到低配模型）。

### 2.2 未选中：模型建独立子表 `provider_models`

**淘汰原因。** 模型没有独立生命周期或外部引用，子表带来的连接、外键与迁移成本换不到
表达力；有序性还得再加 `sort_order` 列。JSON 数组直接携带顺序。

### 2.3 未选中：路由仍引用供应商 id，模型固定取 `models[0]`

**淘汰原因。** 那样多模型只能「配置」不能「分别路由」，等于没满足回退诉求；且与「同一
供应商不同模型独立降级」直接冲突。

## 3. 兼容与迁移

- **v17 数据迁移**：旧单列 `model` 折为一元数组 `["<model>"]`，空 `model` 折为 `[]`
  （建新表→回填→改名，回填在 Go 内 `json.Marshal` 以保证编码正确）。
- **路由值级折叠（无需 SQL 迁移）**：`RoutingEntry.UnmarshalJSON` 把裸字符串 `"id"` 读成
  `{id, ""}`，对象原样读入；`decodeRouting` 再把更旧的 `{"primary","secondary"}` 折为
  `[{primary,""}, {secondary,""}?]`。三种历史形状走同一条解码路径。
- 同一供应商只排入一个模型（空 model）时，行为与旧的「一供应商一条目」完全等价。

## 4. 边界与已知空白

- chat 会话的模型选择 UI（下拉列出所选供应商的多模型）**不在本轮范围**：chat 侧只改共享的
  链构建以消费新路由形状，`chat_conversations.model` 覆盖字符串保持可用，会话默认仍跟随
  供应商的首个模型（`models[0]`）。
- 单元分隔符 `\x1f` 作为复合键连接符：它不会出现在 v4 形状的 provider id（hex）或模型名里，
  因此可安全拆分。若未来模型名允许任意字节，改用结构化键而非字符串拼接。

## 5. 回退

若「按对路由」在实践中过于细碎，收敛点在设置归一化层
（`internal/settings.normalizeRouting`）：把同一 providerId 的多条目坍缩为首条，链退回
「按供应商」语义；`providers.models` 列与 DTO 形状不变，前端路由编辑器与 `ai.Chain` 的复合
键实现也无需改动。
