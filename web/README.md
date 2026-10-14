# Daygo 网页

Daygo 配套静态网页，独立于桌面应用的 `frontend/`。

当前页面同步自 [AwayC/daygo-website](https://github.com/AwayC/daygo-website)，
来源 commit 为 `712b864`（2026-10-12 同步）。
保留滚动天空、时间线卡片（随滚动逐张生成）、日报、周报图表、对话预览及本地字体 / 图标；
设计说明见 [DESIGN.md](DESIGN.md)。依赖与锁文件随来源一起更新。
同步时保留本仓库自有部分：最新 Release 下载链接（`src/downloads.js` 与单测）、本 README，
以及「截图仅发送给你配置的 AI」的隐私文案；来源移除的 Dayflow「设计参考」独立章节与
`dayflow.png` 不再保留，页尾仍保留 Dayflow 致谢。导航与章节标题中的「隐私」
随桌面应用设置改为「隐私安全 / Privacy & security」。

按用户本次要求，不扩展多语种；保留来源的中英切换，首次访问默认英文，选择后本机保存。
页面内活动、计划与对话均为静态匿名演示，不连接桌面应用、真实录制或 Provider；
对话预览不改变 chat 的 v1 不交付范围。隐私文案明确截图只发送给用户配置的 AI。

```bash
npm --prefix web ci
npm --prefix web run dev
npm --prefix web run test:unit
npm --prefix web run build
npm --prefix web run preview
```

构建产物为 `web/dist/`。Vite 使用相对资源路径，适配 GitHub Pages 项目路径。
开发地址为 `http://127.0.0.1:5180`，生产预览地址为 `http://localhost:5181`。
仓库不提交 `node_modules/` 或 `dist/`。导入时移除来源的本地 `deploy` 命令，
沿用下文的 Release 部署工作流。

## 下载链接

首页与页尾的 macOS 按钮下载 Apple Silicon DMG，Windows 按钮下载 x64 EXE。
页面加载时通过 GitHub 的公开 `releases/latest` 接口读取最新正式 Release，按发布工作流的
`vX.Y.Z` tag 匹配 `Daygo-X.Y.Z-arm64.dmg` 与 `Daygo-X.Y.Z-amd64-installer.exe`，
使用对应资产的 `browser_download_url`。发布新正式版并上传安装包后，刷新页面即可取得新链接，
无需改网页源码或重新部署网页；已打开页面需刷新。草稿和预发布不作为下载目标。

接口请求不带凭据，要求浏览器重新验证缓存，8 秒超时。某平台资产尚未上传完成时，
该平台按钮回退到最新 Release 页面；接口失败、限流或 JavaScript 不可用时也使用该页面。
不猜测不存在的安装包地址，不用 appcast、源码压缩包或其他架构替代安装器。
匿名单测覆盖版本更新、平台匹配、资产未齐、地址校验和接口失败；部署前也执行这些单测。
接口依据：[GitHub Get the latest release](https://docs.github.com/en/rest/releases/releases#get-the-latest-release)。

## Release 部署

[部署工作流](../.github/workflows/deploy-web.yml) 在正式 Release 的 `published` 事件后
检出对应 tag，安装锁文件中的依赖、构建网页，上传 Pages artifact 并部署。
草稿、预发布和普通 push 不部署；可在 Actions 中重跑失败任务。
也可在 Actions 页面手动运行（`workflow_dispatch`），此时部署所选分支当前提交的网页，
不依赖新 Release；`github-pages` environment 须允许该分支（当前允许 `main`）。
网页与安装器工作流独立，网页部署不等待安装器资产或 appcast 完成。

首次启用前，在仓库 Settings → Pages → Build and deployment 将 Source 设为
GitHub Actions；确认 `github-pages` environment 的 deployment branch/tag rules
允许正式发布 tag（例如 `v*`）。工作流和 `web/` 必须已包含在发布 tag 中。
默认项目地址为 `https://jwz-git.github.io/Daygo/`，实际地址以部署任务输出为准。

不再使用本地脚本强制推送 `gh-pages`。失败时检查 Actions 日志后重跑；需回退网页时，
重跑已验证的旧 Release 部署工作流会重新部署该 tag 的网页，不修改 Release 资产。

实现依据：[GitHub Pages 自定义工作流](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages)。
配套网页部署与下载链路于 2026-10-13 用户确认已验收，未附 Pages / CI 逐项运行记录；
本轮未重新部署或改动服务端配置，统一证据边界见 [09 §9.1.1](../docs/09-roadmap.md#911-本轮验收记录与证据边界)。

## 动画与资源占用

2026-10-14 的优化保留滚动天空、时间线生成、详情播放器与流水线演示。
页面与时间线改为按需请求动画帧：滚动 / 布局变化触发，平滑滚动与轨道位移收敛后停止；
语言 / 宽窄布局重建时销毁旧时间线的帧、定时任务和播放器。
时间线所在舞台离屏或页面隐藏时暂停生成任务，回到可见状态后按剩余延迟恢复；
离屏装饰与后台 CSS 动画暂停，后台流水线停止生成截图。
星空在加载与尺寸变化时绘制一次，随滚动淡入；标题渐变保持静态，生成状态灯改为透明度动画。
不再持续闪烁星空或平移文字渐变。

`npm --prefix web run test:unit` 包含动画帧合并 / 收敛、暂停恢复与重建销毁夹具。
浏览器回归可在已有 Playwright 环境运行（不属于 `npm ci` 的依赖）：

```bash
# 先在另一个终端启动 npm --prefix web run dev
node web/tests/performance.browser.cjs http://127.0.0.1:5180 optimized
```

脚本默认 Windows 使用 Edge，其它平台使用 Playwright Chromium；可用 `PLAYWRIGHT_CHANNEL`
指定已安装的浏览器。需从仓库根目录执行，并确保 Node 能解析 Playwright
（本轮通过 `NODE_PATH` 使用宿主提供的包）。输出到忽略的 `.tmp/`。
每个位置等待 2.5 秒后采样 3 秒，统计动画帧回调、画布重绘和 CDP 主线程任务时间。
覆盖桌面静止 / 反复重建、滚动唤醒、离屏 / 合成页面隐藏与恢复、卡片生成完成后展开、
播放器离屏停止、减少动态效果的手机布局。`baseline` 参数仅供未优化版本采集对照，
不执行新增行为断言；优化断言不得用此参数跳过。

Windows / Edge 无头浏览器的测量结果与证据边界见
[delivery 验证记录](../docs/modules/delivery.md#验证记录)。主线程任务时间不是整机 CPU、
GPU 或内存指标；macOS / Safari、真实后台切换及线上部署需另外验收。
调度依据：[Page Visibility API](https://developer.mozilla.org/en-US/docs/Web/API/Page_Visibility_API)、
[requestAnimationFrame](https://developer.mozilla.org/en-US/docs/Web/API/Window/requestAnimationFrame)；
Lenis 时钟与滚动状态按锁文件的 1.3.26 源码 / 类型核对。
