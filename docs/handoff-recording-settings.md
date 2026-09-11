# 录制设置与状态栏交接

> **状态：本轮已实现；用户已确认 dev 基本功能正常，production 与长期门禁仍待验收。**
>
> 本文记录最近一轮关于录制设置、主页控制和菜单栏状态同步的实现事实。它不是 recording
> 模块完成声明；完整验收仍以 [`modules/recording.md`](modules/recording.md) 和
> [`09-roadmap.md`](09-roadmap.md) 为准。

## 1. 本轮完成内容

### 1.1 存储设置

`frontend/src/views/Settings/StorageSection.vue` 现在通过真实的 `GetSettings` /
`UpdateSettings` 路径读取和写入：

- 截图间隔：1、5、10、20、30、60 秒；
- 截图高度：720、1080 像素；
- 录制占用上限：GB 或不限；
- 录制目录：由 Go 端返回并只读展示。

中英文设置文案已补齐，避免 vue-i18n 在界面上直接显示
`settings.storage.intervalOption` 等 key。

设置写入失败时，`useSettingsSection` 会重新读取后端权威值并暴露失败状态；存储页面会显示
错误提示，不继续显示未经确认的乐观值。

### 1.2 录制目录

正式录制目录不是前端硬编码，也不是提交到仓库的开发者路径。Go 端在运行时按以下逻辑解析：

```text
os.UserConfigDir()
  → <用户应用配置目录>/Daygo
  → <该目录>/recordings
```

macOS 典型结果为：

```text
~/Library/Application Support/Daygo/recordings/
```

`GetRecordingDirectory` 是只读 Wails binding，并已加入接口契约。录制器和设置展示使用同一
应用支持目录来源，因此不会出现“页面展示一个目录、recorder 写入另一个目录”的分叉。

截图 ABI 联调页的 `/tmp/daygo-capture-test` 仍是独立的临时测试输出目录；它不是正式后台录制
目录，也不应被误认为用户数据目录。

### 1.3 运行中设置生效

`internal/recorder.Recorder.UpdateSettings` 已接入 `Backend.UpdateSettings`。录制运行中修改
截图间隔、截图高度或隐私名单后：

- 不重启 recorder；
- 后续 tick 使用新的截图间隔；
- 后续截图使用新的高度和屏蔽名单；
- 当前生命周期状态保持不变。

对应行为由 `TestRecorderUpdateSettingsAffectsNextCapture` 覆盖。

### 1.4 主页录制控制和状态同步

主页是 Timeline 页面。主页右上角保留“开始录制”按钮；录制开始后显示实时状态，而不是
只显示启动意愿：

```text
idle → starting → capturing
capturing ↔ paused
```

前端通过 `recording:state` 事件订阅 Go recorder 状态，并在挂载时调用
`GetRecordingState` 做一次权威读取。页面离开时取消订阅，避免重复监听。

### 1.5 菜单栏 Pause 修复

此前每次状态变化都重新创建 `NSStatusItem`。Pause 触发状态刷新后，旧状态栏对象被释放，
因此菜单栏项目消失。

`native/darwin/Sources/StatusItemABI.swift` 现在复用已有的 `StatusController` 和
`NSStatusItem`，只更新按钮标题、Pause/Resume 菜单标题及 enabled 状态。菜单栏项目不会因
状态变化而被重建。

Go 端对菜单动作的处理为：

- idle：开始录制；
- capturing：暂停；
- paused：恢复；
- open：显示 Daygo 窗口；
- quit：退出应用。

Pause 和 Resume 绑定现在也检查 capture-owner，非捕获实例不会伪装成可控制录制的实例。

## 2. 代码路径

```text
设置页面
  frontend/src/views/Settings/StorageSection.vue
    → frontend/src/views/Settings/useSettingsSection.ts
      → frontend/src/api/settings.ts
        → GetSettings / UpdateSettings
          → internal/app/api_settings.go
            → internal/settings
              → internal/storage app_settings

主页
  frontend/src/views/Timeline/TimelineView.vue
    → frontend/src/api/recording.ts
      → GetRecordingState / SetRecording
      → recording:state

菜单栏
  native StatusItemABI.swift
    → cgo status bridge
      → internal/platform/darwin
        → internal/app.startSystemEventPump
          → PauseRecording / ResumeRecording
            → internal/recorder
```

## 3. 已验证证据

以下命令已通过：

```bash
./scripts/gate.sh
```

其中包括：

- Go build、Go tests、`go vet`、gofmt 检查；
- recorder 运行中设置回归测试；
- 前端 unit tests、`vue-tsc`、Vite production build；
- 文档链接与孤立文档检查。

另外已运行：

```bash
native/darwin/build.sh
go run github.com/wailsapp/wails/v2/cmd/wails@v2.15.0 build \\
  -clean -platform darwin/arm64
```

打包应用的 arm64 Mach-O 可执行文件已生成，应用进程可启动。`wails dev` 也可启动，状态栏
初始化位于 Wails `OnStartup`，因此菜单栏逻辑不要求 production build。

## 4. 尚未完成或尚未通过的事项

本轮没有把以下事项标为完成：

1. **production 与完整菜单矩阵**：用户已确认 dev 基本功能正常，可达到当前阶段验收程度；仍需对
   production 包重复开始录制、Pause、Resume，记录状态栏持续存在、菜单标题、页面 `paused`
   状态和暂停期间帧计数等匿名证据。
2. **G-host 长期门禁**：关闭窗口后持续捕获至少 10 分钟、从状态栏重开窗口、激活策略切换
   和后台稳定性仍未完成。
3. **录制完整闭环**：真实系统权限请求仍未实现；系统事件桥与 recorder 处理已有代码，但
   睡眠/锁屏/屏保恢复的真实矩阵未验收；分段 Media、完整清理和长期资源观察仍未完成。
4. **隐私实机矩阵**：屏蔽应用、双显示器、旋转/HDR、受保护内容、前台快速切换等 MC 项仍未
   完成；fake 和单次截图 smoke 不能替代这些证据。
5. **用户可选录制目录**：当前目录按应用支持目录运行时解析，设置页只展示不可编辑路径。
   如果未来允许用户迁移目录，必须先设计数据迁移、旧数据兼容、资源白名单、备份和回滚，
   不能只把路径改成一个文本设置。
6. **前端开发 fixture 的正式录制控制**：Vite 浏览器预览可以展示设置页面，但没有真实 Wails
   bridge；正式录制目录和 recorder 行为只能在 Wails 应用内验证。
7. **Swift actor isolation 警告**：原生构建目前有 AppKit 属性在非隔离上下文访问的编译警告，
   不阻塞当前构建，但后续应按当前 Swift/AppKit SDK 的 MainActor 约束清理。

## 5. 下一步建议

按风险顺序：

1. 在 production Wails 应用中重复 dev 已通过的菜单和暂停矩阵，并保存匿名结果；
2. 补真实 system permission，实现后运行 system event 的睡眠、锁屏和屏保矩阵；
3. 接入 Media 分段前，先验证 pending capture 的崩溃恢复、整段清理和重放去重；
4. 只有完成 G-host 与相关 MC 门禁后，再扩张录制设置和常驻 UI；
5. 用户可选目录另立决策记录，不与当前应用支持目录方案混合。

## 6. 回退方式

- 前端录制控制可暂时隐藏，不影响 Timeline 查询和 CaptureTest 联调页；
- 停止调用 recorder 后，保留已写入的数据库和 staging 文件，按录制存储决策执行对账与清理；
- 菜单栏更新可以回退为只在生命周期边界设置一次，但不得恢复“状态刷新即重建并释放
  `NSStatusItem`”的实现；
- 录制目录继续使用应用支持目录，禁止通过删除目录或重置数据库进行回退。
