# recording 跨平台应用身份解析与 ABI

> **状态：两平台有限实现。** 本记录冻结“用户选择一个平台应用”与“已配置 ID 回查展示身份”
> 两条路径的最小边界。它不完成 `System.InstalledApplications`、recorder、占位帧或 MC 隐私
> 矩阵，也不决定其余平台能力的适配形态。

## 1. 决定

### 1.1 选择路径

`PickApplication`（正式绑定）通过 Wails v2 `OpenFileDialog` 调起原生文件面板。macOS 默认打开
`/Applications`。Wails v2 把 `*.app` filter 映射到 `allowedFileTypes` 后会使 `.app` package
呈灰色不可选，因此当前不向面板下发文件 filter；用户仍选择 `.app`，Go 与原生 inspector
负责权威校验，任何非应用输入失败关闭。Wails 返回的路径只在一次绑定调用中使用，不持久化、
不返回前端、不写日志。Windows 调起 Explorer common-item dialog，并用 `*.exe` filter 帮助选择；
原生 inspector 仍会二次校验绝对路径、文件类型和存在性。

Go 通过 `platform.ApplicationInspector.InspectApplication` 调用
[`daygo_application.h`](../../native/include/daygo_application.h) ABI。原生实现使用
Foundation / AppKit：

1. 加载所选 `Bundle`，读取非空 `CFBundleIdentifier`；
2. 通过 `object(forInfoDictionaryKey:)` 读取本地化显示名称；
3. 用 `NSWorkspace.icon(forFile:)` 取图标，绘制到 64×64 私有 bitmap 后编码为 PNG；
4. 只向 Go 返回 `ApplicationIdentity{ID, Name, IconPNG}`。

Windows 实现位于同一 ABI 后面：规范化所选 `.exe` 的最终路径并仅对其做 SHA-256，返回
`win32.exe.sha256:<hex>`（路径本身不跨 Wails 边界）；名称取版本资源并回退文件名，图标由 Shell
取得并用 WIC 编码为 64×64 PNG。截图端按运行进程的规范路径计算同一种 ID，因此设置与原生
WGC 排除不需要第二套身份协议。

`CFBundleIdentifier` 是本接口的身份，而不是代码签名 identifier。ScreenCaptureKit 的
`SCRunningApplication.bundleIdentifier` 正是应用的 bundle identifier；前台兜底也从
`NSRunningApplication.bundleIdentifier` 读取同一种身份。签名完整性不参与屏蔽匹配。

截图 ABI `dg_capture_once` 不变。返回的 `ID` 进入 `privacy.blockedApplicationIds` 后，仍通过
`dg_capture_request_v1.blocked_application_ids` 的完整单次快照进入截图实现。

### 1.2 回查路径

设置只持久化 ID（[03 §3.3.5](../03-data-model.md)）。名称与图标是展示数据，每次读取时由
`GetBlockedApplications` 经 `ApplicationInspector.DescribeApplications` 解析：

- 原生 `dg_application_lookup` 用 `NSWorkspace.urlForApplication(withBundleIdentifier:)`
  找到 bundle 后走同一套身份 + 图标逻辑；
- 系统没有该应用的安装记录时返回 `DG_APPLICATION_E_NOT_FOUND`；
- 解析不到的 ID **保留在列表中**，只回 `{id, name: "", iconDataUrl: ""}`，前端回退显示
  ID 本身——这是唯一已知的标签，不伪造名称；
- Windows 回查优先使用本进程缓存，再检查运行进程以及 `App Paths` / `Uninstall` 注册表；
  便携应用既未运行又没有注册表记录时会诚实降级为 ID-only，而不会保存或猜测路径。

### 1.3 图标边界

图标是装饰，不是身份：没有可加载图标、编码失败或 PNG 超过调用方缓冲区时 `icon_png.len`
为 0，调用本身仍成功。图标在离主线程的私有 bitmap 上绘制，不使用 `DispatchQueue.main.sync`
——Go 宿主的主线程不跑 AppKit run loop，同步派发会死锁。

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

原生层只做平台应用身份解析。对话框、取消语义、超时、DTO、去重和设置写入仍归 Go /
Wails；ABI 使用 caller-owned buffer，不跨语言分配返回字符串，也不保留路径或指针。

Dayflow 的“已安装应用搜索网格 + 已屏蔽列表”仍比文件面板更适合作为最终的“添加应用”交互；但
`System.InstalledApplications` 尚未实现（darwin 适配器返回空列表），因此当前正式设置界面复用
已验证的 picker 交互，并把已配置 ID 的展示身份交给 `DescribeApplications`。

## 3. 当前调用路径

```text
设置页 / 录制联调页
  → PickApplication（正式 Wails binding）
    → Wails OpenFileDialog（默认 /Applications；无文件 filter）
      → platform.ApplicationInspector.InspectApplication
        → cgo → dg_application_inspect
          → Foundation Bundle identifier/名称 + AppKit 图标
    ← ApplicationDTO { id, name, iconDataUrl }
  → 设置页把 id 追加进 privacy.blockedApplicationIds

设置页（读取）
  → GetBlockedApplications（正式 Wails binding）
    → settings.privacy.blockedApplicationIds
      → ApplicationInspector.DescribeApplications
        → cgo → dg_application_lookup（逐个 ID）
    ← []ApplicationDTO（解析不到的条目只带 id）
```

## 4. ABI 与错误

应用 ABI 为 **2.1**。相对 1.0 的变化：`dg_application_info_v2` 增加 `icon_png` buffer，
新增 `dg_application_lookup`，新增错误码 `DG_APPLICATION_E_NOT_FOUND`。major 提升意味着旧
prebuilt archive 会被 Go 侧握手明确拒绝，而不是静默返回缺图标的旧结构。

`dg_application_info_v2` 由调用方提供 identifier / name / icon buffer，原生层只写实际长度。
稳定结果码包括：参数错误、ABI 不匹配、非应用、未找到和未分类原生错误。业务只按 Go
`ApplicationError.Code` 分支，`NativeCode` 仅作本机数值诊断；`ApplicationNotFound` 在
`DescribeApplications` 内部降级为 ID-only 条目，不上抛。

`darwin/windows && !cgo` 和其他平台 factory 返回 `unsupported`，因此不破坏
`CGO_ENABLED=0 go build ./...` 与 Linux 核心门禁。

## 5. 验证与限制

2026-09-11：

- `native/darwin/build.sh` 生成 arm64 + x86_64 universal archive；
- `DAYGO_APPLICATION_SMOKE_PATH=/System/Applications/Calculator.app go test -run TestApplicationInspectorSmoke -v ./internal/platform/darwin`
  经 Go → cgo → Swift → Foundation 返回 `Calculator`、`com.apple.calculator`；
- binding 测试覆盖成功、取消不调用 inspector、非应用输入映射为 `invalid_argument`；
- 前端 typecheck 通过。
- 首轮人工视觉验收发现带 `*.app` filter 时所有应用呈灰色不可选；已移除该 Wails filter，
  由既有 inspector 保持 `.app` 与 Bundle ID 校验。
- VS Code 拒绝问题已定位到资源被扩展修改导致代码签名完整性检查失败；应用本身有
  `com.microsoft.VSCode` identity、Microsoft Developer ID 与 notarization ticket，不是未签名。
- 修复后
  `DAYGO_APPLICATION_SMOKE_PATH="/Applications/Visual Studio Code.app" go test -run TestApplicationInspectorSmoke -v ./internal/platform/darwin`
  返回 `Code` / `com.microsoft.VSCode`；无签名测试 bundle 的回归测试也通过，防止重新引入
  代码签名门禁。

2026-09-13（ABI 2.0 macOS 基线）：

- `DAYGO_APPLICATION_SMOKE_PATH=/System/Applications/Calculator.app go test -run TestApplicationInspectorSmoke -v ./internal/platform/darwin`
  返回 `Calculator` / `com.apple.calculator`，图标 6045 字节 PNG，且同一次运行用
  `com.apple.calculator` 走回查路径得到同一身份；
- `go test ./internal/platform/darwin/` 覆盖未安装 ID 回查降级为 ID-only 条目；
- `go test ./internal/app/` 覆盖 picker 图标 data URL、解析 / 未解析混排顺序、无 store 时
  `database_error`、无解析能力时保留 ID；
- 前端 typecheck / build 通过；无 Wails 桥的 Vite 页用夹具渲染了设置页隐私名单（图标占位
  + 名称 + 删除），点击选择与 `testData=off` 的不可用态均已确认。

2026-09-13（Windows ABI 2.1）：

- Explorer `.exe` picker 已接正式 `PickApplication`；选择 Edge 返回 `Microsoft Edge`、稳定
  `win32.exe.sha256:*` ID 与 2849 字节 PNG，应用 ABI inspect → lookup 往返测试通过；
- 当前系统版本由 `RtlGetVersion` 读取并在设置页展示；build 26100 是 WGC 窗口排除最低门禁；
- 同一 Edge 窗口的基线 JPEG 能看到页面，带所选 ID 的 JPEG 中 Edge 完全消失并露出底层窗口，
  两张图均为 1280×720 非黑画面；原生 smoke、Go adapter/app 测试、前端 typecheck/build 通过。
- 尚未覆盖便携应用重启后且未运行时的名称/图标回查、全部 WC 竞态和 24 小时资源矩阵。

仍未验收：picker 原生面板的视觉与交互、沙盒 / 发行身份、缺失 Bundle ID 的应用、helper / XPC
子进程、多 Space、多显示器与快速前台切换；图标分辨率与暗色模式观感未做视觉验收。选择一个主
`.app` 不保证其 helper 使用相同 ID；MC 隐私矩阵通过前不得宣称应用已被完整屏蔽。

## 6. 回退

删除 `PickApplication` / `GetBlockedApplications` 两个 binding、`ApplicationInspector` 端口与
独立 application ABI 即可回退；`privacy.blockedApplicationIds` 仍是 `[]string`，
`dg_capture_request_v1`、截图文件、数据库 schema 与已保存设置均未改变。
