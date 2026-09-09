# Daygo

**简体中文** · [English](README.en.md)

Daygo 是一款面向 macOS 的隐私优先、本地优先工作日志。它定时采集屏幕活动，使用用户选择的 AI 服务理解工作内容，并将结果整理为可检索的每日时间线、站会摘要和复盘记录。

> 项目状态：Daygo 正在以 Go Core + Wails + Vue 构建。目前落盘的只有桌面外壳与前端页面骨架，尚未达到公开安装条件。

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
平台适配层（实现待定设计）
  └── 屏幕捕获、系统授权、钥匙串、状态栏、自动更新
```

Go 拥有全部可移植业务逻辑，并且是 SQLite 的唯一写入方。需要 macOS 系统能力的部分收拢在一组端口后面，**实现方式尚未选定**——文档只定义任何方案都必须满足的契约。这条边界让 Go Core 能在无 macOS 环境下构建与测试（`CGO_ENABLED=0`），整套测试策略以此为前提。

完整的需求、接口、数据模型、测试策略、里程碑与风险见[设计文档](docs/README.md)。

## 当前仓库结构

```text
cmd/daygo/                  Go 命令入口与 wails.json
internal/                   Go Core（当前仅 app 外壳）
frontend/                   Vue 3 + TypeScript 前端
build/                      Wails 构建资源与产物
testdata/                   夹具与参考数据库（尚未落盘）
docs/                       设计文档
```

`docs/` 中描述的多数 Go 目录和接口仍属于目标状态，尚未落盘。**当前已落盘的只有 Wails 外壳、前端页面骨架和设置页**，不含存储、分析、AI、平台适配或后台生命周期。开发路线见 [docs/09-roadmap.md](docs/09-roadmap.md)。

## 构建与运行

环境要求：macOS 14+、Go 1.25+、Node.js 20.19+（或 22.12+）、npm。

```bash
git clone https://github.com/Jwz-git/Daygo.git
cd Daygo

# 前端依赖与构建
npm --prefix frontend ci
npm --prefix frontend run build

# Go 检查
go test ./...
go vet ./...

# 构建 macOS 应用（wails.json 位于入口目录）
cd cmd/daygo
go run github.com/wailsapp/wails/v2/cmd/wails@v2.15.0 build -platform darwin/arm64
```

产物位于 `build/bin/Daygo.app`。首次克隆后必须先构建前端，否则 `frontend/dist` 不存在会导致 Go 构建失败。`wails` 命令需在 `cmd/daygo` 目录执行，它会自动解析仓库根目录下的 `frontend/` 和 `build/`。

## 参与贡献

实现工作应遵循 [docs/09-roadmap.md](docs/09-roadmap.md) 的里程碑顺序，并遵守 [docs/07-privacy-security.md](docs/07-privacy-security.md) 的隐私约束。开始修改前请阅读 [AGENTS.md](AGENTS.md)。

计划进行较大改动时，请先创建 Issue，并说明改动所属的里程碑及其验证门禁。
