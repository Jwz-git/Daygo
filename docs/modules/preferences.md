# preferences — 应用偏好

## 用户结果与范围

用户可切换浅色 / 深色 / 系统主题和语言，刷新、重启后偏好保持；各设置分区有一致的入口。
负责 F-V6/7、前端外壳与通用设置访问，不等待所有功能设置一次完成。
录制 / 隐私 / 自启 / Dock 归 recording，Provider / 输出语言归 providers，提醒归 daily，
磁盘 / 遥测归 data，更新归 delivery；本模块不实现这些字段的产品逻辑。

依据：[02 前端约定](../02-architecture.md#25-前端约定)、
[05 设置契约](../05-interface-contract.md#设置与分类)、
[05 前端规则](../05-interface-contract.md#555-前端侧规则)。

## 当前状态与证据

> **验收状态**：已实现能力于 2026-09-22 经用户确认已验收；无逐项运行记录。未实现能力见 [09 §9.1](../09-roadmap.md#91-模块总表)。

实现进度：**部分实现**。settings-access、前端外壳、主题 / 语言设置和设置页面已落盘；外观设置已优先通过 `GetSettings` / `UpdateSettings` 接管，浏览器预览仍保留安全的 localStorage 回退。

已交付：

- `internal/settings`：19 个设置键的类型化读写、默认值、规范化与夹取、`Patch` 语义
  （nil = 本次不改）、跨键规则（`llm.outputLanguage` 与 `appearance.language` 相互独立、
  空串语言保留为"跟随系统"哨兵）。只经 `storage.SettingsRepo` 读写，不含 SQL。
- `internal/app`：`GetSettings` / `UpdateSettings` 绑定与 `SettingsDTO` / `SettingsPatchDTO`；
  `UpdateSettings` 返回生效后的完整设置，`settings:changed` 只带改动键名且仅在提交后发出。
  事件经可注入的 `EventEmitter` 发布，绑定层测试不需要 Wails runtime。
- `frontend`：路由、分组侧栏、浅 / 深 / 跟随系统主题、双语切换和按用户任务重组的设置分区；
  设置分区写入查询参数，可从其他功能深链进入。界面使用 macOS 系统字体与低饱和蓝色强调层级，
  录制状态及控制已提升至全局外壳，截图测试仅从开发环境的存储与诊断分区进入。
- Windows 外壳仅在该平台启用保留系统 resize / Aero decorations 的 frameless 窗口，并由前端
  提供 36px 顶栏：左侧只显示 Daygo 图标，右侧提供最小化、最大化 / 还原和“关闭即隐藏”控制；
  macOS 的隐藏标题栏和 Linux 原生窗口行为不变。

未交付：前端所有 DTO 的生成类型替换、统一错误模型、localStorage 全量接管迁移，以及尚无产品消费者的启动项 / Dock / 遥测行为接入。
`frontend/src/api/dto.ts` 仍保留时间线 / Provider 等尚未生成绑定的手写类型；主题 / 语言在 Wails 内以 SQLite 为权威来源，只有无桥预览使用 localStorage。模型输出语言、识别增强和存储上限已通过生成绑定接入设置页。

**绑定面已收口**：`SetEventEmitter` 与 `Store` 原本是包内装配用的导出方法，被 Wails 当成
绑定导出到 `frontend/wailsjs/go/app/Backend.d.ts`（`Store` 还把 `storage.Store` 拉进了生成的
`models.ts`）。两者已改为非导出，正式契约生成产物现在对应 [05 §5.2.1](../05-interface-contract.md#521-按功能能力的可用性)；
临时截图联调绑定另列于 recording，不属于设置页面契约。接 ui-bridge 时沿用这条规则：**绑定对象上的导出方法就是前端 API**，
装配用的入口一律非导出，`internal/app/bindings_test.go` 会在两者不一致时失败。

配置的**产品逻辑**不在本模块：录制 / 隐私 / 自启 / Dock 归 recording，
Provider / 输出语言归 providers，提醒归 daily，磁盘 / 遥测归 data。
`internal/settings` 只回答"这个设置是什么、什么值有效"。

## 能力与跨层职责

| 输入 | 可独立推进 | 真实接入条件 |
|---|---|---|
| data: settings-store | 已完成；`internal/settings` 在其上做类型化访问 | 已就绪 |
| 05 DTO / 事件 / 错误契约 | 搭前端测试运行器、生成类型消费与 wrapper fixture | 真实绑定存在；不将未实现方法补成假成功 |
| 各功能设置定义 | 页面容器、键级 patch 和事件分发 | 功能负责方的字段规则与真实能力验收 |

输出 settings-access 与 ui-bridge；GetCapabilities 公共接入归本模块协调，
锁状态由 data、功能可用性由真实实现供给，不能按开发模块清单机械增加 feature。
internal/settings 提供类型化读写，底层只调用 data 的 repository；
internal/app 拥有 Get/UpdateSettings 和 DTO；store / api 拥有取数与事件，组件只负责交互。
用户可见文本经 i18n；API key 永不进入通用设置或 localStorage。


## 实验与失败条件

| 实验 | 输入与操作 | 预期结果 | 失败条件 / 证据 |
|---|---|---|---|
| 主题 / 语言 | 三态主题、系统变化、中文 / 英文 / 跟随系统，刷新 / 重启 | DOM 解析值和保存偏好正确，缺 key 类型失败 | 闪回旧偏好、跟随失效、文案绕过 i18n 失败 |
| settings patch | 未提供、置空、越界值，按 05 调用 | 只改显式键，规范化后的值持久化，事件带键名 | 擅自覆盖其他功能设置或绕过规范失败 |
| localStorage 接管 | 有效旧记录、错误信封、首次入库失败、重启读回 | 按功能切换单一来源，确认新值后停旧写，失败可恢复 | 双写、旧值丢失、密钥落盘失败 |
| DTO / 错误 / 事件 | null、未知枚举、错误码、写操作与失效事件 | 生成 DTO 为类型来源，unknown 显式解析，写后事件重拉 | any 跨界、乐观写入、组件直连绑定失败 |

## 实现切片与集成

1. 为现有主题 / 语言 / localStorage 行为保留最小必要夹具与单元运行器，沿用 npm 与现有锁文件；不为改测试工具链额外创建锁文件。
2. 以 settings-store fake 实现类型化访问与 patch；data repository 已就绪。
3. 接生成绑定与 DTO、薄 wrapper、统一错误和事件消费；设置页已消费生成的 `SettingsDTO`，未实现的功能仍保持占位。
4. 迁移外观 / 语言的存储来源，先通过一次 `UpdateSettings` 成功返回后删除旧 localStorage；数据库不可用时保留本地预览值且停止写入，避免双写。
5. 接入模型输出语言、识别增强和存储上限的低风险设置 UI；启动项 / Dock / 遥测须等各自真实消费者就绪后再开放开关。
6. 验收偏好闭环与双语言状态；新增大规模界面仍受 G-host 约束。

## 验收、阻塞与回退

完成要求：外观 / 语言真实持久化、生成绑定和 settings-access / ui-bridge 契约通过，
不需要等所有设置分区完成。单元与类型检查不代替实际重启交互。
apiRevision 的生产检查由 preferences 工程在版本不一致处理实现前决定，保持现有 wire 字段。

db-core 未就绪可推进纯设置和 wrapper fixture；G-host 不阻止维护已有外壳与必要验证界面。
回退：按分区恢复旧 store 接入，保留未清理的旧偏好及新库，明确恢复哪一个单一来源；
不能靠全清 localStorage 或把密钥写到本地偏好恢复状态。

## 验证记录

2026-09-23：补回 Windows 专用 `Frameless` 宿主配置，避免原生黑色标题栏与 Vue 顶栏同时显示。
保留系统缩放边框、阴影及圆角，macOS / Linux 窗口策略不变。新增平台窗口配置回归测试；
`CGO_ENABLED=0 go test ./internal/app`、`go vet ./internal/app`、Windows `go build ./...`、
Linux / macOS `go build ./internal/app` 和 `python scripts/check-docs.py` 通过。
真实窗口视觉、拖动、缩放和 DPI 仍需重启新版应用后验收。

2026-09-10—12：前端 typecheck、unit、build 与 Vite 预览覆盖设置分区、深链、外观 / 语言、录制上限和诊断展示。真实 Wails 写入、重启及浅色 / 英文矩阵由用户于 2026-09-22 确认验收，未附逐项运行记录。

| 日期 / commit / 环境 | 命令或人工步骤 / 输入 | 期望与实际结果 | 限制 / 下一步 |
|---|---|---|---|
| 2026-09-14 / 当前工作树 / Windows 11 amd64 | `CGO_ENABLED=0 go test ./...`、`go vet ./...`、`CGO_ENABLED=0 go build ./...`、`npm --prefix frontend ci`、前端 typecheck / 31 项 unit / build、`git diff --check` | 通过；Windows frameless 配置、平台解析和自绘标题栏均可编译，浏览器生产 bundle 生成成功 | 真实 Wails 窗口的拖动、缩放、DPI 和关闭隐藏仍需人工验收 |
