# web/ — Daygo 官网

Daygo 配套静态宣传网页（独立于桌面应用的 `frontend/`），使用 Vite 构建并部署到 GitHub Pages（默认地址：`https://jwz-git.github.io/Daygo/`）。

- **上游来源与自维护部分**：同步自 [AwayC/daygo-website](https://github.com/AwayC/daygo-website)（`712b864`，2026-10-12），保留视觉设计（见 [DESIGN.md](DESIGN.md)）、中英双语切换（首次访问默认英文并本机记忆，不扩展九语种）以及滚动天空、时间线、日报、周报和对话静态演示。本仓库额外维护：GitHub Release 动态下载链接（`src/downloads.js` 及单测）、隐私安全文案（「截图仅发送给你配置的 AI」）与页尾 Dayflow 致谢。
- **演示边界**：页面内的时间线、计划与对话均为静态匿名演示，不连接本地应用、真实录制或 AI Provider；对话预览不改变 `chat` 模块在 v1 不交付的范围。

## 本地开发与构建

```bash
npm --prefix web ci
npm --prefix web run dev        # http://127.0.0.1:5180
npm --prefix web run test:unit  # 下载解析 + 动画调度单元测试
npm --prefix web run build      # 输出至 web/dist/（相对资源路径，适配 GitHub Pages）
npm --prefix web run preview    # http://localhost:5181
```

`web/node_modules/` 与 `web/dist/` 均不入库。

## 下载链接

首页与页尾按钮在页面加载时请求 GitHub 公开接口 `releases/latest`（无凭据、强制缓存校验、8 秒超时），按 `vX.Y.Z` tag 匹配最新正式版资产：

| 平台 | 匹配资产 |
|---|---|
| macOS（Apple Silicon） | `Daygo-X.Y.Z-arm64.dmg` |
| Windows（x64） | `Daygo-X.Y.Z-amd64-installer.exe` |

- 发布新正式版并上传资产后，刷新网页即可获得新链接，无需修改网页代码或重新部署；草稿与预发布不作为下载目标。
- 当某平台资产缺失、接口限流 / 失败或禁用 JavaScript 时，对应按钮安全回退到最新 Release 页面，绝不猜测链接或替换为 appcast / 源码包。
- `npm --prefix web run test:unit` 覆盖版本匹配、缺失回退、URL 校验与接口异常，部署前必跑。接口规范见 [GitHub Get the latest release](https://docs.github.com/en/rest/releases/releases#get-the-latest-release)。

## 手动部署

网页不随普通 push 或 Release 自动部署，需在 GitHub **Actions → Deploy website → Run workflow** 手动触发（当前允许分支为 `main`）。[部署工作流](../.github/workflows/deploy-web.yml) 依次执行单测、Vite 构建并发布到 GitHub Pages；需回退时重跑历史已验证的运行记录即可。

首次启用需在仓库 **Settings → Pages → Build and deployment** 将 Source 设为 **GitHub Actions**，并在 `github-pages` environment 放行目标分支。实现参考 [GitHub Pages 自定义工作流](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages)；2026-10-13 的网页部署与下载链路确认边界见 [09 §9.1.1](../docs/09-roadmap.md#911-本轮验收记录与证据边界)。

## 动画与资源占用

网页已移除 Lenis 依赖，滚轮 / 触摸 / 键盘及锚点均使用浏览器原生滚动，并针对静止与滚动时的 CPU/合成开销做了如下约束：

- **帧率与可见性调度**：演示动画按需更新且上限约 30 fps；静止时停止 `requestAnimationFrame`。时间线舞台离屏或页面隐藏时暂停生成与背景 CSS 装饰动画，恢复可见后按剩余延迟继续；语言切换或宽窄布局重建时销毁旧定时器与播放器。
- **滚动期降载**：滚动期间暂停状态灯、AI 镜头、下载图标装饰及流水线生成，最后一个滚动事件 160ms 后恢复；时间线进度直接映射滚动位置，不再运行额外缓动循环。
- **纯合成天空渲染（`src/sky.js`）**：天空渲染隔离在独立背景层，逐帧仅改 `opacity` 与 `transform`（双层渐变复用、跨色阶区间才换背景），避免写入 `html` CSS 变量触发整页样式重算；星空仅在加载与尺寸变化时重绘一次；周报悬停与提示框复用节点并限频 `transform`。
- **验证方式**：`npm --prefix web run test:unit` 包含帧合并、暂停恢复、重建销毁与天空纯合成夹具；浏览器手工采样从 `dev` / `preview` 检查首页、切换语言与布局、滚动唤醒、离屏停止与 `prefers-reduced-motion` 表现。历史无头测量结果与证据边界见 [delivery 验证记录](../docs/modules/delivery.md#验证记录)。参考规范：[Page Visibility API](https://developer.mozilla.org/en-US/docs/Web/API/Page_Visibility_API)、[requestAnimationFrame](https://developer.mozilla.org/en-US/docs/Web/API/Window/requestAnimationFrame)、[Web Animations Performance Guide](https://web.dev/articles/animations-guide)。
