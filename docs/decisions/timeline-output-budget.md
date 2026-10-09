# 卡片输出预算与 token 截断

状态：已决定，2026-10-09；实现与验证见 [timeline 执行册](../modules/timeline.md#验证记录)。

## 输入与候选

用户报告的 #436 脱敏调用元数据：转录成功，卡片先后三次耗尽 4096、提高额度后再次三次
输出恰好 8192，均 `invalid_output`；最终固定说明为响应未完成。原始响应未保存，不能
从用量相等单独认定具体终止原因，也不能确认推理占比。此前匿名小输入在两个额度下均
成功，未复现原故障，不能证明翻倍有效。

候选：继续提高全局固定额度；默认采用服务端预算；增加完整的模型预算 / 推理配置。
选择服务端预算，避免继续猜一个跨模型的统一数值。完整配置超出本次最小修复范围。

## 决定

- 首批、持续窗口、单卡生成与纠错请求的 `MaxOutputTokens` 为 0。Chat Completions 与
  Responses 省略对应可选参数，额度由用户配置的服务决定，不猜模型名或网关能力。
- Anthropic Messages 必须传 `max_tokens`，0 的卡片请求保留此前有效的 8192 额度；其它
  0 请求仍用既有 4096。正数显式额度保持原样。该协议的默认值仍有边界，不能宣称已消除
  所有模型的 token 截断。
- 只凭明确的协议字段识别 token 终止：Responses 的
  `incomplete_details.reason=max_output_tokens`、Chat 的 `finish_reason=length`、
  Anthropic 的 `stop_reason=max_tokens` 或 `model_context_window_exceeded`。
  Chat 的 `length` 也可能来自上下文限制，因此固定错误只说达到 token 限制。
- 使用类型化原因保留外部 `invalid_output` 分类，不接受截断正文，即使其 JSON 合法。
  此原因不在同一 Provider 内原样重试，直接交给既有回退链；其它错误保留原有策略。
  Responses 的显式 error 优先于 incomplete_details。未知原因不作为 token 限制猜测。
- 保留 Schema、本地时间覆盖与分类校验、每次 2 分钟超时及响应体大小限制；不续写部分
  JSON，不修改批次冷却 / 手动重试策略，不改变 Provider 配置、存储或用户数据。
- 可见输出采用字段分工减少重复：新增活动点合并连续重复 observation，但保留有意义
  的变化与融合时的历史点；摘要通常一两句，详细摘要仅补充前两者没有的信息，没有则为空。
  移除长标题和逐项展开动作的要求，省去 JSON 格式空白，不新增硬字数 / 条目上限。

## 验收与边界

匿名夹具先复现六个卡片入口受到 8192 截断，三协议 token 原因被重复请求三次，以及
Anthropic 必填额度在 0 时降回 4096。修复须让六入口使用协议默认、明确 token 终止只调
一次且可回退，未知原因仍保持有界重试，错误仅保留已知用量与固定说明。

服务端默认可能更高、更低或仍然不足；采用默认可能增加用量。轻量提示词指导只约束可见
内容，不证明隐藏推理受限。匿名请求与夹具不能证明 #436 恢复，真实批次 / Wails 回归须
单独记录，不能提升 G-loop / G-native。

依据（2026-10-09 核验）：[DeepSeek Responses 参数](https://api-docs.deepseek.com/api/create-response/)
说明 `max_output_tokens` 包含可见与推理 tokens，未设置推理参数使用模型默认行为；
[DeepSeek Chat 参数](https://api-docs.deepseek.com/api/create-chat-completion/)
说明 `length` 可能来自输出或上下文上限。不把官方默认值套用到用户的兼容网关。

回退：撤销本次提交，恢复卡片显式 8192 与原有三次重试；无数据库迁移。
