<p align="center">
  <img src="docs/assets/readme/banner.en.webp" width="100%" alt="Daygo — 一天结束，你还记得自己做了什么吗？" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><img src="https://img.shields.io/github/v/release/Jwz-git/Daygo?style=flat-square&color=F3854B&label=%E7%89%88%E6%9C%AC" alt="最新正式版本" /></a>
  <img src="https://img.shields.io/badge/macOS-14%2B-333333?style=flat-square&logo=apple&logoColor=white" alt="macOS 14+" />
  <img src="https://img.shields.io/badge/Windows-11%2024H2%2B-0078D4?style=flat-square&logo=windows&logoColor=white" alt="Windows 11 24H2+" />
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-6E7DF7?style=flat-square" alt="MIT License" /></a>
  <img src="https://img.shields.io/badge/Go%20%C2%B7%20Wails%203%20%C2%B7%20Vue%203-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go · Wails 3 · Vue 3" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><b>下载安装包</b></a> ·
  <a href="https://jwz-git.github.io/Daygo/"><b>官方网站</b></a> ·
  <a href="docs/README.md">设计文档</a>
</p>

<p align="center">
  <strong>简体中文</strong> · <a href="readme/README.zh-Hant.md">繁體中文</a> · <a href="readme/README.en.md">English</a> · <a href="readme/README.ja.md">日本語</a> · <a href="readme/README.ko.md">한국어</a> · <a href="readme/README.de.md">Deutsch</a> · <a href="readme/README.fr.md">Français</a> · <a href="readme/README.es.md">Español</a> · <a href="readme/README.pt-BR.md">Português (Brasil)</a>
</p>

<p align="center"><strong>零手工打卡，自动记录屏幕工作，回头看得清每一天。</strong></p>

Daygo 是面向 **macOS** 与 **Windows** 的本地优先工作记录与复盘工具。它在后台按间隔离散截取主显示器，交由你自选的 AI 整理成**自动时间线、每日站会复盘、每周回顾与计划提醒**——无需启停计时器，也不依赖第三方云端账号。

<p align="center"><b>离散截图 · 本地存储 · 自选 AI（含本机模型） · 九种界面语言</b></p>

<p align="center">
  <a href="#核心特性">核心特性</a> ·
  <a href="#下载与安装">下载与安装</a> ·
  <a href="#快速上手">快速上手</a> ·
  <a href="#隐私与安全">隐私与安全</a> ·
  <a href="#参与开发">参与开发</a>
</p>

---

## 核心特性

<sub>以下截图均使用匿名示例数据。</sub>

### 自动时间线

后台离散捕获系统主显示器，AI 将活动整理为带有时间段、标题、摘要与分类的卡片。展开任意卡片可回放原始帧，支持日 / 周视图切换，也能随时编辑、删除、单卡或按日重处理、一键清理当日失败批次，并在右侧检查器安排今日计划块。

<img src="docs/assets/readme/timeline.en.webp" width="100%" alt="自动时间线：按时段排列的活动卡片与详情面板" />

### 每日复盘

一屏查看当日工作流，自动生成包含亮点、完成项与阻塞项的站会摘要；搭配日记与每日目标沉淀思考。支持开启每日日记提醒通知，并可按需在页尾展开 Token 用量图表（默认关闭）。

<img src="docs/assets/readme/daily.en.webp" width="100%" alt="每日复盘：工作流概览与站会摘要" />

### 每周回顾

通过周工作流、专注与分心热力图、分类占比、常用应用与时间流向回顾整周节奏，同样支持按需显示 Token 用量图表（跟踪时长统计排除 `System` 分类）。

<img src="docs/assets/readme/weekly.en.webp" width="100%" alt="每周回顾：分类常用应用与时间流向图" />

<details>
<summary>查看深色模式</summary>

<p><img src="docs/assets/readme/dark.en.webp" width="100%" alt="Daygo 深色模式预览" /></p>

</details>

### 后台常驻与自定义

| 能力 | 说明 |
|---|---|
| 后台常驻 | 关闭主窗口后继续录制（macOS 同步隐藏 Dock 运行标记），随时从 macOS 状态栏或 Windows 通知区唤回 |
| 暂停与系统事件 | 支持暂停 15 / 30 / 60 分钟或一直暂停，定时暂停到期自动恢复；睡眠、锁屏、屏保自动暂停；手动关闭录制后不会被系统事件重启 |
| 截图与存储 | 间隔 1 / 5 / 10 / 20 / 30 / 60 秒（默认 10 秒）；高度 720 / 1080 px（默认 1080）；支持磁盘占用上限与整段清理，Windows 支持迁移录制目录 |
| 多协议 AI 与回退 | API Key 支持 OpenAI Chat Completions、OpenAI Responses、Anthropic Messages、Google Gemini（符合条件的完整已收尾 MP4 分段优先直传，否则回退截图）；另支持 ChatGPT 订阅授权；单服务多模型与有序回退链 |
| 计划与系统通知 | 支持今日计划时间块与完成标记；macOS / Windows 支持每日日记提醒、计划开始与分心系统通知 |
| 外观、分类与语言 | 浅色 / 深色 / 跟随系统；首次引导内置职业分类预设，分类名称、顺序与配色可自由编辑；支持开机自启与 9 种界面语言 |

> **关于凌晨 4 点逻辑日**：时间线、计划、日记与目标按本地时间凌晨 **4:00** 划分一天，站会摘要按日历日聚合，深夜工作的归属可能因此不同。

---

## 下载与安装

前往 [最新正式版 Releases](https://github.com/Jwz-git/Daygo/releases/latest) 下载对应安装包：

| 平台 | 系统要求与架构 | 安装包 |
|---|---|---|
| macOS | macOS 14+，Apple Silicon（`arm64`） | `Daygo-<版本>-arm64.dmg` |
| Windows | Windows 11 24H2+（build 26100+），x64（`amd64`） | `Daygo-<版本>-amd64-installer.exe` |

macOS 为主线开发平台，Windows 提供 x64 安装包；其他架构与 Linux 暂未提供发布安装包。仓库源码进度可能领先于发布版，各模块实现与验收状态见 [模块总表](docs/09-roadmap.md#91-模块总表)，签名、公证与安装更新细节见 [安装与更新](docs/modules/delivery.md)。

---

## 快速上手

1. **安装并完成引导**：启动 Daygo，按首次引导选择职业分类预设与基础偏好；macOS 按提示授予「屏幕录制」权限，并在应用提示时重启生效。
2. **配置 AI 服务**：在引导或设置中选择 **ChatGPT 订阅**（通过浏览器完成授权）或 **配置 API Key**（填写地址、密钥并手填或拉取多选模型）；保存后可在「模型测试与试用」用文字或单张图片验证回复。
3. **设置记录与隐私**：按需调整截图间隔、应用屏蔽名单与磁盘占用上限（Windows 支持迁移录制目录），开启录制。
4. **回顾与整理**：等待首批分析生成卡片，在时间线查看活动与原始帧，按需编辑或重处理；前往每日、每周页面复盘。

> Daygo 不附带任何 AI 服务或额度。自动时间线分析需要模型具备视觉理解与对应协议的结构化输出能力（本机模型亦然）；试用页能正常回复不代表自动分析能力已全部满足。

---

## 隐私与安全

- **全部数据留在本地**：截图分段、时间线、日记、计划、设置与 SQLite 数据库均存放在你的设备上。Daygo 没有自有后端、账号系统或云端同步。
- **去向由你决定**：屏幕数据离开设备的唯一路径是你明确配置的 AI 服务。接入兼容的本机模型时分析可完全在本地完成；使用第三方服务时遵循对应服务商的隐私政策。
- **应用屏蔽 + 前台脱敏双保护**：被屏蔽应用从截图中排除；当被屏蔽应用处于前台时，Daygo 仅保存脱敏占位帧。两层保护始终同时生效。
- **系统凭据存储，只写不读**：API Key 与 ChatGPT 订阅授权凭据仅存系统凭据库（macOS 钥匙串 / Windows Credential Manager），界面与绑定层永不读取凭据明文，也不进入数据库、`localStorage` 或错误信息。

记录采用**离散截图**而非连续屏幕录制流。使用分析与崩溃上报默认关闭（上报消费者尚未实现）。卸载应用不会删除你的用户数据与凭据。完整数据与安全边界见 [隐私与安全](docs/07-privacy-security.md)。

---

## 参与开发

日常开发分支 `test` 基于 **Go 1.26.10 + Wails v3.1.0 + Vue 3**，`main` 为稳定分支（迁移范围与验收边界见 [Wails v3 GA 迁移记录](docs/decisions/wails-v3-migration.md)）。Go 负责业务逻辑与 SQLite 写入，平台能力经接口隔离，Vue 通过生成的 TypeScript 绑定与 Go 通信。

### 环境要求

- [go.mod](go.mod) 指定的 Go 版本
- Node.js `20.19+`（20.x）或 `22.12+`（含 npm）
- **macOS**：Xcode Command Line Tools
- **Windows**：另需 MinGW-w64（含 binutils）、Visual Studio 2022 C++ 工具、Windows SDK 26100 及 WebView2 Runtime，详见 [Windows 构建依赖](scripts/README.md#windows-build-dependencies)
- **Linux**：默认使用 GTK4 与 WebKitGTK 6.0 开发包（arm64 宿主构建已验证，截图与系统集成尚待实现）

### 克隆、验证与运行

```bash
git clone --branch test https://github.com/Jwz-git/Daygo.git
cd Daygo
./scripts/gate.sh   # 干净检出自动引导、构建、Go / 前端单测与文档检查
```

| 平台 | 启动开发应用 |
|---|---|
| macOS | `./scripts/dev.sh` |
| Windows | `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dev.ps1` |
| Linux | `./scripts/dev-linux.sh` |

更多构建、打包与脚本说明见 [scripts/README.md](scripts/README.md)，设计规格与工程约定见 [设计文档](docs/README.md) 与 [AGENTS.md](AGENTS.md)。

提交问题请前往 [Issues](https://github.com/Jwz-git/Daygo/issues)，并附上操作系统、应用版本、复现步骤与脱敏错误信息；**请勿上传真实截图、数据库或 API Key**。

---

## 后续计划

- 插件功能开发
- Linux 截图与系统集成

---

## 许可证

本项目基于 [MIT License](LICENSE) 开源。

<sub>受 <a href="https://github.com/JerryZLiu/Dayflow">Dayflow</a> 启发（MIT，© 2025 Jerry Liu）。</sub>
