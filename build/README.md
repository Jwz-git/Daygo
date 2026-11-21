# build/

Wails v3 桌面构建资源、平台清单与本地产物目录。根 [`Taskfile.yml`](../Taskfile.yml) 与 [`build/config.yml`](config.yml) 编排桌面开发与构建；原生静态库和 DLL 由 `scripts/` 显式编译，不再使用 Wails v2 的 `preBuildHooks`。

## 目录结构

| 路径 | 用途 | Git 跟踪 |
|---|---|:--:|
| `config.yml` | Wails v3 应用元数据（`io.github.jwz-git.Daygo`）与 `dev_mode` 编排 | ✅ |
| `appicon.png` | 桌面应用图标源文件 | ✅ |
| `darwin/` | macOS 资源：`Info.plist`（开发与打包共用；`Info.dev.plist` 为停用的 v2 历史模板） | ✅ |
| `windows/` | Windows 资源：`icon.ico`、`info.json`、`wails.exe.manifest` | ✅ |
| `windows/installer/` | 打包时从 `scripts/windows-installer/` 复制并补齐的临时 NSIS 目录 | ❌ 忽略 |
| `bin/` | 编译出的桌面应用与可执行产物 | ❌ 忽略 |
| `native/` | 原生库产物（`native/darwin/build.sh`、`native/windows/build.ps1`） | ❌ 忽略 |
| `deps/` | 脚本下载并校验的 Sparkle / WinSparkle 构建依赖 | ❌ 忽略 |
| `diagnostics/` | 本机诊断输出 | ❌ 忽略 |

## 忽略策略与核验

仓库根 `.gitignore` 仅精确忽略上述五个生成目录，不整目录忽略 `build/`，防止新增图标、plist、manifest 或安装资源被意外漏提交。`testdata/` 仅对数据库本体和匿名压缩夹具开放例外，不放行 WAL/SHM 旁路文件、日志、环境文件或签名材料；忽略规则不能替代匿名化与敏感信息审查。

常用核验命令：

```bash
git check-ignore -v --no-index -- <path>                  # 检查命中规则（! 表示放行）
git ls-files --cached --ignored --exclude-standard        # 预期无输出
```

## 构建与打包入口

修改 `darwin/` 或 `windows/` 下的平台资源后需重新构建：

- 干净检出请先执行 `./scripts/bootstrap-frontend.sh`。
- macOS 打包走 [`scripts/package-macos.sh`](../scripts/package-macos.sh)；Windows 打包请在 Windows 主机走 [`scripts/package-windows.ps1`](../scripts/package-windows.ps1)，由脚本自动补齐原生产物、Sparkle / WinSparkle 与安装器。
- 详细参数见 [`scripts/README.md`](../scripts/README.md)，安装升级与签名身份边界见 [`docs/modules/delivery.md`](../docs/modules/delivery.md)。
