# timeline 卡片 favicon 回退：按站点域名抓取

> **最新验收确认（2026-10-13，`test` / `bf49b34` 工作树）**：本文涉及的已实现能力及后续增量，
> 此前仍待验收的项目均由用户确认已验收，未附逐项运行记录；未实现项与待定设计保留。
> 下文旧日期的失败 / 跳过 / 未运行结果是历史记录，不倒填为通过；后续新改动需独立验收。
> 正式身份与签名材料的证据边界、统一范围见 [09 §9.1.1](../09-roadmap.md#911-本轮验收记录与证据边界)。

> **状态：已决定（2026-09-16 初定，2026-09-18 抓取移至 Go 侧）。** 品牌匹配表未命中时，
> 按卡片显示的主机名抓取站标；失败不重试、不阻塞卡片渲染。抓取由 **Go 侧**
> `internal/favicon` 执行（原为 webview 内直连），经同源 `/favicon?host=` 资源路由暴露。

## 1. 行为

1. 卡片 `appSites` 需要网络站标时（显式域名，或命中前端精选知名站点映射表；未知单词桌面应用不再盲猜 `<word>.com`），`AppSiteIcon.vue`
   通过 `lib/favicon.ts` 请求同源资源 `/favicon?host=<host>`；
2. Go 侧 `internal/favicon` 并行请求 3 个主来源，并按**来源权威性与分辨率打分**择优，拒绝占位图与误跳转图标：
   - 站点直连：`https://<host>/favicon.ico`（0ms 启动，无延迟劣势；返回有效 ≥32px 官方图标时直接胜出）；
   - Google S2：`https://www.google.com/s2/favicons?sz=64&domain_url=https://<host>`（使用 `domain_url` 保留 `mail.google.com` / `drive.google.com` 等子产品图标；过滤默认 16×16 地球图与跨产品 `accounts.google.com` 登录重定向图标）；
   - icon.horse：`https://icon.horse/icon/<host>`（Google 不可达时的第二聚合器；严格拒绝 `X-Icon-Fallback: true`、`X-Icon-Confidence: 0` 与指向通用登录域的 `X-Icon-Source`）；
   - 当聚合器先于站点直连返回时，等待短暂收敛窗口（350ms）让站点直连或更高清来源有机会胜出；
   - 仅当三个主来源全部失败时，才顺序回退到：① 读取站点首页 `<head>` 前 64 KiB 内的光栅 `<link rel="icon" / "apple-touch-icon">`（同 SSRF 守卫、跳过 SVG 与 `data:` URI）；② 非多租户子域回退到可注册主域；
3. **代理**：`http.Client` 先用环境变量代理（`HTTPS_PROXY`），再回退到 **macOS 系统代理**
   （首次需要回退时执行一次 `scutil --proxy`，超时 1 秒，见 `proxy_darwin.go`）。这一步对齐 Dayflow——它的原生
   `URLSession` 天然走系统代理，这也是 Google S2 在被墙网络里仍能取到图标的原因；
4. 别名表 `hostAliases` 将短链、更名域与子品牌域规范到官网主域（如 `codex.com` / `chat.openai.com` → `chatgpt.com`、`twitter.com` → `x.com`、`youtu.be` → `youtube.com`、`b23.tv` → `bilibili.com`、`notion.site` → `notion.so`、`app.slack.com` → `slack.com`、`larkoffice.com` → `feishu.cn`）；
5. 抓取只发送**主机名**，不携带 cookie、referrer、屏幕内容；单次 4 秒超时；
6. 成功结果**落盘缓存**到 `~/Library/Application Support/Daygo/favicons/<sha256("v2:"+host)>.ico`（14 天 TTL，版本前缀使旧版缓存中的占位图/低质量图自动刷新）。
   单进程跨主机最多同时发出 12 个图标 HTTP 请求。磁盘缓存首次访问检查 1,024 文件 / 32 MiB
   预算，写入跨过预算时优先淘汰最早写入的缓存文件。各实例独立记账，共享目录可能暂时超限。
   失败在内存负缓存 10 分钟，命中负缓存直接返回；取消及排队超时不写负缓存；
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
| 抓取 / 质量打分 / 磁盘缓存 / SSRF | `internal/favicon`（`favicon.go` / `quality.go` / `guard.go` / `disk_cache.go`） |
| 资源 handler `GET /favicon?host=` | `internal/app/favicon_binding.go`，经 `serveAsset` 分发 |
| 客户端缓存 / 规范域解析 / 去重 / 负缓存 | `frontend/src/lib/favicon.ts`（请求同源资源） |
| 渲染（已安装应用 / 官网 favicon / 内置品牌图 → 手绘 SVG → monogram） | `frontend/src/components/AppSiteIcon.vue` |
| 卡片固定图标列（无图标也留位） | `TimelineActivityCard.vue` / `TimelineWeekView.vue` |
