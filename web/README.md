# Daygo 网页

Daygo 配套静态网页，独立于桌面应用的 `frontend/`。

当前页面导入自 [AwayC/daygo-website](https://github.com/AwayC/daygo-website)，
固定来源 commit 为 `20d938c354bdf02c142f89a98d01ee2be7461287`（2026-10-07 导入）。
保留滚动天空、时间线卡片、日报、周报图表、对话预览及本地字体 / 图标；
设计说明见 [DESIGN.md](DESIGN.md)。依赖与锁文件随来源一起更新。
页面对 Dayflow 的介绍统一为「设计参考 / 设计灵感」，与本项目 README 的产品定位一致。

按用户本次要求，不扩展多语种；保留来源的中英切换，首次访问默认英文，选择后本机保存。
页面内活动、计划与对话均为静态匿名演示，不连接桌面应用、真实录制或 Provider；
对话预览不改变 chat 的 v1 不交付范围。隐私文案明确截图只发送给用户配置的 AI。

```bash
npm --prefix web ci
npm --prefix web run dev
npm --prefix web run build
npm --prefix web run preview
```

构建产物为 `web/dist/`。Vite 使用相对资源路径，适配 GitHub Pages 项目路径。
开发地址为 `http://127.0.0.1:5180`，生产预览地址为 `http://localhost:5181`。
仓库不提交 `node_modules/` 或 `dist/`。导入时移除来源的本地 `deploy` 命令，
沿用下文的 Release 部署工作流。

## Release 部署

[部署工作流](../.github/workflows/deploy-web.yml) 在正式 Release 的 `published` 事件后
检出对应 tag，安装锁文件中的依赖、构建网页，上传 Pages artifact 并部署。
草稿、预发布和普通 push 不部署；可在 Actions 中重跑失败任务。
网页与安装器工作流独立，网页部署不等待安装器资产或 appcast 完成。

首次启用前，在仓库 Settings → Pages → Build and deployment 将 Source 设为
GitHub Actions；确认 `github-pages` environment 的 deployment branch/tag rules
允许正式发布 tag（例如 `v*`）。工作流和 `web/` 必须已包含在发布 tag 中。
默认项目地址为 `https://jwz-git.github.io/Daygo/`，实际地址以部署任务输出为准。

不再使用本地脚本强制推送 `gh-pages`。失败时检查 Actions 日志后重跑；需回退网页时，
重跑已验证的旧 Release 部署工作流会重新部署该 tag 的网页，不修改 Release 资产。

实现依据：[GitHub Pages 自定义工作流](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages)。
代码配置不代表 Pages 服务端已启用或真实部署已验收。
