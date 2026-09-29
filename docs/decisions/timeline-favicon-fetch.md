# timeline 卡片 favicon 回退：按站点域名抓取

> **最新功能验收（2026-09-26）**：本文涉及的所有已实现能力、长期观察与现有身份下真实安装升级，
> 均按本次用户确认记为已验收，未附逐项运行记录；未实现项、待定设计与正式证书缺失保留。
> 下文旧日期的失败 / 跳过 / 未运行结果是历史记录，不倒填为通过；统一范围见
> [09 §9.1.1](../09-roadmap.md#911-本轮验收记录与证据边界)。

> **状态：已决定（2026-09-16 初定，2026-09-18 抓取移至 Go 侧）。** 品牌匹配表未命中时，
> 按卡片显示的主机名抓取站标；失败不重试、不阻塞卡片渲染。抓取由 **Go 侧**
> `internal/favicon` 执行（原为 webview 内直连），经同源 `/favicon?host=` 资源路由暴露。

## 1. 行为

1. 卡片 `appSites` 未命中内置品牌表（vscode / claude / chrome 等）时，`AppSiteIcon.vue`
   通过 `lib/favicon.ts` 请求同源资源 `/favicon?host=<host>`；
2. Go 侧 `internal/favicon` 以 Dayflow 的方式**并行竞速**多个来源，先返回可解码图片者胜出：
   - Google S2：`https://www.google.com/s2/favicons?sz=64&domain=<host>`（Dayflow 首选）；
   - icon.horse：`https://icon.horse/icon/<host>`（Google 被墙且无代理时仍可达的第二聚合器）；
   - 站点直连：`https://<host>/favicon.ico`（聚合器领先 150ms，直连兜底）；
   - 两个聚合器都会在服务端解析站点 HTML 的 `<link rel="icon">`，因此能拿到不在
     `/favicon.ico` 的图标（如 Discourse 站）；
3. **代理**：`http.Client` 先用环境变量代理（`HTTPS_PROXY`），再回退到 **macOS 系统代理**
   （启动时 `scutil --proxy` 读取，见 `proxy_darwin.go`）。这一步对齐 Dayflow——它的原生
   `URLSession` 天然走系统代理，这也是 Google S2 在被墙网络里仍能取到图标的原因；
4. 别名表 `hostAliases`（如 `codex.com → chatgpt.com`）与 Dayflow 一致；
5. 抓取只发送**主机名**，不携带 cookie、referrer、屏幕内容；单次 4 秒超时；
6. 成功结果**落盘缓存**到 `~/Library/Application Support/Daygo/favicons/<sha256(host)>`
   （按机器抓一次，不再每次渲染重抓；离线也能显示）；失败在内存负缓存 10 分钟；
7. 抓取失败或站点无 favicon 时回落到首字母 monogram。

## 2. 为什么抓取在 Go 侧

- webview 内直连 Google S2 在部分网络环境（如 Google 不可达）无法完成，导致网站图标始终
  取不到；而已安装应用图标走本地 IPC 不受影响。表现为"app 图标能、网站图标不能"。
- Go 侧用进程网络栈，`http.DefaultTransport` 经 `http.ProxyFromEnvironment` **认系统代理
  环境变量**，用户配置代理后即可抓取；这是根因层面的修复，不是绕过。
- 契合架构：可移植逻辑归 Go，webview 只消费同源资源；与帧回放 `/media/frame` 同一模式。
- 抓取属出网行为，放独立 Go 包 `internal/favicon`（类比 `internal/ai`），**不放平台适配层**
  （适配层禁止发网络请求）。

## 3. 隐私与安全边界

发送到第三方的只有主机名字符串，它是卡片标题已经展示的信息，不是屏幕内容、窗口标题原文
或文件路径；不与任何用户身份关联。它仍是"由屏幕内容派生的信息离开设备"，因此：

- 端点固定为 Google S2 + 站点自身，不引入可配置的第三方列表；
- 只在卡片实际渲染时按需抓取，不批量回填；
- **SSRF 防护（host 来自 LLM 输出，视为不可信数据）**：`NormalizeHost` 校验为合法公网域名
  并拒绝 IP 字面量；`guardedDialer` 在连接前解析并**拒绝 loopback / 私网 / link-local / 未指定
  地址**，防止 DNS rebinding；响应体上限 512 KiB；
- 后续若引入设置开关，默认值与原版一致（开启），开关属于 preferences 模块切片。

## 4. 实现落点

| 部分 | 位置 |
|---|---|
| 抓取 / 竞速 / 磁盘缓存 / SSRF | `internal/favicon`（`favicon.go` / `guard.go`） |
| 资源 handler `GET /favicon?host=` | `internal/app/favicon_binding.go`，经 `serveAsset` 分发 |
| 客户端缓存 / 去重 / 负缓存 | `frontend/src/lib/favicon.ts`（请求同源资源） |
| 渲染（品牌 SVG → 已安装应用图标 → favicon 图 → monogram） | `frontend/src/components/AppSiteIcon.vue` |
| 卡片固定图标列（无图标也留位） | `TimelineActivityCard.vue` / `TimelineWeekView.vue` |
