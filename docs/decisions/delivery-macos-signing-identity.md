# delivery macOS 签名身份：持久化自签名稳定证书，保住 TCC 授权跨更新

> **状态：已落盘的打包脚本 + CI 签名身份闸门；`release` 环境已有一张固定自签名证书的 Secret。
> 首次带证书的 CI 打包、真机重启 / 升级身份、跨版本 DR 一致、干净机器 Gatekeeper 与公证可行性仍受 G-native 门禁约束，
> 缺可复核证据时不得宣称「已支持」。**
> 本文记录 Daygo 用哪种签名身份、为什么，以及打包脚本与发布 workflow 已经做了什么。发布链路（Developer ID +
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

**决定**：采用**持久化自签名稳定证书**。理由：无需 Apple 付费账号即可获得稳定 DR，解决「更新 / 重构后
授权失效」的身份问题。它**不公证、不构成正式分发身份**（Gatekeeper 仍提示未识别开发者；自用或明确知情的测试者可手动放行）；
标准对外分发仍需要 Developer ID + 公证，属未通过的 G-native 门禁。

关键是**同一把证书跨版本复用**：DR 为 `identifier "io.github.jwz-git.Daygo" and certificate leaf = H"…"`，
其中 `certificate leaf` 锚定这张证书本身。因此只有当**每个发布版本都用同一张证书签名**时 DR 才稳定、TCC
授权才跨更新保留。本地开发把证书存在 login keychain 里，CI 则把同一张证书的 p12 存进 GitHub Secret，
每次发布都用它签名（见 §3.1）。Sparkle 的更新信任锚是不变的 EdDSA 归档签名（非代码签名，见
[自动更新决策](delivery-auto-update.md)），所以从旧 ad-hoc 包升到首个自签名包也能被接受——代价只是首版
DR 变一次、需重授权一次，此后永久保留。

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

### 3.1 CI 持久化（发布用同一张证书）

发布产物由 GitHub Actions 打包，必须复用同一张证书 DR 才能让授权跨更新保留。链路：

- **导出证书材料**（一次性，本地）：`DAYGO_DEV_CERT_P12_OUT=/path/outside/repo/daygo-dev.p12 ./scripts/dev-cert-macos.sh`
  在生成 / 导入证书的同时，导出一份**带口令的 p12**及单独的 `.password` 文件，权限均为 `0600`，不在终端打印内容。
  脚本拒绝把它们写进仓库，也拒绝在同名本地身份已存在时再生成一张不同的 CI 证书。OpenSSL 3 默认
  PBES2/SHA-256 的 PKCS#12 在当前 macOS 钥匙串导入时报 `MAC verification failed`，脚本改用经本机
  临时钥匙串导入验证的 3DES PKCS#12 容器；加密备份与密码文件须妥善保管，私钥绝不入 Git。
- **GitHub Secrets**：`release` 环境中的 `DAYGO_DEV_CERT_P12_BASE64`、`DAYGO_DEV_CERT_P12_PASSWORD`。
  已有的 `SPARKLE_ED25519_PRIVATE_KEY`（更新信任锚）不变；Base64 只是传输编码，不是加密。
- **workflow**（[`publish-release.yml`](../../.github/workflows/publish-release.yml) 的 macOS job）：打包前新增
  "Import signing certificate" 步骤——macOS job 引用 `release` 环境；secret 缺失则失败，禁止发布 ad-hoc 包。
  导入临时 keychain 后核对固定证书的 SHA-256 指纹，按其证书哈希签名；挂载最终 DMG，核对应用签名与
  `identifier + certificate leaf` 的 DR，合格后才上传。`always()` 步骤删除临时 keychain。EdDSA /
  appcast 步骤不受影响。

**安全考量**：自签名代码签名私钥进 GitHub Secret，泄露时攻击者可签出满足 Daygo DR 的二进制，但仍需另一把
EdDSA 私钥才能过 Sparkle 更新校验，且该证书非 Apple 身份、不可用于更广泛冒充。风险低但非零；与项目已用于
存放 EdDSA 私钥的模式同级。

## 4. 未验证与门禁

- **真机验收：2026-09-22 用户确认通过（无逐项记录）**：自签名包安装后授权 → 重启生效 → 重构后授权保持，须在真实 macOS 上完整观察一轮。
- **2026-09-25 本机准备（非发行验收）**：生成固定自签名证书，本机钥匙串与临时钥匙串均成功导入兼容的
  PKCS#12；临时可执行文件用证书哈希签名后，DR 为 `identifier "io.github.jwz-git.Daygo" and certificate leaf`。
  两个证书 Secret 已存入 GitHub `release` 环境；尚无使用此证书的 CI 安装包或真实升级记录。
- **CI 持久化验收（未验证）**：检查首次 CI 产物的 DR 为上述身份，且连续两版一致；升第二版后 Sparkle
  更新**不重新授权**即可继续录制；Console.app 无 Sparkle 代码签名拒绝。须在真实 macOS + 测试
  appcast 上走一轮，属 G-native；此配置不等于正式证书、公证或干净机验收。
- **Gatekeeper**：自签名包在别的机器上仍触发「未识别开发者」提示（自用无妨）；干净机器 Gatekeeper /
  公证可行性属 [G-native](../09-roadmap.md#94-全局门禁与阻塞范围)，未验证。
- **同签名重启 / 升级身份**：providers 密钥与录制授权在同一稳定身份下重启、升级后是否保持，
  是 G-native 的验收项，本切片不覆盖。
- Developer ID 就绪后改传 `DAYGO_SIGN_IDENTITY` + `DAYGO_NOTARY_PROFILE`（`package-macos.sh` 已支持），
  同时解决 Gatekeeper；EdDSA 锚不变，可平滑切换。未经用户明确要求不产出 release 产物。
