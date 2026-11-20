# Wails v3 GA 宿主迁移

日期 2026-11-20。范围为录制宿主、桌面桥接和开发 / 打包链路。工具链基线见
[delivery 执行册](../modules/delivery.md#当前状态与证据)，宿主验收见 [IT-14](../08-testing-strategy.md#861-l2-用例)。

## 版本及边界

Go module 和 CLI 固定为 Wails `v3.1.0`，前端 runtime 固定为 `3.1.0`。Go 固定
`1.26.10`。沿用现有 Backend service、SQLite 与分段格式、写入和捕获所有权、应用身份、
Sparkle / WinSparkle 及九语言 NSIS 安装链路。

- 桌面入口使用 `daygo_desktop` 标签。无头核心仍能在 `CGO_ENABLED=0` 下构建与测试。
  绑定生成以 `daygo_desktop,server` 标记加载 Go 类型，只运行生成器。
- 窗口关闭由原生红叉回调或 v3 窗口 hook 隐藏现有窗口。应用退出回调先取消原请求，离开 UI
  线程收尾，再放行退出。状态栏、Dock、授权重启、更新及系统信号沿用 Daygo 的意图路由。
  macOS 的应用级 Show / Hide 在主线程执行。关闭事件独立标记窗口隐藏，状态栏或激活路径重开时显示并激活主窗口，再清除标记。
- Vue 在读取设置前确认原生桥接。桥接集中解包 `event.data`，薄封装将 Go nil 集合归一为
  空数组。媒体请求经 Wails 资产中间件调用原有处理器，保留数字 ID 和目录约束。错误消息沿用
  `daygo:<code>: <message>`，前端只收到公开错误消息。
- Windows 在加载平台适配器前将进程 DLL 搜索范围限制为应用目录与 System32。构建脚本
  继续编译自有 helper 和 updater DLL。NSIS 封装签名后的载荷。

## 验证与回退

验证依次覆盖纯 Go 门禁、匿名 macOS 原生宿主、真实 WebView 绑定、Windows 安装器和安装版升级。
对照版本是 `c07a0cc` 的 Wails v2 宿主。回退可还原 v3 宿主迁移提交并重新构建旧宿主。
沿用原 schema 和录制格式，可在匿名副本上演练 v2 回读。正式发布身份和真实安装升级单独验收。


macOS 匿名宿主 IT-14 已通过，Linux arm64 的纯 Go 测试和带 GTK4 / WebKitGTK 6.0 的宿主构建通过。
Windows amd64 宿主交叉编译及本地 NSIS 匿名夹具通过，安装链路仍需目标平台验证。
详见 [录制宿主实测](../modules/recording.md#2026-11-20-wails-v310-宿主验收)和
[安装与更新验证记录](../modules/delivery.md#验证记录)。
