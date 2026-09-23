# delivery macOS 签名身份：本地自签名稳定证书，保住 TCC 授权跨重构

> **状态：已落盘的打包脚本切片（三条签名路径 + 自签名证书生成脚本）。真机重启 / 升级身份、
> 干净机器 Gatekeeper 与公证可行性仍受 G-native 门禁约束，缺可复核证据时不得宣称「已支持」。**
> 本文记录 Daygo 本地开发用哪种签名身份、为什么，以及打包脚本已经做了什么。发布链路（Developer ID +
> 公证）以 [delivery 模块册](../modules/delivery.md) 与 [`package-macos.sh`](../../scripts/package-macos.sh) 为准。

## 1. 决策

**症状**：`./scripts/dev.sh`（`wails dev`，从 Terminal 启动）录制正常；打包安装后「点录制只持续几秒就停」，
且「重新打包、重新安装还是不行」。

**根因（查官方 + 实测确认）**：macOS TCC 把屏幕录制授权绑定到 App 的 **designated requirement (DR)**，
DR 取决于签名方式：

- **ad-hoc 签名**（`codesign -s -`，裸 `wails build` 的默认）：无证书可绑，DR 退化为
  `cdhash H"..."`——代码内容哈希，**每次构建都变**。bundle id 根本不进入 ad-hoc 二进制的 DR。
  于是每次重构 = 新 cdhash = TCC 眼中的陌生新 App，旧授权被孤立（orphaned），开关静默失效。
- **证书签名**（哪怕本地自签名，无需付费 Apple 账号）：DR 为
  `identifier "io.github.jwz-git.Daygo" and certificate leaf ...`——**跨重构稳定**，授权保持。

Terminal 启动 `wails dev` 能录制，是 "responsible process" 机制把权限归因到已授权的 Terminal，
掩盖了 App 自身缺授权；直接双击安装的包则暴露真相。

**决定**：本地开发采用**自签名稳定证书**。理由：无需 Apple 付费账号即可获得稳定 DR，根治「重构后
授权失效」的开发循环。它**不公证、不用于分发**；对外发布仍走 Developer ID + 公证（已排期，属 G-native）。

## 2. 三条签名路径

[`package-macos.sh`](../../scripts/package-macos.sh) 按环境变量选择，互斥且优先级递减：

| 变量 | 路径 | 用途 | 标志 |
|---|---|---|---|
| `DAYGO_SIGN_IDENTITY` | Developer ID | 分发 | `--options runtime --timestamp`，可接 `DAYGO_NOTARY_PROFILE` 公证 |
| `DAYGO_DEV_SIGN_IDENTITY` | 自签名 | 本地稳定 TCC | `--identifier io.github.jwz-git.Daygo`，无 timestamp / 无 runtime / 无公证 |
| （都不设） | ad-hoc | 一次性冒烟 | `--sign -`，授权不跨重构 |

自签名路径显式 `--identifier` 锁定 bundle id，确保 DR 为 `identifier + certificate leaf` 而非 cdhash。
不加 `--timestamp`（离线）、不加 `--options runtime`（那是可分发身份才需要的加固）。

## 3. 证书生成

[`scripts/dev-cert-macos.sh`](../../scripts/dev-cert-macos.sh)（幂等，已存在则跳过）：

- `openssl req -x509` 生成带 `codeSigning` EKU + `digitalSignature` keyUsage（均 critical）的自签名证书；
- 打包成无口令 PKCS#12，`security import -T /usr/bin/codesign` 导入 login keychain；
- `security set-key-partition-list` 预授权 codesign 非交互使用私钥（需 keychain 密码，脚本会提示）。

证书**无需设为受信任**：codesign 嵌入签名只要 identity 在 keychain 且有 codeSigning EKU 即可；
信任只影响 Gatekeeper 验证，不影响 TCC 的 DR 匹配。

用法：

```bash
./scripts/dev-cert-macos.sh                                   # 建证书（一次）
DAYGO_DEV_SIGN_IDENTITY="Daygo Dev" ./scripts/package-macos.sh # 稳定签名打包
```

安装后授权一次并重启 Daygo；因身份稳定，之后重构仍保留该授权。

## 4. 未验证与门禁

- **真机验收：2026-09-22 用户确认通过（无逐项记录）**：自签名包安装后授权 → 重启生效 → 重构后授权保持，须在真实 macOS 上完整观察一轮。
- **Gatekeeper**：自签名包在别的机器上仍触发「未识别开发者」提示（自用无妨）；干净机器 Gatekeeper /
  公证可行性属 [G-native](../09-roadmap.md#94-全局门禁与阻塞范围)，未验证。
- **同签名重启 / 升级身份**：providers 密钥与录制授权在同一稳定身份下重启、升级后是否保持，
  是 G-native 的验收项，本切片不覆盖。
- 发布链路（Developer ID、公证、自动更新）尚未建立；未经用户明确要求不产出 release 产物。
