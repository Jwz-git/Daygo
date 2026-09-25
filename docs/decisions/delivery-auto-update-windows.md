# Windows 自动更新：WinSparkle 0.9.4 + NSIS

> 状态：GitHub Actions 已实现 Windows 安装器自动构建与上传；WinSparkle 客户端适配器已落盘。
> 发布工作流与客户端旧版到新版升级是两项独立验收，后者未附逐项运行记录。

## 决策

Windows 使用 WinSparkle 0.9.4、与 macOS 共用 HTTPS appcast 和 Ed25519 更新签名。更新载荷是现有
`package-windows.ps1` 生成的 NSIS 安装器；不新增第二套安装格式。构建脚本固定下载版本和 SHA-256，
NSIS 将 `WinSparkle.dll` 与 `Daygo.exe`、`daygo_windows_native.dll` 一起安装，三者在有 Authenticode
证书时均先签名再封装。

## 生命周期与安全

`win_sparkle_set_can_shutdown_callback` 只允许同时持有写锁和捕获锁的实例安装，并在返回允许前同步
停止 recorder、收尾活跃分段；随后 `win_sparkle_set_shutdown_request_callback` 走 Wails 真退出。
更新安装器必须通过 Ed25519 验证；有 Authenticode 证书时再提供平台签名。2026-09-25 用户决定
不申请正式平台签名材料，发布工作流不再以 Authenticode 验证阻止 appcast 生成。
Ed25519 私钥只在本机钥匙串和受保护的 GitHub Actions Secret，客户端只包含公钥。无 Authenticode
时的真实安装与升级仍须在 Windows 真机验收，不能因 feed 可用就标记完成。

## 回退与验收

缺 DLL、公钥不合法或发布配置不完整时 factory 返回 nil，设置页明确显示此构建未配置安全更新，
不回退到未验签下载。完成状态要求 Windows 真机执行旧正式版 → 新正式版，验证 NSIS 替换、分段收尾、
锁重获、数据库、Credential Manager 密钥、通知区和卸载保留策略。
