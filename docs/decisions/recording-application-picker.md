# recording macOS 应用选择与 Bundle ID ABI

> **状态：有限实现。** 本记录只冻结“用户选择一个 `.app` 后，解析截图隐私名单所需稳定身份”
> 的最小边界。它不完成 `System.InstalledApplications`、正式隐私设置页、recorder、占位帧或
> MC 隐私矩阵，也不决定其余平台能力的适配形态。

## 1. 决定

临时 `CaptureTest` 页面通过 Wails v2 `OpenFileDialog` 调起 macOS `NSOpenPanel`，默认打开
`/Applications`。Wails v2 把 `*.app` filter 映射到 `allowedFileTypes` 后会使 `.app` package
呈灰色不可选，因此当前不向面板下发文件 filter；用户仍选择 `.app`，Go 与原生 inspector
负责权威校验，任何非应用输入失败关闭。Wails 返回的路径只在一次绑定调用中使用，不持久化、
不返回前端、不写日志。

Go 通过独立的 `platform.ApplicationInspector` 调用
[`daygo_application.h`](../../native/include/daygo_application.h) ABI。原生实现使用 Foundation：

1. 加载所选 `Bundle`，读取非空 `CFBundleIdentifier`；
2. 通过 `object(forInfoDictionaryKey:)` 读取本地化显示名称；
3. 只向 Go 返回 `AppInfo{ID, Name}`。

`CFBundleIdentifier` 是本接口的身份，而不是代码签名 identifier。ScreenCaptureKit 的
`SCRunningApplication.bundleIdentifier` 正是应用的 bundle identifier；前台兜底也从
`NSRunningApplication.bundleIdentifier` 读取同一种身份。签名完整性不参与屏蔽匹配。

截图 ABI `dg_capture_once` 不变。返回的 `ID` 加入现有 `BlockedApplicationIDs` 后，仍通过
`dg_capture_request_v1.blocked_application_ids` 的完整单次快照进入截图实现。

## 2. 为什么不校验代码签名

代码签名完整性与截图过滤身份是两个问题。`SecStaticCodeCheckValidity` 会验证 bundle 的 sealed
resources；只要扩展或本地定制修改过应用资源，即使应用仍有稳定的 `CFBundleIdentifier`，该调用也会
失败。2026-09-11 的本机证据：VS Code 的签名 identifier 与 bundle identifier 均为
`com.microsoft.VSCode`，Developer ID 和 notarization ticket 均存在，但 Custom UI Style 扩展修改 /
新增资源后，`codesign --verify --verbose=4` 报 `a sealed resource is missing or invalid`。旧实现因此
错误拒绝了仍能被 ScreenCaptureKit 识别和排除的应用。

[Dayflow 同类实现](https://github.com/JerryZLiu/Dayflow/blob/09b9c7eb8c738bbbaa504a6d285ef3d9c76d0af8/Dayflow/Dayflow/Core/Recording/RecordingPrivacyPreferences.swift)
也直接从已安装 bundle、`NSRunningApplication` 和 `SCRunningApplication` 使用
`bundleIdentifier`，不把代码签名验证作为隐私名单的接入条件。Daygo 采用这一更贴合
ScreenCaptureKit 接口的身份边界，但保留自身既有的两次前台检查、失败关闭和严格 `.app` 路径校验。

原生层只做 macOS bundle 身份解析。对话框、取消语义、超时、DTO、去重和设置写入仍归 Go /
Wails；ABI 使用 caller-owned buffer，不跨语言分配返回字符串，也不保留路径或指针。

Dayflow 的“已安装应用搜索网格 + 已屏蔽列表”比文件面板更适合作为正式设置界面；但当前
`CaptureTest` 是有限联调页，仓库的 `System.InstalledApplications`、正式设置持久化和 G-host
尚未过门禁。此处不把应用扫描与第二套设置状态临时塞进测试页；正式 recording 设置切片应复用
该交互结构并通过既定 System 端口供数。

## 3. 当前调用路径

```text
CaptureTest Vue
  → PickCaptureTestApplication Wails binding
    → Wails OpenFileDialog（默认 /Applications；无文件 filter）
      → platform.ApplicationInspector
        → cgo → dg_application_inspect
          → Foundation Bundle identifier
    ← CaptureTestApplicationDTO { id, name }
  → 把 id 加入临时 blockedApplicationIds 文本框
  → 现有 CaptureTest → dg_capture_once
```

正式设置页接入时只保存 ID；名称是展示信息，路径不是产品数据。

## 4. ABI 与错误

应用 ABI 独立版本为 `1.0`。`dg_application_info_v1` 由调用方提供 identifier/name buffer，原生层
只写实际长度。稳定结果码包括：参数错误、ABI 不匹配、非应用和未分类原生错误。业务只按 Go
`ApplicationError.Code` 分支，`NativeCode` 仅作本机数值诊断。

`darwin && !cgo` 和非 macOS factory 返回 `unsupported`，因此不破坏
`CGO_ENABLED=0 go build ./...` 与 Linux 核心门禁。

## 5. 验证与限制

2026-09-11：

- `native/darwin/build.sh` 生成 arm64 + x86_64 universal archive；
- `DAYGO_APPLICATION_SMOKE_PATH=/System/Applications/Calculator.app go test -run TestApplicationInspectorSmoke -v ./internal/platform/darwin`
  经 Go → cgo → Swift → Foundation 返回 `Calculator`、`com.apple.calculator`；
- binding 测试覆盖成功、取消不调用 inspector、非应用输入映射为 `invalid_argument`；
- 前端 typecheck 通过，Wails 生成绑定含 `PickCaptureTestApplication` 和
  `CaptureTestApplicationDTO`。
- 首轮人工视觉验收发现带 `*.app` filter 时所有应用呈灰色不可选；已移除该 Wails filter，
  由既有 inspector 保持 `.app` 与 Bundle ID 校验，修复后的原生面板等待复验。
- VS Code 拒绝问题已定位到资源被扩展修改导致代码签名完整性检查失败；应用本身有
  `com.microsoft.VSCode` identity、Microsoft Developer ID 与 notarization ticket，不是未签名。
- 修复后
  `DAYGO_APPLICATION_SMOKE_PATH="/Applications/Visual Studio Code.app" go test -run TestApplicationInspectorSmoke -v ./internal/platform/darwin`
  返回 `Code` / `com.microsoft.VSCode`；无签名测试 bundle 的回归测试也通过，防止重新引入
  代码签名门禁。

仍未验收：Wails 原生面板视觉与交互、沙盒 / 发行身份、缺失 Bundle ID 的应用、helper / XPC
子进程、多 Space、多显示器与快速前台切换。选择一个主 `.app` 不保证其 helper 使用相同 ID；
MC 隐私矩阵通过前不得宣称应用已被完整屏蔽。

## 6. 回退

删除临时 picker binding、`ApplicationInspector` 端口与独立 application ABI 即可回退；
`dg_capture_request_v1`、截图文件、数据库 schema 和已保存设置均未改变。
