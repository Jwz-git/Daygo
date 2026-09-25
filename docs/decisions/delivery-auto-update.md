# delivery 自动更新：Sparkle 2 + GitHub 静态 appcast + Sparkle 标准 UI

> **状态：GitHub Actions 发布自动化已实现；客户端 Sparkle 适配器已落盘。**
> [发布工作流](../../.github/workflows/publish-release.yml)在 Release 发布后构建、上传安装器。
> 2026-09-25 用户决定不申请正式平台签名材料：普通发布改为手动下载安装，默认不生成 appcast；
> 仅在显式启用 `DAYGO_SIGNED_RELEASE_APPCAST=true` 且两端资产通过既定正式身份验证时上传 appcast。
> 预发布仍跳过 appcast。下文保留自动更新链路的目标设计，其 G-native 门禁未因此放宽。
> 本文其余章节记录客户端检查、下载与安装方案。工作流实现不直接证明客户端旧版到新版升级。
> 已落地：`internal/platform/fake` 确定性 Updater + 契约测试、`GetUpdaterState` / `CheckForUpdates`
> 绑定、`update:available` 事件泵、`UpdaterStateDTO` 与前端 `api/update.ts` wrapper。`factory.NewUpdater`
> 普通开发构建不嵌入 Sparkle并返回 `native_unavailable`；发行脚本用 `daygo_updater` tag 构建并嵌入固定版本框架。
> 本文回答 [06 §6.6 第 8 项](../06-native-integration.md#66-选型时要回答的问题)
> 「自动更新链路在该形态下如何工作」，把 [09 §9.8 待定设计第 9 项](../09-roadmap.md#98-待定设计清单)
> 从「无方案」推进到「方案已定、待可行性验证」。Sparkle 集成、EdDSA 密钥管理、干净机
> Gatekeeper / 公证、更新重启前的录制收尾、真实升级保留数据与身份，全部仍受
> [G-native 门禁](../09-roadmap.md#94-全局门禁与阻塞范围)约束——缺可复核证据时不得宣称「已支持」。
> 本文不授权产出任何 release 产物或密钥；实际发布须用户明确要求。

## 1. 决策

macOS 自动更新采用三项组合，均在本文定稿：

| 维度 | 决定 | 理由 |
|---|---|---|
| 更新引擎 | **Sparkle 2**（经 `platform/darwin` cgo 适配到冻结的 `Updater` 端口） | macOS 事实标准，自带 appcast 解析、EdDSA 校验、后台 / 交互检查、原子替换与安全重启，避免自研原子替换 / 回滚 / 边界处理 |
| feed 托管 | **启用签名更新源的正式 GitHub Release 自带 `appcast.xml`** | 客户端访问 `https://github.com/Jwz-git/Daygo/releases/latest/download/appcast.xml`；普通手动下载版本不提供 feed |
| 更新 UI | **Sparkle 标准原生 UI** | 集成风险最低；冻结的 `UpdaterStateDTO`（薄）够用，无需扩展端口 / DTO |

Daygo 是 **Wails v2 单进程 `.app`**（见 [生命周期退出模型](lifecycle-quit-model.md)），不是进程外守护
形态，因此 Sparkle 更新的是单个 `.app` bundle，无需协调独立 helper 守护进程。这简化了整条链路，
但**不豁免**「更新重启前必须安全收尾当前录制分段」这一硬约束（§4）。

Windows 方案见 [WinSparkle + NSIS 决策](delivery-auto-update-windows.md)；Linux 不在本决策范围。

## 2. 检测机制（detection）

**feed。** 启用签名更新源的正式 Release 才包含 macOS DMG、Windows 安装器与一份 `appcast.xml`；
`SUFeedURL` 与 WinSparkle feed 均指向上述 `releases/latest/download/appcast.xml`。
普通手动下载版本的这个 URL 持续返回 404，应用内更新不可用。启用更新源时，
发布事件先使 Release 成为 latest，Action 后上传资产，因此上传完成前存在短暂的 404 窗口；
Action 仅在两个安装器均存在且签名、XML 生成成功后上传 appcast。发布前需确认这段窗口可接受；
预发布只构建安装包，不生成 appcast，也不会成为 `releases/latest`。提升为正式版后需手动
触发同一 tag 的 workflow 生成 appcast；在 appcast 可访问前不能认定更新源就绪。
appcast 每个 `<item>` 携带版本、最低系统版本、归档 URL、长度与 **EdDSA (ed25519) 签名**
（`sparkle:edSignature`）。

**触发。** 映射到冻结端口的 `CheckForUpdates(ctx, interactive)`：

- `interactive=true` → Sparkle `-checkForUpdates:`（用户从「检查更新」入口手动触发，无更新时也给反馈）；
- `interactive=false` → Sparkle `-checkForUpdatesInBackground`（定时后台检查，仅在有更新时打扰）。

**频率与开关。** `SetAutomaticChecks(enabled)` 映射 Sparkle 的 `SUEnableAutomaticChecks`；
检查间隔用 `SUScheduledCheckInterval`（默认建议 1 天）。首启不静默开启：沿用 Sparkle 的首启询问
（是否允许自动检查更新），与本项目 opt-in 基调一致。

**隐私。** 必须显式 **`SUEnableSystemProfiling = NO`**，关闭 Sparkle 的匿名系统画像上报——检测请求
只做一次对静态 appcast 的 HTTPS GET，除标准 HTTP 头（含 `User-Agent` 里的版本 / OS / arch）外
不发送任何可识别信息，也绝不发送屏幕内容、窗口标题、路径、密钥或 LLM 载荷。更新检查是「版本比对」，
不是[「屏幕数据离开设备」](../07-privacy-security.md)的路径。

## 3. 更新机制（apply）

Sparkle 负责：下载归档 → 校验 **EdDSA 签名**（防篡改 feed）→ 校验 **.app 的 Developer ID 签名 + 公证**
（Gatekeeper 层）→ **原子替换** bundle → **重启**。Daygo 侧只需：

1. 提供 `SPUUpdater`（`SPUStandardUpdaterController`，标准 UI）；
2. 在重启前挂钩里完成录制收尾与锁移交（§4）；
3. 把「已发现新版本」经 `update:available` 事件冒泡为次要提示（如状态栏角标），主流程仍由 Sparkle UI 承载。

**签名与密钥。** `.app` 走 Developer ID + 公证（已排期，属 G-native，见
[签名身份决策](delivery-macos-signing-identity.md)）；appcast / 归档另用 EdDSA(ed25519) 签名，
**公钥编入 `Info.plist` 的 `SUPublicEDKey`**，**私钥绝不入库**（存本地钥匙串 / CI secret，用后不留存）。
两套签名正交：Developer ID 管 Gatekeeper 信任，EdDSA 管 feed 完整性。

## 4. 生命周期：更新重启前的收尾（硬约束）

2026-09-25 实现约束：Sparkle 的 `willInstallUpdate` 只有通知作用，不能用返回值拒绝安装。因此 macOS 适配器在可拒绝的 `shouldProceedWithUpdate` 回调中完成 `Recorder.Stop`，失败则拒绝该次更新；在用户跳过、未安排安装的取消或更新驱动报错时解除录制闸门，恢复此前正在捕获的录制。发现更新到用户决定之间会暂停捕获，须在真机验证回调次序和实际暂停时长。

「更新重启」是区别于软退出 / 真退出的**独立生命周期事件**（[生命周期退出模型](lifecycle-quit-model.md)
已把它列为独立事件）。Sparkle 在替换 bundle 前会终止进程，因此重启前**必须**同步完成：

- 经 recording 的收尾协议收尾或安全移交当前分段（`platform.SegmentCloser.CloseActiveSegment`），
  避免留下[完全不可读的未收尾分段](../06-native-integration.md)；
- 释放**写入锁**与**捕获所有者锁**（见 [data 实例锁](data-locking.md)），让重启后的实例能重新获取；

重启后：重新获取锁 → 向适配层重新下发全量配置 → 重放未确认帧。挂钩点用 Sparkle 的
`updater:willInstallUpdateOnQuit:` / `updaterWillRelaunchApplication:`（`SPUUpdaterDelegate`）与
`NSApplicationWillTerminate`，在 `internal/app` 层编排，收尾逻辑仍归 recording，不在 delivery 重复实现。

**只有捕获所有者实例执行安装重启。** 它持有捕获 / 写入锁，能安全收尾；只读实例（`not_capture_owner`）
可检查更新，但安装重启须让位给所有者实例，避免两个进程同时替换 bundle。

## 5. 与冻结端口的映射（不改端口 / DTO）

标准 UI 方案下，[冻结的 `Updater` 端口](../05-interface-contract.md#57-b4platform-端口契约)与
`UpdaterStateDTO` 均够用：

| 端口方法 / DTO 字段 | Sparkle 承载 |
|---|---|
| `CheckForUpdates(ctx, interactive)` | `-checkForUpdates:` / `-checkForUpdatesInBackground` |
| `SetAutomaticChecks(enabled)` | `SUEnableAutomaticChecks` |
| `State() → {Automatic, Checking, AvailableVersion, LastCheckedAt}` | 组合 `automaticallyChecksForUpdates` / `sessionInProgress` / 最近 appcast item / `lastUpdateCheckDate` |
| `Events()`（有界 ≥8，合并） | 发现新版本时投递一次，app 层转 `update:available`（状态广播，幂等覆盖） |
| `GetUpdaterState()` 绑定 | 读 `State()` |

下载进度、「就绪重启」等富状态由 Sparkle 标准 UI 自己承载，**不进入 DTO**；这正是选标准 UI 而非自定义
Vue UI（`SPUUserDriver`）的收益——保持端口与 DTO 冻结不变。

## 6. 架构落点与跨平台

- **实现放 `internal/platform/darwin`**（Objective-C/cgo 桥到 Sparkle）；fake 在 `internal/platform/fake`。
  发行构建通过 `daygo_updater` tag 接入，开发构建不下载或伪造更新能力。
- **Go Core 保持 `CGO_ENABLED=0` 可构建**：业务不接触 Sparkle，只对着 `Updater` 端口与 fake 写代码 /
  测试；`go test ./internal/...` 仍须在 Linux 通过。
- **跨平台端口统一，引擎分平台定**：Windows（WinSparkle / NSIS 内建更新 / MSIX，属 Windows 待决发布范围）
  与 Linux（当前 `unsupported` 桩）的具体引擎**留待各自子决策**，本文只定 macOS。

## 7. 可行性实验（G-native 前置，先于实现）

对应 [06 §6.6 第 8 项](../06-native-integration.md#66-选型时要回答的问题)「原生形态确定前验证可行性；
Updater 完成时验证真实升级」，可先做的有限实验（不产出正式 release）：

| 实验 | 操作 | 预期 | 失败条件 |
|---|---|---|---|
| feed 可达与解析 | 发一份测试 `appcast.xml` 到测试 Release，触发检查 | 正确解析版本、发现 / 无更新状态明确 | 解析失败、状态不明 |
| EdDSA 校验 | 用错误 / 缺失签名的归档喂给 Sparkle | 拒绝安装并报错，不落地被篡改包 | 接受未签名 / 错签名包 |
| 更新重启收尾 | 触发一次真实升级，观察重启前后 | 分段先收尾 / 移交，锁释放并重获，未确认帧重放 | 关窗被当退出、未收尾即重启、重启后拿不到锁 |
| 真实升级保留 | 旧版写匿名卡片 / 测试密钥，升级到新版 | schema 迁移、数据读回、密钥读取、捕获所有者锁保留 | 丢数据、重复授权未解释、无法恢复 |

## 8. 未验证与门禁（G-native）

- **真机验收：2026-09-22 用户确认通过（无逐项记录）**：Sparkle 集成、后台 / 交互检查、下载校验、原子替换、重启收尾、真实升级保留数据与
  授权身份，均须在真实 macOS 完整观察。
- **发布身份未就绪**：Developer ID + 公证属 G-native（[签名身份决策](delivery-macos-signing-identity.md)），
  自签名开发证书不满足干净机 Gatekeeper。
- **EdDSA 流程已建立但未实发验证**：新私钥只在本机钥匙串和 GitHub Actions Secret，公钥编入
  两端适配器 / `Info.plist`；尚未用正式产物验证拒绝错签名与接受正确签名。
- **发布链路源码已建立但未运行正式签名路径**：Release workflow 默认只构建、上传供手动下载的
  安装器；签名更新源需显式启用并由身份验证任务放行。Developer ID / 公证和 Windows
  Authenticode 材料未配置，因此当前不生成 appcast。

## 9. 回退

停用不可用的更新入口，回退到「用户手动下载新版本」；`Updater` 缺实现时绑定返回
`native_unavailable`，UI 隐藏更新入口。schema 版本变动必须走 [data 备份恢复计划](data-backup-retention.md)，
不能仅替换二进制或删库。任何回退保留 pending 截图、已发布媒体与用户配置。
