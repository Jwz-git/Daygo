# recording 屏幕截屏 v2：macOS 实现与上层调用

> **状态：已实现的有限原生切片。** 本文记录当前代码已经完成的 macOS 单次截图路径，以及 Go
> 上层应如何调用。它不表示 recorder、数据库 pending 恢复、隐私实机矩阵、G-host、签名公证或
> 长期稳定性已经验收。
>
> C ABI 的字段布局和返回码以 [`native/include/daygo_capture.h`](../../native/include/daygo_capture.h)
> 为唯一事实来源；平台端口以 [`internal/platform`](../../internal/platform) 当前代码为准。

## 1. 这一步完成了什么

本次实现把原先只有 Go 端口和 fake 的截图能力，接到了真实 macOS API：

```text
上层 Go 调用方
  └─ platform.Capture.Capture(ctx, request)
       └─ internal/platform/darwin
            └─ cgo + daygo_capture.h
                 └─ dg_capture_once(...)
                      └─ Swift + ScreenCaptureKit
                           ├─ 权限预检
                           ├─ 隐私前置检查
                           ├─ 解析系统主显示器
                           ├─ SCScreenshotManager.captureImage
                           ├─ ImageIO 编码 JPEG
                           └─ 排他、原子发布结果文件
```

已经落盘的能力：

1. 每次调用只截取一次调用时的系统主显示器；不启动 `SCStream`，没有持续录屏流；
2. 使用 `CGPreflightScreenCaptureAccess` 做运行期权限预检；不会由截图函数主动弹授权框；
3. 屏蔽名单非空时，在截图前检查最前方可见应用；无法可靠识别时失败关闭并返回
   `privacy_unsupported`；
4. `SCContentFilter` 同时排除名单中可解析到的 `SCRunningApplication`；
5. 使用 `SCScreenshotManager.captureImage` 获取一张 `CGImage`；
6. 按 `TargetHeight` 等比计算输出宽度，按 `JPEGQuality` 编码；
7. 在目标目录创建权限为 `0600` 的 sibling `.partial` 文件；编码完成后通过 hard link 排他发布，
   再删除临时目录项，绝不覆盖已有目标；
8. 同步 C ABI 包装 Swift 异步调用，并实现 ABI 版本、结构尺寸、UTF-8 和参数边界校验；
9. cgo wrapper 只把小型参数和结果跨边界传递，像素不进入 Go 内存；
10. macOS `!cgo` 构建返回 `unsupported`，保证 Go Core 的无 cgo 和 Linux 构建门禁；
11. `DAYGO_CAPTURE_DEBUG=1` 可输出不含路径、窗口标题、应用标识或图像内容的阶段日志。

真实 smoke 已验证：1920×1080 主显示器生成 1280×720 JPEG，磁盘解码结果与
`CaptureResult` 的宽、高、字节数一致。隐私两层保护仍需要受控实机矩阵，不能由这次普通截图
替代。

## 2. 当前不在截图函数里的职责

`Capture.Capture` 是无状态原语，不是 recorder。以下工作必须由上层 Go 服务、storage、System
或后续 Media 能力承担：

- 捕获间隔、ticker、暂停、恢复延迟和 recorder 状态机；
- 查询 / 申请屏幕录制权限、打开系统设置；
- 捕获所有者锁和只读实例判断；
- 输出路径分配、pending 记录、幂等入库和启动对账；
- blocked 时生成并保存脱敏占位帧；
- 空闲时间采样、重试、退避、诊断计数和用户可见状态；
- 分段、帧索引、Media 解码、磁盘配额和清理；
- 睡眠、唤醒、锁屏、解锁、屏保、窗口关闭和进程退出编排。

当前应用尚未装配真实 `darwin.Capture`，也没有 recorder 消费者。生产代码不能直接从 Vue 或
Wails binding 调用原生包；调用方向必须保持：

```text
Vue → internal/app binding → Go recorder/service → platform.Capture
```

## 3. Go 上层接口

当前冻结的消费者侧端口：

```go
type Capture interface {
    Capture(ctx context.Context, req CaptureRequest) (CaptureResult, error)
}
```

请求：

```go
type CaptureRequest struct {
    OutputPath            string
    ImageFormat           CaptureImageFormat
    TargetHeight          int
    JPEGQuality           int
    ShowsCursor           bool
    BlockedApplicationIDs []string
}
```

结果：

```go
type CaptureResult struct {
    Outcome    CaptureOutcome
    CapturedAt time.Time
    Width      int
    Height     int
    FileSize   int64
}
```

只有一种 v1 图片格式：

```go
platform.CaptureImageJPEG
```

成功控制结果有两种：

| `Outcome` | error | 文件 | 上层动作 |
|---|---:|---|---|
| `CaptureWritten` | `nil` | 完整 JPEG 已存在 | 校验元数据，幂等入库，清除 pending |
| `CaptureBlocked` | `nil` | 不存在 | 记录脱敏诊断；由 Go 决定是否生成占位帧 |

`blocked` 不是错误。上层不能把它计为 native failure，也不能假定有 JPEG。

## 4. 调用前必须满足的条件

上层每次调用前必须准备一份完整、不可变的请求快照：

1. `OutputPath` 是绝对 `.jpg` / `.jpeg` 路径；
2. 父目录已经存在；
3. 目标文件尚不存在；截图层不会覆盖它；
4. 路径由 Go storage 分配，不由平台层拼接业务目录；
5. `ImageFormat` 为 `platform.CaptureImageJPEG`；
6. `TargetHeight` 在 `1...16384`；
7. `JPEGQuality` 在 `1...100`；
8. `BlockedApplicationIDs` 是本次调用所需的完整 bundle identifier 集合；
9. context 有明确生命周期；recorder 不应把 cgo 调用放进失去所有者的 goroutine；
10. 若需要超过默认 10 秒，使用 deadline；native ABI 最长接受 60 秒。

`CaptureRequest.Validate()` 会在进入 native 前检查平台无关边界，但不会创建父目录、分配路径或
写 pending 记录。

## 5. 最小调用示例

下面的代码反映当前真实 API；可放在 `internal/` 下的临时 smoke 中运行。生产 recorder 还必须
补上 §6 的 pending 事务边界。

```go
package captureexample

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/Jwz-git/Daygo/internal/platform"
)

func CaptureOnce(
    ctx context.Context,
    capture platform.Capture,
    outputPath string,
    blockedApplicationIDs []string,
) (platform.CaptureResult, error) {
    ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
    defer cancel()

    result, err := capture.Capture(ctx, platform.CaptureRequest{
        OutputPath:            outputPath,
        ImageFormat:           platform.CaptureImageJPEG,
        TargetHeight:          720,
        JPEGQuality:           85,
        ShowsCursor:           false,
        BlockedApplicationIDs: blockedApplicationIDs,
    })
    if err != nil {
        if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
            return platform.CaptureResult{}, err
        }

        var captureErr *platform.CaptureError
        if errors.As(err, &captureErr) {
            return platform.CaptureResult{}, fmt.Errorf(
                "capture screenshot (%s): %w",
                captureErr.Code,
                err,
            )
        }
        return platform.CaptureResult{}, fmt.Errorf("capture screenshot: %w", err)
    }

    switch result.Outcome {
    case platform.CaptureWritten:
        return result, nil
    case platform.CaptureBlocked:
        return result, nil
    default:
        return platform.CaptureResult{}, fmt.Errorf(
            "capture screenshot: unexpected outcome %q",
            result.Outcome,
        )
    }
}
```

macOS composition root 最终应注入真实适配器：

```go
capture := darwin.NewCapture()
recorder := recording.NewRecorder(/* repositories, system, */ capture)
```

这两行是目标装配形状，不是当前已经存在的 `recording.NewRecorder` API。当前仓库只有
`darwin.NewCapture()`；在 recorder 切片落盘前，不要为示例伪造 service 或绑定。

## 6. 生产 recorder 的正确事务边界

截图文件和 SQLite 不能形成一个共同的原子事务，因此上层应使用 pending 状态协调。以下是行为
顺序，不是当前已实现的 repository API：

```text
1. recorder 确认本实例持有 capture-owner 锁
2. storage 创建 pending capture，并分配唯一绝对 OutputPath
3. recorder 读取本次完整设置快照
4. 调用 Capture(ctx, request)
5a. written：校验文件 / result → 幂等写 screenshots → 删除 pending
5b. blocked：不期待文件 → 记录安全诊断 → 删除 pending → 可生成占位帧
5c. error：确认没有可接受结果 → 保留或标记 pending，按错误分类决定恢复 / 重试
6. 启动时只对账 storage 已登记的 pending 路径，不扫描和接纳未知文件
```

关键不变量：

- 数据库提交前崩溃：已发布 JPEG 与 pending 一起留给启动恢复；
- 数据库提交后崩溃：幂等键防止重复 screenshot 行；
- 超时或 context 取消：过期结果不能写入业务数据；
- `CaptureBlocked`：不创建真实截图，不把“无文件”误判为 I/O 故障；
- output collision：返回 `io`，不得覆盖另一调用或未知文件；
- 上层日志不得记录 `OutputPath`、窗口标题、屏幕内容或应用活动明细。

## 7. 错误处理

`CaptureError.Code` 是业务可分支的稳定分类；`NativeCode` 只用于本地数值诊断：

| Code | 含义 | 推荐上层处理 |
|---|---|---|
| `invalid_argument` | 请求边界错误 | 编程 / 配置错误，不重试同一请求 |
| `abi_mismatch` | Go 与静态库 ABI major 不同 | 禁用 capture，要求修复构建产物 |
| `unsupported` | 当前构建或字段不支持 | 禁用该能力；`!cgo` 会返回此值 |
| `permission_denied` | 当前宿主没有 TCC 权限或运行中被撤权 | 更新状态，提示用户授权，不自动循环请求 |
| `no_display` | 调用时没有系统主显示器 | 等显示器事件或有界退避 |
| `timeout` | native 超过本次 ABI timeout | 丢弃本次结果；上层按 recorder 策略退避 |
| `io` | 临时文件、编码或发布失败 | 检查磁盘 / 路径状态后有界重试 |
| `privacy_unsupported` | 无法可靠执行本次隐私前置条件 | 失败关闭，绝不降级为无屏蔽截图 |
| `native` | 未分类 Apple 错误 | 记录稳定 code 与数值 NativeCode，有界退避 |

context 规则：

- 进入 cgo 前 context 已取消：直接返回 `ctx.Err()`；
- 进入 cgo 后显式取消不能中断同步 C 调用；wrapper 等 native 返回后优先返回 `ctx.Err()`；
- native 成功发布但 Go 随后发现 context 已取消：wrapper 删除该结果；
- ABI 自身使用 context deadline 的剩余时间；无 deadline 默认 10 秒，最大 60 秒；
- 超时时撤销发布权限。已经提交给 ScreenCaptureKit 的系统回调可能短暂排空，但不能在 ABI 返回后
  发布文件或写调用方输出指针。

## 8. macOS 具体实现

### 8.1 主显示器和尺寸

Swift 通过 `CGMainDisplayID()` 获取调用时的系统主显示器 ID，再从 `SCShareableContent.displays`
中选择匹配项。调用方不传 display ID；显示器切换后的下一次调用自然使用新的主显示器。

输出宽度计算：

$$
\text{width}=\operatorname{round}\left(\frac{\text{displayWidth}\times\text{targetHeight}}
{\text{displayHeight}}\right)
$$

最小宽度为 1。`CaptureResult.Width/Height` 使用实际 `CGImage` 尺寸，不使用预估值。

### 8.2 隐私双保护

屏蔽名单为空时，不执行前台应用查询。名单非空时：

1. 截图内容枚举前检查一次最前方可见应用；
2. `SCShareableContent` 返回后、创建 filter 前再检查一次；
3. 任一次命中都返回 `CaptureBlocked`，且不调用截图 API；
4. 未命中时仍把名单对应的 `SCRunningApplication` 放进
   `SCContentFilter(excludingApplications:)`。

前台识别使用 WindowServer 的有序可见窗口列表取得 PID，再使用 Security framework 读取签名
identifier。这样不依赖 AppKit MainActor，CLI smoke 和 Wails 宿主都能调用。无法取得可靠标识时
返回 `privacy_unsupported`。

这缩小但不能数学上消除“最终检查后切换前台应用”的竞态；因此仍须执行真实快速切换、后台窗口、
多空间、多屏和系统窗口矩阵。

### 8.3 原子文件发布

当前实现不是覆盖式 rename：

1. 在目标目录创建唯一 `.partial` 文件，权限 `0600`；
2. ImageIO 完成 JPEG 编码并 finalize；
3. 在取消锁内执行 `link(temp, output)`；目标已存在时原子失败；
4. `unlink(temp)`，只保留最终目录项；
5. 查询最终文件大小后返回。

同目录 hard link 保证发布不会跨文件系统，也不会覆盖未知文件。任何失败都会清理本次临时文件；
临时目录项删除失败时同时删除本次最终链接并返回 `io`。

## 9. cgo、构建与应用装配

正常 macOS 路径使用 cgo：

```text
internal/platform/darwin/bridge_darwin.go
  ├─ include native/include/daygo_capture.h
  ├─ link build/native/darwin/universal/libdaygo_capture.a
  └─ link CoreGraphics / Foundation / ImageIO / ScreenCaptureKit /
     Security / UniformTypeIdentifiers / Swift runtime
```

构建静态库：

```bash
native/darwin/build.sh
```

脚本分别编译 arm64、x86_64，然后用 `lipo` 生成：

```text
build/native/darwin/universal/libdaygo_capture.a
```

Wails 构建通过 `cmd/daygo/wails.json` 的 Darwin `preBuildHooks` 调用该脚本。直接运行 Go smoke 时
必须先执行脚本。

`internal/platform/darwin/unavailable_darwin.go` 只服务于：

```text
darwin && !cgo
```

它不是正常运行路径，也不表示移除了 cgo。它使下列门禁不依赖 macOS framework：

```bash
CGO_ENABLED=0 go build ./...
GOOS=linux CGO_ENABLED=0 go build ./internal/...
```

## 10. 调试和 smoke

详细阶段日志默认关闭。显式启用：

```bash
native/darwin/build.sh
DAYGO_CAPTURE_DEBUG=1 go run -a ./tmp/capture-smoke
```

`-a` 用于强制 Go 重新链接刚构建的静态库；否则 Go 缓存可能继续使用旧 archive。

关键阶段：

```text
abi.enter
permission.preflight.granted
content.query.completed
display.resolved.1920x1080
screenshot.capture.completed.1280x720
jpeg.write.completed.<bytes>
abi.success
```

日志停在 `abi.timeout` 表示同步 ABI 未在 timeout 内拿到任务结果。2026-09-10 的一次已修复原因是：
空屏蔽列表仍无条件 `await` AppKit `@MainActor` 前台查询，而 CLI smoke 没有 AppKit 主事件循环。
当前实现对空列表跳过检查，并彻底移除了该 MainActor 依赖。

成功后检查：

```bash
open /tmp/daygo-capture-smoke.jpg
```

屏蔽验证示例（在 VS Code 集成终端保持 VS Code 前台）：

```bash
DAYGO_CAPTURE_DEBUG=1 \
DAYGO_BLOCKED_APP_ID=com.microsoft.VSCode \
go run -a ./tmp/capture-smoke
```

预期 `CaptureBlocked`，且 `/tmp/daygo-capture-smoke.jpg` 不存在。此项必须记录实际前台应用、系统
版本和结果，普通截图成功不能代替隐私验收。

## 11. 当前门禁和下一步

已经验证：

- Swift arm64 / x86_64 通用静态库构建；
- Go → cgo → C ABI → Swift → ScreenCaptureKit 真实调用；
- 真实 JPEG 可解码，结果元数据与文件一致；
- `go test ./...`、`go vet ./...`；
- macOS `CGO_ENABLED=0 go build ./...`；
- Linux `CGO_ENABLED=0 go build ./internal/...` 交叉构建。

仍未验证或实现：

- recorder 和真实适配器应用装配；
- capture-owner 锁、pending repository 与启动恢复；
- 前台屏蔽、后台屏蔽窗口、快速切换和多空间隐私矩阵；
- 多显示器、旋转、HDR、睡眠 / 唤醒、锁屏 / 解锁；
- 关闭窗口后 10 分钟持续离散捕获与状态栏重开，即 G-host；
- 开发 / Release / 升级签名的 TCC 身份；
- 24 小时资源和捕获指示观察，即 [08 §8.6.2](../08-testing-strategy.md#862-mc真实-macos-捕获矩阵) 的 MC-1…MC-12；
- Windows 适配已另有实现，但未验证且不在发布范围，见
  [Windows 截图实现与限制](recording-screen-capture-windows.md)。

回退方式：停止在 composition root 注入真实 `darwin.Capture`，保留 Go recorder、fake 和权限 UI；
不得回退到持续 `SCStream`、覆盖式文件写入或隐私失败后继续截图。
