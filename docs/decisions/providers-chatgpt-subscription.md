# ChatGPT 订阅接入

- 日期：2026-10-20
- 模块：providers / delivery（首次引导）
- 状态：已决定；实现与验证进度见 [providers 执行册](../modules/providers.md)

## 决定与依据

用户确认将 AI 服务与首次引导共用入口命名为「ChatGPT 订阅」和「配置API Key」。
采用 OpenAI 面向开源、本地运行应用的 Sign in with ChatGPT 动态注册流程，不依赖 Codex
安装或其登录文件，不使用 ChatGPT backend-api、不冒用其它客户端身份，也不建立自有后端。
登录身份与套餐调用授权分开校验；仅身份登录不足以配置自动分析。

2026-10-20 核验的官方依据：
[注册与授权](https://developers.openai.com/siwc/token-sharing-open-source/sign-in)、
[会话与刷新](https://developers.openai.com/siwc/token-sharing-open-source/profiles-and-sessions)、
[推理与模型目录](https://developers.openai.com/siwc/token-sharing-open-source/models-and-inference)、
[预览限制](https://developers.openai.com/siwc/token-sharing-open-source/preview-limitations)、
[错误恢复](https://developers.openai.com/siwc/token-sharing-open-source/errors-and-recovery)、
[交互规范](https://developers.openai.com/siwc/ui-ux-guidelines)。文档路径表示核验时间的契约，
不保证任何具体账号、地区或模型已通过实际授权。

## 边界与失败条件

- `chatgpt` 为独立的 Provider 协议值，复用现有 providers 表，无 schema 迁移。固定官方
  `https://api.openai.com/v1`，不能从 API Key 草稿创建，也不能修改为第三方目的地。
- 系统浏览器 + `127.0.0.1` 动态端口 `/auth/callback`；每次生成 state / nonce / PKCE，
  验证签名、issuer、audience、期限与账号。首次 `dynamic_agent_client`，保存实际返回的
  client ID、稳定宿主 ID；不需要客户端密钥。
- OAuth 数据由 Go 管理，token、ID token 与 account/client 映射全部保存在系统凭据存储，
  不进入数据库、前端、localStorage、日志或诊断。凭据按不可变小块 + manifest 原子切换，
  适配 Windows 单条 2560 字节限制；写入失败不替换旧 bundle。失败清理可能遗留无引用块，
  不构成可读取的有效 session；不据交叉构建宣称 Windows 真机凭据验证完成。
- 读取状态不触发刷新。请求按需刷新、同一 Manager 串行轮换；退出时尝试远端撤销再清除
  本地 token，保留注册映射。不能确认远端撤销时明确提示 ChatGPT 设置中的断开入口。
- Responses 强制 `store:false` / `stream:true`；按流完整终态判断成功，图片与 Schema 沿用
  显式映射和本地校验。省略此路径不支持的 `max_output_tokens`，因此现有输出预算在订阅
  路径不生效；保留超时、响应字节与图片上限。API Key 三协议维持原有预算行为。
- 额度耗尽持久暂停该订阅的新请求，用户在管理用量后显式恢复；不推测重置时间。
  既有用户配置的回退链继续适用，不自动创建或授权新的 API 计费路径。
- 设置与引导共用配置组件，支持模型选择、重授权、断开和用量入口；同步九语言。

## 验证与回退

先固定匿名 OAuth / JWT / 流终态 / 并发刷新 / 凭据轮换夹具，再接线并跑核心与前端门禁。
真实账号图文结构化转录 → 卡片、重启读回与长期额度消耗单独记录，不用模拟通过代替。
回退前在设置断开订阅并删除服务；撤销代码提交，无数据库迁移，已有 API Key 服务继续可用。
