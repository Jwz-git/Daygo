# Build Directory

Wails 构建资源与产物目录（Wails v2 默认布局）：

- `bin/` — 构建产物输出目录（不入库）
- `darwin/` — macOS 构建文件：`Info.plist`（正式）、`Info.dev.plist`（`wails dev`）
- `windows/` — Windows 构建文件：`icon.ico`、`info.json`、`wails.exe.manifest`；
  `installer/` 是打包时从 `scripts/windows-installer/` 复制并由 Wails 补齐的临时目录
- `native/` — 原生静态库构建产物（`native/darwin/build.sh` 与
  `native/windows/build.ps1` 的输出，不入库）

修改 macOS / Windows 构建文件后，按 Wails 惯例重新 `wails build` 即可生效。
