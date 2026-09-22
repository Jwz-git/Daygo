# Daygo

**简体中文** · [English](README.en.md)

**让一天自然发生，让记录自己完成。**

Daygo 是一款本地优先、注重隐私的个人工作记录应用。它在后台进行离散截图，用你配置的 AI
把零散活动整理成时间线、每日回顾和每周总结，让写代码、查资料、开会、沟通和思考不再随着
窗口关闭而消失。

> 当前版本：**v0.1.0 预发布版** · 支持 macOS 与 Windows · 需要自备 AI Provider

## 下载

从 [v0.1.0 Release 页面](https://github.com/Jwz-git/Daygo/releases/tag/v0.1.0) 下载适合你的安装包：

| 平台 | 系统与架构 | 安装包 |
|---|---|---|
| macOS | macOS 14+，Apple Silicon（arm64） | [`Daygo-0.1.0-arm64.dmg`](https://github.com/Jwz-git/Daygo/releases/download/v0.1.0/Daygo-0.1.0-arm64.dmg) |
| Windows | Windows 11，x64（amd64） | [`Daygo-0.1.0-amd64.exe`](https://github.com/Jwz-git/Daygo/releases/download/v0.1.0/Daygo-0.1.0-amd64.exe) |

这是早期预发布版本。GitHub Actions 已实现发布后自动构建安装包；签名、公证、应用内自动更新、完整安装升级矩阵和长期稳定性验证仍在完善；系统
可能显示未知开发者或安全提示。请只从本仓库的 Releases 页面下载安装包，并在试用前保留重要
数据的独立备份。其他架构和 Linux 暂无可下载版本。

## Daygo 能做什么

- **自动记录工作上下文**：在后台按间隔进行离散截图，无需手动启动计时器。
- **生成可回看的时间线**：AI 将活动整理为带时间、标题、摘要和分类的卡片，并关联原始画面。
- **沉淀每日与每周回顾**：从已有记录查看一天的进展和一周的时间分布、分类占比与趋势。
- **保留编辑控制权**：修改卡片标题、摘要和分类，删除不需要的内容，或重新处理指定时段。
- **接入自己的 AI**：支持 OpenAI Chat Completions、OpenAI Responses 和 Anthropic 兼容协议，
  可配置多个模型与有序回退链。
- **适应你的工作方式**：支持中英文界面、浅色/深色主题、录制间隔、屏蔽应用和磁盘上限设置。

## 隐私边界

Daygo 处理的是高度私密的屏幕信息，因此隐私是产品边界，而不是附加选项。

- 截图、时间线、日记和数据库默认保存在你的电脑上。
- Daygo 没有自有后端，也不提供或默认选择 AI Provider。
- 屏幕数据只会发送给你明确配置的 Provider，也可以连接兼容协议的本地模型。
- 可以屏蔽指定应用；被屏蔽应用位于前台时，Daygo 写入脱敏占位帧。
- API Key 只存入系统凭据存储，不进入前端 localStorage 或项目数据库。
- 分析与崩溃上报默认关闭，且不得包含屏幕内容、窗口标题、文件路径、API Key 或 LLM 请求内容。

完整边界见[隐私与安全设计](docs/07-privacy-security.md)。

## 首次使用

1. 安装并启动 Daygo。
2. macOS 用户按系统提示授予“屏幕录制”权限；授权后可能需要重启 Daygo。
3. 在设置中添加 AI Provider、API Key 和模型，然后运行连接测试。
4. 设置截图间隔、屏蔽应用和存储上限，再开始录制。
5. 首批分析完成后，在时间线中检查结果并按需编辑。

Daygo 不附带 AI 服务或 API 额度。发送到第三方 Provider 的数据如何处理，取决于你选择的服务
及其隐私政策。

## 当前阶段

v0.1.0 已提供 macOS 和 Windows 安装包，但“发布了安装包”不等于全部功能已经完成真实用户闭环
验收。当前仍在补充真实 Provider、关窗后台常驻、隐私双保护、安装升级以及 7/14 天长期运行等
证据。已实现范围、验证记录和剩余风险以[路线图模块总表](docs/09-roadmap.md#91-模块总表)为准。

遇到问题时，请提交包含系统版本、Daygo 版本、复现步骤和预期/实际结果的
[Issue](https://github.com/Jwz-git/Daygo/issues)。请勿附带真实截图、API Key、数据库、录制文件、
窗口标题或其他敏感信息。

## 从源码运行

需要 Go 1.25+、Node.js 20.19+（或 22.12+）和 npm。macOS 还需要 Xcode Command Line Tools；
Windows 原生构建依赖见[脚本说明](scripts/README.md)。

```bash
git clone https://github.com/Jwz-git/Daygo.git
cd Daygo

# macOS
./scripts/dev.sh

# Windows PowerShell
./scripts/dev.ps1
```

## 参与开发

Daygo 使用 Go、Wails、Vue 3、TypeScript、Pinia 和 SQLite。Go Core 持有业务逻辑与唯一数据库
写入权；系统能力通过平台端口隔离；前端只通过生成的 Wails 绑定访问 Go。

```text
Vue 3 + TypeScript
        ↓ Wails bindings
Go services and foundation
        ↓ platform ports
macOS / Windows native adapters
```

提交前运行完整门禁：

```bash
./scripts/gate.sh
```

进一步阅读：[设计文档](docs/README.md) · [测试策略](docs/08-testing-strategy.md) ·
[贡献约束](AGENTS.md)

## 许可证

本项目基于 [MIT License](LICENSE) 开源。

## 致谢

产品创意受 [Dayflow](https://github.com/JerryZLiu/Dayflow) 等项目启发。
