# Build Directory

Wails 构建资源与产物目录（Wails v3，沿用 Daygo 输出路径）：

- `bin/` — 构建产物输出目录（不入库）
- `darwin/` — macOS 构建文件：`Info.plist`（开发 / 打包共用；Info.dev.plist 为不再使用的 v2 历史模板）
- `windows/` — Windows 构建文件：`icon.ico`、`info.json`、`wails.exe.manifest`；
  `installer/` 是打包时从 `scripts/windows-installer/` 复制并由打包入口补齐的临时目录
- `native/` — 原生静态库构建产物（`native/darwin/build.sh` 与
  `native/windows/build.ps1` 的输出，不入库）
- `deps/` — 下载的 Sparkle / WinSparkle 构建依赖（不入库）
- `diagnostics/` — 本机诊断输出（不入库）

仓库根 `.gitignore` 仅忽略这里的 `bin/`、`native/`、`deps/`、`diagnostics/` 与
`windows/installer/`，不忽略整个 `build/`，以免新增图标、plist、manifest 或安装资源被隐藏。
`node_modules/`、`dist/`、`.vite/` 等生成目录在各子项目通用；不整目录忽略 `web/`。
`testdata/` 仅为数据库本体与 ZIP / tar.gz 匿名夹具保留扩展名例外，不放行数据库旁路文件、
依赖、日志、环境文件或签名材料；`.env.example` / `.env.*.example` 仍可跟踪。
忽略规则不检查文件内容，也不会停止跟踪已入库文件，不能替代匿名化与敏感信息审查。

可用 `git check-ignore -v --no-index -- <path>` 复核匹配来源（`!` 前缀表示放行），
用 `git ls-files --cached --ignored --exclude-standard` 检查已跟踪文件是否被规则覆盖，预期无输出。

修改 macOS / Windows 构建文件后需重新构建；干净检出先运行
`./scripts/bootstrap-frontend.sh`。完整打包走 `scripts/package-macos.sh` 或 Windows 主机上的
`scripts/package-windows.ps1`，由入口补齐 Sparkle framework / WinSparkle DLL、原生产物与安装器。
构建不等于签名身份验收；当前安装升级的用户确认与正式证书缺口见
[delivery 执行册](../docs/modules/delivery.md)，脚本参数见 [scripts/README.md](../scripts/README.md)。

根 Taskfile 与 [config.yml](config.yml) 编排桌面构建；原生库由 scripts 显式构建，不使用 v2 preBuildHooks。
