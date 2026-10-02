# Daygo 网页

Daygo 配套静态网页，独立于桌面应用的 `frontend/`。

```bash
npm --prefix web ci
npm --prefix web run dev
npm --prefix web run build
```

构建产物为 `web/dist/`。Vite 使用相对资源路径，适配 GitHub Pages 项目路径。

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
