<p align="center">
  <img src="docs/assets/readme/banner.en.webp" width="100%" alt="Daygo — 一天结束，你还记得自己做了什么吗？" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><img src="https://img.shields.io/github/v/release/Jwz-git/Daygo?style=flat-square&color=F3854B&label=%E7%89%88%E6%9C%AC" alt="最新正式版本" /></a>
  <img src="https://img.shields.io/badge/macOS-14%2B-333333?style=flat-square&logo=apple&logoColor=white" alt="macOS 14+" />
  <img src="https://img.shields.io/badge/Windows-11%2024H2%2B-0078D4?style=flat-square" alt="Windows 11 24H2+" />
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-6E7DF7?style=flat-square" alt="MIT License" /></a>
  <img src="https://img.shields.io/badge/Go%20%C2%B7%20Wails%20%C2%B7%20Vue%203-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go · Wails · Vue 3" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><b>下载</b></a> ·
  <a href="https://jwz-git.github.io/Daygo/"><b>官网</b></a> ·
  <a href="docs/README.md">设计文档</a>
</p>

<p align="center">
  <strong>简体中文</strong> · <a href="readme/README.zh-Hant.md">繁體中文</a> · <a href="readme/README.en.md">English</a> · <a href="readme/README.ja.md">日本語</a> · <a href="readme/README.ko.md">한국어</a> · <a href="readme/README.de.md">Deutsch</a> · <a href="readme/README.fr.md">Français</a> · <a href="readme/README.es.md">Español</a> · <a href="readme/README.pt-BR.md">Português (Brasil)</a>
</p>

<p align="center"><strong>记录屏幕上的工作，回顾自己的每一天。</strong></p>

Daygo 是面向 macOS 与 Windows 的工作记录与复盘工具。它在后台按间隔截图，用你配置的 AI 将屏幕活动整理成**时间线、每日回顾和每周总结**，为站会、复盘和回忆工作细节提供记录。

**离散截图 · 本地存储 · 自选 AI · 九种界面语言**

[下载与安装](#下载与安装) · [快速上手](#快速上手) · [功能预览](#功能预览) · [隐私与数据](#隐私与数据) · [参与开发](#参与开发)

## 下载与安装

在 [最新正式版的 Assets](https://github.com/Jwz-git/Daygo/releases/latest) 中选择对应安装包：

| 平台 | 系统与架构 | 安装包 |
|---|---|---|
| macOS | macOS 14+，Apple Silicon（arm64） | `Daygo-<版本>-arm64.dmg` |
| Windows | Windows 11 24H2+（build 26100+），x64（amd64） | `Daygo-<版本>-amd64-installer.exe` |

macOS 为主线开发平台，Windows 提供 x64 安装包；其他架构与 Linux 暂无发布安装包。源码进度可能领先于发布版，功能状态见 [模块总表](docs/09-roadmap.md#91-模块总表)，签名、公证与安装验证记录见 [安装与更新](docs/modules/delivery.md)。

## 快速上手

1. **安装并授权**：启动 Daygo；macOS 按提示授予「屏幕录制」权限，并在应用提示时重启。
2. **配置 AI**：在设置中添加服务地址、API Key 和模型，保存后打开「模型测试与试用」，用文字或一张图片检查回复。
3. **设置记录方式**：选择截图间隔、要屏蔽的应用和磁盘占用上限，再启用录制。
4. **查看结果**：等待首批分析完成，在时间线中查看卡片与原始帧，按需编辑；到每日、每周页面回顾活动。

Daygo 不附带 AI 服务或额度。自动分析需要图像识别与对应协议的结构化输出能力；试用页收到回复不代表自动分析能力已经验证。本机模型也需满足这些要求。

## 功能预览

<sub>以下截图使用匿名示例数据。</sub>

### 自动时间线

按间隔捕获系统主显示器，AI 将活动整理成带有时间、标题、摘要和分类的卡片。展开卡片可查看原始帧，也能编辑、删除或重新处理结果。

<img src="docs/assets/readme/timeline.en.webp" width="100%" alt="时间线：按时段排列的活动卡片与详情面板" />

### 每日回顾

查看一天的工作流，生成包含亮点、完成项和阻塞项的站会摘要；记录日记和每日目标，为复盘补充自己的想法。

<img src="docs/assets/readme/daily.en.webp" width="100%" alt="每日回顾：工作流概览与站会摘要" />

### 每周回顾

从周工作流、专注与分心热力图、分类占比、常用应用和时间流向回顾一周。跟踪时长统计排除 System 分类。

<img src="docs/assets/readme/weekly.en.webp" width="100%" alt="每周回顾：分类常用应用与时间流向图" />

### 后台常驻与自定义

| 能力 | 说明 |
|---|---|
| 后台记录 | 关闭窗口后继续录制，从 macOS 状态栏或 Windows 通知区重新打开 |
| 暂停与恢复 | 可暂停 15 / 30 / 60 分钟或一直暂停；定时暂停到期自动恢复 |
| 系统事件 | 睡眠、锁屏、屏保期间暂停捕获；用户主动关闭录制后不会被系统事件重新开启 |
| 截图设置 | 间隔 1 / 5 / 10 / 20 / 30 / 60 秒，默认 10 秒；高度 720 / 1080 像素，默认 1080 |
| AI 服务 | OpenAI Chat Completions、OpenAI Responses、Anthropic Messages；单服务多模型与有序回退链 |
| 外观与分类 | 浅色 / 深色 / 跟随系统；分类名称、顺序、配色可编辑；可设置开机自启及 macOS Dock 图标 |

界面支持简体中文、繁体中文、英文、日文、韩文、德文、法文、西班牙文和巴西葡萄牙文。

<details>
<summary>查看深色模式</summary>

<p><img src="docs/assets/readme/dark.en.webp" width="100%" alt="Daygo 深色模式" /></p>

</details>

时间线、日记和目标以本地凌晨 **4 点**划分一天；站会摘要按日历日聚合。深夜活动的归属可能因此不同。

## 隐私与数据

- **本地存储**：截图、时间线、日记、设置与数据库存放在你的设备上，Daygo 没有自有后端、账号或同步服务。
- **由你选择数据去向**：屏幕数据离开设备的唯一路径是你明确配置的 AI 服务。使用兼容的本机模型时，屏幕分析可在本机完成；第三方服务的数据处理遵循其隐私政策。
- **应用屏蔽与前台脱敏**：从截图中排除被屏蔽应用；当前台应用被屏蔽时，保存脱敏占位帧。两层保护同时保留。
- **系统凭据存储**：API Key 只存 macOS 钥匙串或 Windows Credential Manager，界面只写不读，不进入项目数据库、localStorage 或错误信息。

记录使用离散截图，避免持续开启屏幕录制流。使用分析与崩溃上报默认关闭，上报消费者尚未实现。

卸载应用会保留用户数据与凭据。完整的数据边界见 [隐私与安全](docs/07-privacy-security.md)。

## 参与开发

Go 负责业务逻辑与数据库写入，平台能力通过接口隔离，Vue 通过生成的 Wails 绑定访问 Go。`test` 是日常开发分支，`main` 是稳定分支。

macOS 开发需安装 Go、Node.js/npm 和 Xcode Command Line Tools；Go 版本要求见 [go.mod](go.mod)，Node.js 要求见 [开发入口](scripts/dev.sh)。

```bash
git clone --branch test https://github.com/Jwz-git/Daygo.git
cd Daygo
./scripts/gate.sh   # 引导、构建、Go / 前端测试与文档检查
```

门禁会按顺序准备前端产物与 Wails 绑定。各平台开发命令见 [脚本入口](scripts/README.md)，设计与贡献约定见 [设计文档](docs/README.md) 和 [AGENTS.md](AGENTS.md)。

反馈问题请提交 [Issue](https://github.com/Jwz-git/Daygo/issues)，附上系统、应用版本、复现步骤与脱敏错误信息；请勿上传真实截图、数据库或 API Key。

## 许可证

本项目基于 [MIT License](LICENSE) 开源。

<sub>由 <a href="https://github.com/JerryZLiu/Dayflow">Dayflow</a> 启发（MIT，© 2025 Jerry Liu）。</sub>
