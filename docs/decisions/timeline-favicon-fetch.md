# timeline 卡片 favicon 回退：按站点域名抓取

> **状态：已决定（2026-09-16，对齐 Dayflow 的 FaviconService）。** 品牌匹配表未命中时，
> 前端按卡片显示的主机名向 Google S2 favicon 端点请求站标；失败不重试、不阻塞卡片渲染。

## 1. 行为

1. 卡片 `appSites` 未命中内置品牌表（vscode / claude / chrome 等）时，`lib/favicon.ts`
   以站点主机名请求 `https://www.google.com/s2/favicons?sz=64&domain=<host>`；
2. 仅发送**主机名**——不携带屏幕内容、窗口标题、cookie（`credentials: omit`）或
   referrer（`no-referrer`），5 秒超时；
3. 会话内存缓存 + 并发去重；失败负缓存 10 分钟；**不持久化**（重启后按需重新抓取）；
4. 抓取失败或站点无 favicon 时回落到首字母 monogram（原版同样为空时隐藏）。

## 2. 隐私边界说明

发送到第三方的只有主机名字符串，它是卡片标题已经展示的信息，不是屏幕内容、窗口标题
原文或文件路径；不与任何用户身份关联。它仍是"由屏幕内容派生的信息离开设备"，因此：

- 端点固定为 Google S2，不引入可配置的第三方列表；
- 不持久化、不批量回填（只在卡片实际渲染时按需抓取）；
- 后续若引入设置开关，默认值与原版一致（开启），开关属于 preferences 模块切片。

## 3. 实现落点

| 部分 | 位置 |
|---|---|
| 抓取/缓存/去重 | `frontend/src/lib/favicon.ts` |
| 渲染（品牌 SVG → favicon 图 → monogram） | `frontend/src/components/AppSiteIcon.vue` |
| 卡片固定图标列（无图标也留位） | `TimelineActivityCard.vue` / `TimelineWeekView.vue` |
