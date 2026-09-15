# providers Linux 密钥：Secret Service + `secret-tool`

> **状态：访问方式已决定，实现已落盘，真实桌面钥环未验收。**

## 1. 问题与约束

Linux 版需要实现 `platform.Secrets`，但不能把 API key 写入 SQLite、localStorage、
环境变量或自有明文文件。Go Core 仍必须保持 `CGO_ENABLED=0`，错误不得包含密钥、
完整命令输出或可还原密钥属性的诊断。

## 2. 选择

Linux 使用 freedesktop Secret Service，通过 libsecret 提供的 `secret-tool` 子进程访问。

- `store --label=Daygo API key service <service> account api-key`；密钥只经 stdin 传入；
- `lookup service <service> account api-key`；只在 Go 内部返回值；
- `clear service <service> account api-key`；
- 每次调用沿用 5 秒超时，不收集 stderr；
- `secret-tool` 不存在时返回 `unsupported`，应用外壳仍可启动；
- lookup / clear 退出码 1 映射为封闭的 `not_found`，其他失败映射为 `native`。

service 名继续使用已冻结的 `io.github.jwz-git.daygo.apikeys.<provider>`，不因平台改名。

## 3. 候选与取舍

| 候选 | 结论 | 原因 |
|---|---|---|
| D-Bus Secret Service 客户端库 | 暂不选 | 会引入新的 D-Bus 依赖与更大协议面；当 `secret-tool` 可用性成为实际问题时再评估 |
| KWallet 专用接口 | 不选 | 会把 KDE 变成第二套产品语义；KWallet 可通过 Secret Service 兼容层提供能力 |
| 自建加密文件 | 否决 | 密钥包装密钥仍需安全存放，只是把问题后移 |
| 存 SQLite / localStorage | 否决 | 违反已有密钥边界 |

## 4. 验收与失败条件

1. 命令夹具证明密钥只进 stdin，不进 argv 或错误。
2. 缺工具、缺条目与原生失败分别映射 `unsupported` / `not_found` / `native`。
3. 真实 Linux 桌面上用专用测试 provider ID 运行写入、覆盖、读取、删除和重启读回；
   GNOME Keyring 与至少一个 KDE 环境分别记录。
4. 锁定、无 session bus、无 `secret-tool` 时不得伪报成功，不得退回明文存储。

当前只有命令夹具与 Linux 交叉构建证据；真实桌面钥环矩阵未运行。
真机验证入口为
`DAYGO_SECRET_SERVICE_TEST=1 go test ./internal/platform/secrets -run TestSecretServiceRealRoundTrip -count=1`。

## 5. 回退

将 Linux `New()` 恢复为 `unavailableSecrets`，Provider 非密钥配置和其他 Go Core 能力仍可使用。
回退不删除已写入 Secret Service 的条目，避免破坏用户数据。
