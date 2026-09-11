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

实现进度：**部分实现**。settings-access 已落盘，前端接入未开始。

已交付：

- `internal/settings`：15 个设置键的类型化读写、默认值、规范化与夹取、`Patch` 语义
  （nil = 本次不改）、跨键规则（`llm.outputLanguage` 与 `appearance.language` 相互独立、
  空串语言保留为"跟随系统"哨兵）。只经 `storage.SettingsRepo` 读写，不含 SQL。
- `internal/app`：`GetSettings` / `UpdateSettings` 绑定与 `SettingsDTO` / `SettingsPatchDTO`；
  `UpdateSettings` 返回生效后的完整设置，`settings:changed` 只带改动键名且仅在提交后发出。
  事件经可注入的 `EventEmitter` 发布，绑定层测试不需要 Wails runtime。

未交付：前端生成绑定与 wrapper、错误解析、前端单元测试运行器、localStorage 接管迁移。
`api/dto.ts` 仍是手写子集。页面容器已有，但设置尚未经绑定持久化。

**绑定面已收口**：`SetEventEmitter` 与 `Store` 原本是包内装配用的导出方法，被 Wails 当成
绑定导出到 `frontend/wailsjs/go/app/Backend.d.ts`（`Store` 还把 `storage.Store` 拉进了生成的
`models.ts`）。两者已改为非导出，生成产物现在恰好是 [05 §5.2.1](../05-interface-contract.md#521-按功能能力的可用性)
的十个方法。接 ui-bridge 时沿用这条规则：**绑定对象上的导出方法就是前端 API**，
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

1. 为现有主题 / 语言 / localStorage 行为补最小必要夹具与单元运行器，
   沿用 npm 与现有锁文件；不为改测试工具链额外创建锁文件。
2. 以 settings-store fake 实现类型化访问与 patch；data repository 就绪后验收持久化。
3. 接生成绑定与 DTO、薄 wrapper、统一错误和事件消费，移除已替代的手写 DTO；
   不要求一次生成所有未来绑定，不声明未实现功能可用。
4. 先迁移外观 / 语言的存储来源，验证保存、事件重拉、重启；其余分区由所属功能按同样规则接管。
   迁移以功能为单位，不同时写 localStorage 和 SQLite，不自动持久化旧内存密钥。
5. 验收偏好闭环与双语言状态；新增大规模界面仍受 G-host 约束。

## 验收、阻塞与回退

完成要求：外观 / 语言真实持久化、生成绑定和 settings-access / ui-bridge 契约通过，
不需要等所有设置分区完成。单元与类型检查不代替实际重启交互。
apiRevision 的生产检查由 preferences 工程在版本不一致处理实现前决定，保持现有 wire 字段。

db-core 未就绪可推进纯设置和 wrapper fixture；G-host 不阻止维护已有外壳与必要验证界面。
回退：按分区恢复旧 store 接入，保留未清理的旧偏好及新库，明确恢复哪一个单一来源；
不能靠全清 localStorage 或把密钥写到本地偏好恢复状态。

## 验证记录

| 日期 / commit / 环境 | 命令或人工步骤 / 输入 | 期望与实际结果 | 限制 / 下一步 |
|---|---|---|---|
| 2026-09-10 | `npm --prefix frontend run typecheck` | 通过，见 [基线](../09-roadmap.md#当前代码证据) | — |
| 2026-09-11 / 见本次提交 / macOS arm64 · go1.26.3 · `CGO_ENABLED=0` | `go test ./internal/settings/ ./internal/app/`、`-race`、`go build ./...`、`go vet ./...` | 通过；默认值、夹取、patch 只改显式键、失败不留部分写入、单键读与全量读一致、事件只带改动键且仅在提交后发出、重启读回 | 单元通过不等于真实重启交互；前端未接入 |

前端单元运行器、绑定持久化、重启交互与 localStorage 迁移实验未验收。
`settings patch`、`localStorage 接管` 两项实验的**前端侧**仍未运行。
