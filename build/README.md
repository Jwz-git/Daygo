# Build Directory

Wails 构建资源与产物目录（Wails v2 默认布局）：

- `bin/` — 构建产物输出目录（不入库）
- `darwin/` — macOS 构建文件：`Info.plist`（正式）、`Info.dev.plist`（`wails dev`）
- `windows/` — Windows 构建文件：`icon.ico`、`info.json`、`wails.exe.manifest`；
  `installer/` 是打包时从 `scripts/windows-installer/` 复制并由 Wails 补齐的临时目录
- `native/` — 原生静态库构建产物（`native/darwin/build.sh` 与
  `native/windows/build.ps1` 的输出，不入库）

修改 macOS / Windows 构建文件后需重新构建；干净检出先运行
`./scripts/bootstrap-frontend.sh`。完整打包走 `scripts/package-macos.sh` 或 Windows 主机上的
`scripts/package-windows.ps1`，由入口补齐 Sparkle framework / WinSparkle DLL、原生产物与安装器。
构建不等于签名身份验收；当前安装升级的用户确认与正式证书缺口见
[delivery 执行册](../docs/modules/delivery.md)，脚本参数见 [scripts/README.md](../scripts/README.md)。
