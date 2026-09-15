# Daygo

**简体中文** · [English](README.en.md)

Daygo 是一款面向 macOS 的隐私优先、本地优先工作日志。它按固定间隔采集当前主显示器的画面，使用用户选择的 AI 服务理解工作内容，并将结果整理为可检索的每日时间线、站会摘要和复盘记录。

> **项目状态：开发中，尚未达到公开安装条件。** 九个功能模块（录制、AI 接入、时间线、
> 每日 / 每周复盘、数据管理、偏好、分发、对话）均已部分实现：Go 录制循环与截图落盘、
> 分析流水线与时间线卡片、Provider 持久化与钥匙串、每日 / 每周视图、数据维护等已有
> 代码与自动化覆盖，但没有任何模块完成真实用户闭环验收。
> 逐项状态见 [docs/09-roadmap.md §9.1](docs/09-roadmap.md#91-模块总表)。

## 为什么做 Daygo

普通时间追踪工具通常只能判断哪个应用处于前台。Daygo 希望保留工作的真实上下文：你在构建什么、调查什么、讨论什么，以及审查什么。

- 无需手动启停计时器的自动活动时间线
- 每日总结与站会内容整理
- 每周回顾与分心活动分析
- 基于工作历史的自然语言问答
- 本地优先存储与可配置的数据保留策略
- 由用户选择本地或云端 AI provider

## 隐私模型

隐私是架构约束，而不是可选模式：

- 录制、时间线和数据库默认保存在本机。
- 只有发送给用户明确配置的 AI provider 时，屏幕数据才可以离开设备。
- 可以使用本地模型，让分析过程完全留在设备上。
- 被屏蔽的应用会从采集中过滤；必要时使用脱敏占位帧。
- 分析和崩溃报告必须由用户主动选择加入，且不得包含屏幕内容、窗口标题、文件路径、凭据或 LLM payload。

所有数据都在一个目录里：

```text
~/Library/Application Support/Daygo/
```

数据库、录制、备份都在这里，密钥在系统钥匙串。没有第二处副本——卸载即彻底删除。

## 架构

```text
Vue 3 + TypeScript
        ↓ Wails bindings
Go Core
  ├── 存储与设置
  ├── 分析与 AI providers
  ├── 时间线、每日与每周洞察
  └── 生命周期编排
        ↓ internal/platform 端口
平台适配层（形态待定设计；屏幕捕获已有 darwin / windows 实现）
  └── 屏幕捕获、系统授权、钥匙串、状态栏、自动更新
```

Go 拥有全部可移植业务逻辑，并且是 SQLite 的唯一写入方。需要 macOS 系统能力的部分收拢在一组端口后面，**实现方式尚未选定**——文档只定义任何方案都必须满足的契约。这条边界让 Go Core 能在无 macOS 环境下构建与测试（`CGO_ENABLED=0`），整套测试策略以此为前提。

完整的需求、接口、数据模型、测试策略、功能模块路线与风险见[设计文档](docs/README.md)。

## 当前仓库结构

```text
cmd/daygo/                  Go 命令入口与 wails.json
internal/
  app/                      Wails 绑定、DTO、错误码与事件
  storage/                  唯一 SQLite 写入方：连接、迁移、实例锁、维护、诊断
  settings/                 app_settings 之上的类型化设置
  ai/                       三种协议客户端、重试 / 回退、结构化输出、连接探针
  platform/                 平台端口 + fake + 契约套件 + darwin / windows 适配器
                            （Linux 走 `//go:build !darwin && !windows` 占位，
                            返回 `unsupported` 而非构建失败）
  timeutil/                 凌晨 4 点逻辑日
native/                     原生截图实现（共用一份 C ABI）
  include/daygo_capture.h   ABI v1
  darwin/                   Swift + ScreenCaptureKit
  windows/                  C++ + DXGI（实验，有限真机 smoke）
frontend/                   Vue 3 + TypeScript 前端
scripts/                    引导、门禁与开发脚本
build/                      Wails 构建资源与产物
docs/                       设计文档
```

`docs/` 是设计规格；实现与验证的当前状态见
[docs/09-roadmap.md §9.1](docs/09-roadmap.md#91-模块总表)，不要把规划中的目录或行为
当作现状。

**关于 Linux：** Linux 桌面壳（Wails v2 + GTK3 + WebKit2GTK）已经能启动并加载 Vue 前端，
Go Core 与 SQLite 数据库层在 Linux 上与 macOS 等价。屏幕截图、系统授权、状态栏、钥匙串这些
需要原生能力的功能按 `internal/platform` 端口的"未实现 = 返回 `unsupported`"约定工作；
真正的 Linux 适配器属于 [docs/09 §9.8](docs/09-roadmap.md#98-待定设计清单) 的待定设计，
落决策前不会大规模实现。

## 构建与运行

环境要求：

- **macOS（主目标）：** macOS 14+、Go 1.25+、Node.js 20.19+（或 22.12+）、npm、Xcode Command Line Tools
- **Windows（实验）：** Windows 10/11、Go 1.25+、Node.js 20.19+、npm、（可选）MinGW-w64
- **Linux（初级适配）：** 任意较新的发行版、Go 1.25+、Node.js 20.19+、npm、GTK3 与 WebKit2GTK 开发头文件（见下）

```bash
git clone https://github.com/Jwz-git/Daygo.git
cd Daygo

# macOS：开发运行（自动完成依赖安装与生成产物引导）
./scripts/dev.sh

# Windows：开发运行（PowerShell）
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev.ps1

# Linux：开发运行（自动选择 WebKit2GTK ABI tag）
./scripts/dev-linux.sh

# 提交前门禁：引导 + Go 构建 / 测试 / vet / gofmt + 前端 typecheck / build
./scripts/gate.sh
```

**干净检出必须先引导，不能直接跑 `go build` 或 `npm run build`。** 两个生成目录互为前提：
`frontend/dist` 被 `go:embed all:dist` 引用（缺失则整个 Go 模块无法编译），
`frontend/wailsjs` 被前端源码引用（缺失则 `vue-tsc` 失败），而生成它又需要可编译的 Go 树。
`scripts/bootstrap-frontend.sh` 按“占位 dist → 生成绑定 → 真实 bundle”解开这个环，
`dev.sh` / `dev.ps1` / `dev-linux.sh` 与 `gate.sh` 都会调用它。
Go 1.25 的 Windows+cgo debug 构建受链接器缺陷影响；`dev.ps1` 会在调用 Wails 时临时设置
`GOEXPERIMENT=nodwarf5`，避免生成 Windows loader 无法接受的 PE，并在退出时恢复原环境。

### Linux 额外要求

Wails v2 在 Linux 上需要 GTK3 + WebKit2GTK。ABI 选择由 `scripts/dev-linux.sh`
自动检测 `pkg-config` 上的 `webkit2gtk-4.1` / `webkit2gtk-4.0`：

```bash
# Debian / Ubuntu 22.04+
sudo apt install build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev

# Fedora 40+
sudo dnf install gcc-c++ pkgconf-pkg-config gtk3-devel webkit2gtk4.1-devel

# Arch
sudo pacman -S --needed base-devel pkgconf gtk3 webkit2gtk-4.1
```

旧发行版（Debian 11、Ubuntu 20.04、RHEL/CentOS 8-9）只有 ABI 4.0 时，需要安装
`libwebkit2gtk-4.0-dev` 并显式指定：

```bash
DAYGO_LINUX_WEBKIT_TAG=webkit2_40 ./scripts/dev-linux.sh
```

打包应用：

```bash
cd cmd/daygo

# macOS
go run github.com/wailsapp/wails/v2/cmd/wails@v2.15.0 build -platform darwin/arm64

# Windows（必须在 Windows 上运行）
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build.ps1
# 额外运行截图与系统事件原生 smoke
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build.ps1 -RunSmoke

# Linux（scripts/build-linux.sh 会自动选择 WebKit2GTK ABI tag）
../scripts/build-linux.sh
# 或者
go run github.com/wailsapp/wails/v2/cmd/wails@v2.15.0 build -platform linux/amd64 \
    -tags "$(scripts/dev-linux.sh --print-tag 2>/dev/null || echo webkit2_41)"
```

产物分别位于 `build/bin/Daygo.app`、`build/bin/Daygo.exe`、`build/bin/Daygo`。
`wails` 命令需在 `cmd/daygo` 目录执行，它会自动解析仓库根目录下的 `frontend/` 和 `build/`，并通过
`preBuildHooks` 构建原生静态库——**目前仅 darwin/windows 触发 preBuildHooks，Linux 的 `native/` 适配器
是「待定设计」（docs/09 §9.8 #1）。**

### 三平台的功能差异

`internal/platform` 的端口层让未实现能力在 Linux 上明确返回 `unsupported`，
已实现的 Secrets 则经 Secret Service 提供。因此 Wails 桌面壳可启动并显示 Vue 前端，
但原生能力仍有显著差异：

| 能力                | macOS | Windows | Linux（当前） |
|--------------------|:-----:|:-------:|:------:|
| 屏幕截图            | ✅ ScreenCaptureKit | ⚠️ DXGI / WGC（实验，限有限 smoke） | ❌ 未实现，返回 `CaptureUnsupported` |
| 系统授权 / TCC      | ✅ | ⚠️ 部分 | ❌ 未实现 |
| 状态栏 / 系统托盘  | ✅ | ⚠️ 部分 | ❌ 未实现 |
| 钥匙串 / 凭据      | ✅ `security` 子进程 | ✅ Credential Manager | ⚠️ Secret Service / `secret-tool`（待真机验收） |
| 启动项 / 激活策略   | ✅ | ⚠️ 部分 | ❌ 未实现 |
| 系统事件订阅        | ✅ | ⚠️ 部分 | ❌ 未实现 |
| SQLite + 设置 + 时间线前端 | ✅ | ✅ | ✅（与 macOS 完全一致） |

**目标平台仍是 macOS。** Linux 与 Windows 上的完整功能矩阵属于 docs/09 §9.8 中的待定设计，
实现前需要各自独立的决策记录。在那之前，Linux 与 Windows 上能做的有：浏览已存在的数据、设置、AI
provider 配置与连接测试、对话——这些都走纯 Go Core，不依赖平台适配。

发布链路（签名、公证、自动更新）尚未建立。

## 参与贡献

实现工作按 [功能模块路线](docs/09-roadmap.md) 和对应的模块执行册推进；模块可以并行开发，按具体能力的依赖接入，并遵守 [docs/07-privacy-security.md](docs/07-privacy-security.md) 的隐私约束。开始修改前请阅读 [AGENTS.md](AGENTS.md)。

计划进行较大改动时，请先创建 Issue，并说明改动所属的功能模块、共享能力影响及其验证门禁。
