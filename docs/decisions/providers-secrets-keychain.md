# providers 密钥存储：`security` CLI 子进程访问钥匙串

> **状态：已决定（本轮实现范围内）。** 本文记录 `platform.Secrets` 端口在 macOS 上的
> 实现手段与边界，解决 `09 §9.8` #4 的「钥匙串访问方式」部分；发布身份（签名 / 公证后
> 钥匙串行为）仍属 delivery 范围，不因本文关闭。

## 1. 决定

macOS 适配层用 **`/usr/bin/security` 命令行子进程**实现 `Secrets` 端口（`platform/ports.go`），
不引入 cgo，不链接 Security.framework：

| 操作 | 命令 |
|---|---|
| `Set` | `security add-generic-password -U -s <service> -a api-key -w <secret>` |
| `Get` | `security find-generic-password -s <service> -a api-key -w` |
| `Delete` | `security delete-generic-password -s <service> -a api-key` |

- service 名遵守已发布身份：`io.github.jwz-git.daygo.apikeys.<providerID>`，account 固定
  `api-key`，条目形态为 generic password。
- 每次调用经 `exec.CommandContext`，超时 5 秒；`Delete` 遇到「条目不存在」映射为类型化的
  not-found 结果，不是错误。
- 密钥值**绝不进入任何 error 文案**；`Get` 的值只供 Go 侧 provider 客户端构造使用，
  绑定层只暴露 `hasSecret` 布尔（05 §5.5.2 既有规则）。
- 非 darwin 平台返回 `native_unavailable`；测试与 Linux CI 使用 `NewFake()` 内存实现。

## 2. 候选与取舍

### 2.1 选中：`security` CLI 子进程

**优点。** 纯 Go：`CGO_ENABLED=0` 门禁与 Linux 可测性（CLAUDE.md「构建与测试」）原样成立；
无新依赖；命令行为在 macOS 各版本稳定。

**代价。** 进程启动开销（每次密钥操作约几十毫秒）——密钥只在构建 provider 客户端时读取，
不在热路径。`-w` 参数经过 argv，同机其他用户的 `ps` 有短暂可见窗口（见 §4）。

### 2.2 未选中：Security.framework（cgo 或纯 Swift 适配层）

**淘汰原因。** cgo 直接破坏 `CGO_ENABLED=0` 门禁；经 Swift 静态库又把它绑进尚未决定的
宿主形态（09 §9.8 #1）。密钥操作频率极低，为它引入 native 构建链不成比例。

### 2.3 未选中：密钥存 SQLite 或设置文件

**淘汰原因。** 违反 07 §7.3 与 CLAUDE.md「密钥只存系统钥匙串」的硬约束，不作为候选。

## 3. 迁移与既有数据

前端 localStorage 从未持久化密钥（`daygo.providers` 无密钥字段，会话内存 Map 即丢），
因此**不存在存量密钥迁移**；供应商配置本身的 localStorage → SQLite 迁移见 providers
执行册。迁移后首次使用需重新录入密钥。

## 4. 边界与已知空白

- **argv 可见性。** `security -w <secret>` 期间密钥出现在进程参数里。同机多用户场景下
  理论可见，单用户桌面场景风险可接受。若需收紧，改用 `-X` 从 stdin 传 plist，回退路径见 §5。
- **未签名 dev 构建的钥匙串授权框。** 无稳定签名身份的构建首次访问可能触发用户授权弹窗；
  真机验证纳入 providers 执行册的接入清单，签名后（delivery 范围）此问题消失。
- **G-native 范围。** 本文只决定「访问方式」；签名身份与公证后的钥匙串 ACL 行为仍待
  delivery 验收，09 §9.8 #4 不因此完全关闭。

## 5. 回退

`Secrets` 是接口，调用方只依赖端口。若 `security` CLI 被证明不可用（行为变更、权限受限），
替换 `keychain_darwin.go` 内部实现为 `-X` stdin 形式或（在宿主形态 #1 落定后）Swift 适配层，
接口与调用方不动。
