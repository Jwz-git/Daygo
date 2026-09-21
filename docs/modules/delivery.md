# delivery — 安装与更新

## 用户结果与范围

用户能在干净机器上安装应用，经授权、Provider 配置、分类初始化产出首条时间线，
并安全升级且保留数据与授权身份。负责 F-L4、首次运行引导与分发验收。
签名、公证、身份和更新可行性要早做有限实验，完整发行只组合已验收模块。
本执行册不授权生成、修改或发布 release 产物；实际发布须用户明确要求。
Chat 已有部分实现，但 v1 明确不交付，v1.1 是否纳入留待后续评估，见 [chat 模块](chat.md)；
Windows 发布也保持待决。CLI / agent socket / MCP 已移交
[agent 模块](agent.md)（设计准备中，v1 不交付）。

依据：[06 选型问题](../06-native-integration.md#66-选型时要回答的问题)、
[05 更新绑定](../05-interface-contract.md#权限系统与更新)、
[07](../07-privacy-security.md)、[风险 C-3 / M-3](../10-risks.md)。

## 当前状态与证据

实现进度：部分实现。已有 [Wails 配置](../../cmd/daygo/wails.json)、macOS / Linux 开发构建链，
以及 Windows 的 `scripts/dev.ps1` / `scripts/build.ps1` 入口；Windows 构建会校验 EXE 与必需的
`daygo_windows_native.dll` 同时产出。打包入口方面，`scripts/package-macos.sh` 产出签名 DMG，
`scripts/package-windows.ps1` 走 `wails build -nsis` 产出 NSIS 安装程序并可选 `signtool` 签名。
Windows 流程先签 EXE 与原生 DLL，再用仓库内 NSIS 模板重新封装最终字节并签安装器；模板显式安装
`daygo_windows_native.dll`，同时输出带源码 commit、文件大小与 SHA-256 的验收清单。
`package-windows.ps1` 已在真实 Windows 上产出 v0.1.0 amd64 NSIS 安装包，并与 macOS arm64 DMG
一同发布到 GitHub Releases。该事实只证明发布资产存在，不自动证明其签名、安装、卸载或升级行为；
安装程序内容、签名、静默安装 / 卸载与干净机启动仍需按验收矩阵补充可复现证据。
Updater 已按 [macOS 决策](../decisions/delivery-auto-update.md)和
[Windows 决策](../decisions/delivery-auto-update-windows.md)接线：fake 契约、绑定、事件泵、设置 UI、
Sparkle / WinSparkle 适配器、共用 Ed25519 appcast、安装前 owner / recorder 收尾和 GitHub Release workflow
均已落盘。普通 macOS 开发构建不带 `daygo_updater` tag，诚实显示不可用；发行脚本才嵌入 Sparkle。
签名 workflow、真实安装升级、Gatekeeper / Authenticode 与首次引导仍未验收。
捕获文档历史静态库编译探针不构成发行身份或升级证据。
**2026-09-21：更新弹窗中属于我们的那句文案接入 i18n**（“只有持有捕获所有权的 Daygo 实例
才能安装更新”，此前是 `updater_bridge.m` 里的硬编码英文）。它随
[05 §5.5.1](../05-interface-contract.md#551-绑定方法目录) 的 `SetNativeUiLabels` 下发，
darwin 适配器经 `platform.UpdateCopySink` 接收并推给 Sparkle 的 delegate；`daygo_updater`
构建（含 Sparkle 链接）的 `go test` 通过，但**真实拒绝路径的弹窗文案未在 Sparkle UI 上
视觉验收**。Sparkle / WinSparkle 自有对话框的文案由框架的 lproj 提供，按系统语言渲染，
不随应用内语言设置变化；本通道不覆盖它们。

## 能力与跨层职责

| 输入 | 可先推进 | 真实接入条件 |
|---|---|---|
| recording: 宿主 / 原生候选、收尾协议 | 桩适配层、签名布局、更新重启有限实验 | 06 §6.6 相关证据；原生形态决定前验证分发可行性 |
| providers: Secrets / 配置；data: 持久化 / 恢复 | 匿名安装状态、升级故障 fixture | 同签名重启 / 升级后的密钥与数据保留 |
| 已交付模块的状态 / 设置契约 | 引导编排和 Updater fake | 实际授权、Provider、分类、时间线能力均已验收 |
| 签名身份、干净设备及发布授权 | 只读检查和无发行副作用的实验设计 | 缺少任何必要条件时记录对应阻塞，不伪造通过 |

输出 update；交付 Updater fake / 原生、绑定、设置和事件 UI，app 编排更新停机。
recording 决定如何安全收尾，data 提供恢复能力；delivery 不重复实现捕获 / 数据库状态机。
首次引导调用各功能已有绑定，不新建另一套设置或授权路径。
data 负责遥测设置和载荷边界，delivery 接入 opt-in 崩溃报告及版本 / 会话关联，
禁止附带真实屏幕或 LLM 内容。

## 实验与失败条件

| 实验 | 输入与操作 | 预期结果 | 失败条件 / 证据 |
|---|---|---|---|
| 提前身份 / 分发探针 | 记录身份和候选产物布局，具备授权后验证签名 / 公证及干净机器 Gatekeeper | 实际产物验证通过，捕获 / 钥匙串身份稳定 | 缺签名材料、未测机器或仅本机启动不能通过 G-native |
| 更新可行性 | 宿主候选下触发检查、取消、失败及更新重启 | 有可行更新路径，状态明确，先收尾或安全移交 | 关窗口被当退出、未收尾即重启失败 |
| 完整安装 | 干净机器安装 → 用户授权 → 配置 Provider → 首条时间线 | 功能真实可用，无默认服务或隐式上传 | 跳过必要能力验收、fixture 卡片冒充真实产出失败 |
| 真实升级 | 在旧构建中存匿名记录及测试密钥，再升级；插入失败 / 中断 | 数据、密钥和授权身份保留，失败路径可恢复 | 丢数据、重复授权未解释、无法恢复失败 |
| 崩溃报告 | opt-out / opt-in 下触发匿名故障 | 默认不上传，只含许可元数据，可关联版本 / 会话 | 屏幕、窗口标题、路径、密钥、payload 出现在报告失败 |

## 实现切片与集成

1. 与 recording / providers 为 06 §6.6 的身份、签名、公证、升级问题建立可复现实验；
   用有限桩验证宿主形态是否可分发，不拖到完整产品交付。
2. 在更新方案决定前补 Updater fake、取消 / 失败 / 状态事件契约，
   固定安全重启调用顺序和中断 fixture。
3. 接真实 Updater，与 recording 收尾和 data 恢复共同验收；命令与产物操作遵循发布授权边界。
4. 复用已验收功能实现首次引导、更新设置 / UI 与 opt-in 崩溃接入，
   完整覆盖拒绝授权、无 Provider、只读实例和失败状态。
5. 明确本次发布范围，只收集完成模块；执行干净机器安装和一次真实升级，
   附长期验证状态，不按版本倒逼未完成模块伪报可用。

## 验收、阻塞与回退

完成要求：签名、公证、干净机器安装与真实升级全部通过，更新路径不丢分段，
数据和身份保留，用户可关闭遥测。原生可行性探针通过仅解锁相应实现，不等于可发布。
签名身份 / 设备 / 发布授权缺失只阻塞相关实验或分发，其他模块可按契约继续开发。

待决：macOS Sparkle 与 Windows WinSparkle + NSIS 均已定稿并实现；真机可行性、签名身份与真实升级
仍受 G-native / WD 约束未验收。Linux 引擎另行决策。后续 Chat / CLI 范围仍单独决定。
回退：停止未验收的更新入口，按已验证更新恢复方案返回可运行构建；
schema 版本变动必须走 data 的备份恢复计划，不能仅替换二进制或删除数据库。
任何回退保留 pending 截图、已发布媒体与用户配置。

### Windows 打包验收单

以下各项必须记录 Windows build、CPU、commit、证书主体（不记录私钥信息）和产物清单 SHA-256；
任一项未执行都只能记为“入口已实现”，不能记为 Windows delivery 通过：

1. 在干净检出上运行 `scripts/package-windows.ps1 -Version <x.y.z> -RunSmoke`；确认
   `dist/Daygo-<x.y.z>-amd64-installer.exe` 与 `dist/windows-package.json` 一致。
2. 有签名材料时，对构建目录的 `Daygo.exe`、`daygo_windows_native.dll` 和最终安装器分别运行
   `signtool verify /pa`；随后安装并对安装目录中的 EXE / DLL 再次验证，证明 NSIS 内层确实是
   已签字节。无签名材料只能验收 unsigned 本地测试路径。
3. 在未安装 Daygo 的 Windows 11 amd64 干净用户上分别验证交互安装与 `/S` 静默安装；启动后确认
   DLL 可加载、通知区可重开窗口、录制可提交一帧。机器级与用户级安装范围若都准备提供，分别运行。
4. 退出 Daygo 后运行卸载（含 `/S`），确认二进制、快捷方式和卸载注册表项移除；应用支持目录、
   SQLite 和 Credential Manager 密钥必须保留，除非另有经过确认的数据删除入口。
5. 用前一版本写入匿名卡片、设置与测试凭据，再安装新版本；确认 schema 迁移、数据读回、密钥读取、
   捕获所有者锁和回退路径。升级中断必须能恢复，不得用删除数据库作为恢复办法。

此清单即 [08 §8.6.4 WD](../08-testing-strategy.md#864-wd真实-windows-分发矩阵) 的人工执行细化，
只覆盖 delivery 产物。Windows 发布还必须同时通过 recording 的 WC-1–8、DB-8 长时并发、
真实 Provider / Credential Manager 身份和长期观察；安装成功不能替代这些门禁。

## 验证记录

更新适配器源码、设置 UI、签名 appcast 生成器和 Release workflow 已做本地静态 / 契约验证；
签名、公证、干净机器、真实升级与崩溃上报实验仍未运行。
记录 commit、构建身份、设备、步骤及匿名结果；证书、密钥、用户数据不入库。

2026-09-21（macOS arm64 本机）：`./scripts/gate.sh` 全部通过；带 `daygo_updater` tag 的 Wails
应用完成编译，并用 `otool` 确认 Sparkle 依赖与 `@executable_path/../Frameworks` rpath；Windows
更新适配器完成 amd64 交叉编译。另用 Sparkle 官方 `sign_update` 对匿名 macOS / Windows 夹具
签名，并验证生成 appcast 同时包含两个平台项目。上述结果只验证源码、链接布局、签名格式与
发布编排；尚未使用 Developer ID / Authenticode 正式证书，也未执行 macOS / Windows 真机升级，
因此不提升 G-native 或 WD 状态。

2026-09-20：GitHub Release `v0.1.0` 已发布为预发布版本，包含
`Daygo-0.1.0-arm64.dmg` 与 `Daygo-0.1.0-amd64.exe`。此记录证明两个资产可从公开 Release 获取；
缺少签名 / 公证、干净机安装、卸载、真实升级和 WD 完整记录，因此不提升 delivery 验收状态。

2026-09-20（Windows 11 amd64、Windows PowerShell 5.1）：`package-windows.ps1` 明确保存为
带 BOM 的 UTF-8，避免 Windows PowerShell 5.1 按本地代码页误解脚本中的 Unicode 输出字符，
并在后续 ASCII 字符串处误报 `MissingArgument`；同时在 `PATH` 未因 winget 安装而刷新的终端中，
自动探测 NSIS 的 machine / 32-bit machine / per-user 标准安装目录。已用 5.1 parser 验证脚本无
语法错误，并验证已安装的 `makensis.exe` 可被发现。Windows 入口也会在 Wails 绑定生成前重建
原生静态库，避免旧 archive 缺少新增 ABI 符号而让绑定生成在链接阶段失败；尚未完成 NSIS 封装、
签名、安装或升级，因此不提升 WD 状态。

2026-09-14（Windows 11 amd64）：通知区适配器把 `NOTIFYICON_VERSION_4` 的
`NIN_SELECT`（并兼容传统 `WM_LBUTTONUP`）映射到既有 open 动作，普通左键单击不再要求先打开
右键菜单。`system_smoke.cpp` 新增 Explorer 回调消息夹具；状态栏源文件通过 `g++ -Wall -Wextra
-Wpedantic` 编译，`go test ./internal/platform/windows -run 'TestSystem|TestStatus' -count=1` 与
`go test ./internal/app -count=1` 通过。因验证时 Daygo 开发实例正占用通知区注册，完整
`system_smoke` 实机交互留待该实例退出后补跑；本记录不提升 Windows 发布状态，也不构成
G-host 通过。

2026-09-12（Windows 11 amd64、go1.25.4）：Wails 2.15 debug+cgo 构建触发 Go
[链接器缺陷 #75077](https://github.com/golang/go/issues/75077)，坏 PE 的 header/file alignment 为
1352/512。`scripts/dev.ps1` 临时启用 `GOEXPERIMENT=nodwarf5` 后变为 1536/512，Windows loader
可启动；Windows 无状态栏适配器时的 nil `System` 启动 panic 也已失败关闭。此记录只证明本机
开发构建可启动，不构成签名、安装、升级或 Windows 发布证据。
