# recording 屏幕截屏（Linux）：候选、约束与决策进度

> **状态：已排期，决策进行中（尚未落定单一方案）。** Linux 从「仅可移植 / CI 目标」升级为
> 一等发布目标（[09 §9.8 #24](../09-roadmap.md#98-待定设计清单)），本文收拢 Linux 屏幕捕获与
> 适配器形态的候选、硬约束与验证门槛。**在本文把某条路径标为「已决定」之前，不得按单一候选
> 大规模实现，也不得删除其它候选路径**（AGENTS.md 待定设计纪律）。
>
> 端口契约以 [`internal/platform/ports.go`](../../internal/platform/ports.go) 与
> [05 §5.7](../05-interface-contract.md#57-b4platform-端口契约) 为准；跨平台单次调用规格见
> [屏幕截屏：单次调用契约与原生 ABI](recording-screen-capture.md)。macOS / Windows 实现见
> [v2](recording-screen-capture-v2.md) 与 [Windows](recording-screen-capture-windows.md)。

## 1. 当前事实（不是规划）

- Linux 桌面壳（Wails v2 + GTK3 + WebKit2GTK）可启动并加载 Vue 前端；Go Core 与 SQLite
  在 Linux 上与 macOS 等价，`options_linux.go` 已拆分窗口选项。
- `Capture` / `System` / `ApplicationInspector` 在 Linux 走
  [`factory/*_unavailable.go`](../../internal/platform/factory)（`//go:build !darwin && !windows`），
  运行时返回 `unsupported`。
- `Media` 在 Linux 是**真实实现**（[`internal/platform/mediafile`](../../internal/platform/mediafile/mediafile.go)，
  纯 Go JPEG 单帧解码 / 探测）；`EncodeVideo` 待 M2 编码决策。
- `Secrets` 已决定用 freedesktop Secret Service / `secret-tool`
  （[Linux 密钥决策](providers-secrets-linux.md)），真实桌面钥环经用户确认已验收（无逐项记录）。
- 实例锁走 `lock_unix.go` 的 `flock`，时区走 `zone_notwindows.go`——与 macOS 共用非 Windows 路径。
- `native/` 下**没有 Linux 适配器**；`preBuildHooks` 只在 darwin / windows 触发。

## 2. 不可让步的约束

1. **捕获必须是离散截图，不是连续屏幕录制流**，且不得让桌面出现常亮的「正在共享屏幕」指示器
   （[06 §6.2.1](../06-native-integration.md#621-关于捕获方式的一条产品约束)）。这条对 Linux 尤其关键——
   见 §3 的 Wayland 取舍。
2. **不得引入第二套 `Capture` 契约、第二个 ABI 或平台专用 DTO。** Linux 差异只落在原生实现能力上。
3. **Go Core 必须保持 `CGO_ENABLED=0` 可构建、`go test ./internal/...` 在 Linux 通过。** 任何 cgo
   或系统库只能进 `internal/platform` 适配层，不得回流核心。
4. 适配层不打开 / 不写 SQLite、不读设置自决策、不发网络请求、不含产品逻辑
   （[06 §6.3](../06-native-integration.md#63-适配层的职责边界)）。

## 3. 候选与取舍

### 3.1 捕获后端：X11 vs Wayland

| 维度 | X11 | Wayland |
|---|---|---|
| 单帧抓取 | `XGetImage` / `XShmGetImage` 直接读根窗口或主输出，天然离散、无持久指示器 | 合成器不允许任意进程读帧；标准路径是 `xdg-desktop-portal` |
| Portal 路径 | 不需要 | `org.freedesktop.portal.ScreenCast`（PipeWire 流）**面向连续流**且通常带共享指示器——与 §2.1 直接冲突；`org.freedesktop.portal.Screenshot` 是交互式单次截图，不适合静默周期捕获 |
| 授权模型 | 无系统授权（与 Windows 类似） | Portal 每会话询问 / 可能带持久指示器 |
| 隐私画面级屏蔽 | 无原生「排除指定应用」能力，须评估失败关闭语义 | Portal 无等价「排除应用」原语 |
| 现状占比 | 仍广泛存在，尤其旧发行版与部分 DE | 新发行版默认（GNOME / KDE Wayland 会话） |

**核心矛盾：** Daygo 的「静默、离散、无常亮指示器」产品约束在 X11 上自然成立，在 Wayland 上
与 Portal 的流式 / 指示器模型**根本冲突**。这不是实现细节，是产品与平台能力的冲突，必须在本文
显式决策，而不是在实现里悄悄放宽 §2.1。

**待验证实验（LC 前置，尚未运行）：**
- LC-a：X11 下 `XShmGetImage` 周期抓主输出的资源占用与多屏 / 缩放 / 旋转正确性；
- LC-b：Wayland 下是否存在任何合规且**不常亮指示器**的周期截图路径（含各 DE 差异），若无则
  明确「Wayland 会话下捕获失败关闭」的产品结论；
- LC-c：X11 隐私屏蔽的失败关闭语义（无法可靠排除应用时返回何种 `CaptureErrorCode`）。

### 3.2 适配器形态

沿用 [06 §6.6](../06-native-integration.md#66-选型时要回答的问题) 的宿主 / 形态问题（#1）：进程内 cgo
适配器 vs 进程外。Linux 与 macOS / Windows 共享同一形态决策，不单独另立。

### 3.3 分发形态

| 候选 | 优点 | 代价 / 风险 |
|---|---|---|
| AppImage | 单文件、跨发行版、无需安装权限 | 自更新与桌面集成需额外处理；沙箱外 |
| Flatpak | 沙箱、Portal 集成、更新链成熟 | 沙箱内截图强制走 Portal，回到 §3.1 的 Wayland 矛盾 |
| deb / rpm | 原生包管理、系统集成好 | 需分别维护，覆盖面碎片化 |

分发身份（自启、密钥钥环、更新）与 [delivery](../modules/delivery.md) 协调，受 G-native 约束。

## 4. 推荐方向（待实验确认，非最终结论）

1. **优先落地 X11 离散抓取**作为第一条可发布路径（`XShmGetImage`），因为它天然满足 §2.1；
   Wayland 作为**独立后续决策**，在 LC-b 给出「有无合规无指示器路径」结论前，Wayland 会话
   按失败关闭处理，不降级到带常亮指示器的流式捕获。
2. **分发先做 AppImage 的有限探针**（无自更新），验证纯 Go Core + GTK 壳的启动与钥环行为，
   再决定 Flatpak / deb / rpm。
3. 隐私屏蔽在 Linux 若无可靠的画面级排除，**失败关闭**（返回 `privacy_unsupported`），
   与 Windows 旧系统同规则，**不得**降级为「只查前台」。

以上均需 §5 的实验证据支撑后方可在本文改标「已决定」。

## 5. 门禁与验证

- **G-core 不变**：`CGO_ENABLED=0` 核心与 `go test ./internal/...` 在 Linux 常绿；Linux 原生代码
  仅在带 cgo / 相应 build tag 时编译，不进核心。
- **LC 真机矩阵（待建）**：类比 [08 §8.6.3 WC](../08-testing-strategy.md#863-wc真实-windows-捕获矩阵)，
  覆盖单帧非黑、多屏 / 缩放 / 旋转、隐私失败关闭、资源 24h、X11 与 Wayland 会话差异。
- **G-host / G-native 照旧**：关窗后持续离散捕获、状态栏 / 托盘重开、激活策略、签名 / 分发身份
  在 Linux 上分别验收；`secret-tool` 真实钥环按 [Linux 密钥决策](providers-secrets-linux.md) 验收。

## 6. 边界与回退

- 回退：composition root 不注入 Linux `Capture` / `System`，保留 `factory/*_unavailable.go` 的
  `unsupported` 路径即回到当前「可移植核心 + 桌面壳」状态。
- 不为让 Linux 出图而放宽 §2 任一约束；任何降低隐私或引入常亮录制指示器的路径直接否决。
