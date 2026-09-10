# recording 屏幕截屏：单次调用契约与原生 ABI

> **状态：recording 原生实验规格。** 本文只定义“一次调用截取一张图片”的平台能力、Go 端口和
> C ABI。真实授权、捕获指示、隐私遮蔽、签名、公证和长时间运行实验通过前，平台实现仍是
>「待定设计」。
>
> **当前代码事实。** `internal/platform` 仍保留早期的长生命周期 `Start/Stop/Events/Ack`
> 草案；它不是本文目标接口。按本次范围不修改该代码和其它规范，后续实现前必须单独迁移，不能
> 同时保留两套捕获契约。`native/include/daygo_capture.h` 是 C ABI 的唯一事实来源。

## 1. 决定

Daygo 的原生截屏层是一个**无会话、调用方驱动的单次函数**：

```text
Go recorder
  └─ Capture(ctx, request)
       └─ dg_capture_once(...)
            ├─ 检查本次隐私条件
            ├─ 调用一次系统截屏 API
            ├─ 编码一张图片
            └─ 原子发布到 request.OutputPath
```

原生层不持续运行，不拥有定时器，不保存 recorder 状态，也不产生异步事件。每次调用最多生成
一张完整图片；返回后不再持有调用方内存或本次截图资源。

| 项 | 决定 | 原因 |
|---|---|---|
| 调度 | Go 拥有 timer/worker | 间隔、暂停、恢复和重试是产品逻辑 |
| Go 端口 | `Capture(ctx, request) (result, error)` | 与“一次调用、一张图片”一致 |
| 截图目标 | 每次调用时的系统主显示器 | 没有显示器枚举、选择、光标跟随或跨屏状态 |
| ABI | 同步 C 函数 `dg_capture_once` | 没有 handle、回调、poll、ack 或 native 状态机 |
| 像素交接 | 原生层写调用方指定的文件 | 像素不进入 Go/cgo，避免大块内存复制 |
| 正常交付 | Go 使用已知路径，不扫描目录 | 请求与结果天然一一对应 |
| 崩溃恢复 | Go 的 pending 记录与 staging 目录对账 | 可靠性归 SQLite 唯一写入方，不在 native 复制日志 |
| v1 图片 | JPEG，质量由每次请求给出 | macOS 14 和 Windows 均可实现，便于后续批处理 |
| macOS | Swift + `SCScreenshotManager.captureImage` | 真正的一次性截图，不启动持续 `SCStream` |
| Windows | 继续实验 WGC/DXGI | 隐私双保护和捕获指示未验证前不发布 |

## 2. 职责边界

### 2.1 原生截屏层唯一职责

一次 `Capture` 调用只做这些工作：

1. 校验本次请求和 ABI 版本；
2. 在每次调用中解析当前系统主显示器；
3. 在真正截图前检查前台应用是否命中本次传入的屏蔽列表；
4. 命中时不调用系统截图 API、不创建文件，并返回 `blocked`；
5. 未命中时把屏蔽应用排除规则应用到本次系统截图；
6. 调用一次系统截图 API；
7. 按目标高度等比缩放，编码为 JPEG；
8. 在目标目录写临时文件，关闭后原子发布为请求路径；
9. 返回这张图片的采集时间、尺寸和文件大小；
10. 在返回前释放本次调用创建的系统对象和临时内存。

前台兜底与画面排除是截图时的隐私前置条件，不是长期业务状态。屏蔽 ID 必须由 Go 在每次调用
中完整传入；原生层不得读设置。

### 2.2 明确不属于原生截屏层

以下职责全部在 Go、storage、Media 或 System 层：

- 捕获间隔和 timer；
- `idle/starting/capturing/paused` 状态机；
- 用户开关、定时暂停和自动恢复；
- sleep/wake、lock/unlock、屏保和退出编排；
- 唤醒 5 秒、解锁 0.5 秒等恢复策略；
- 并发限制、重试、退避、故障熔断和诊断计数；
- 空闲时间采样、批处理、LLM 调用和时间线生成；
- SQLite、pending 记录、幂等插入和启动恢复；
- 占位帧的生成与保存；
- 分段轮换、HEVC 编码、帧索引、缩略图和 timelapse；
- 权限状态查询、权限申请和打开系统设置；
- 文件清理和磁盘配额。

因此本 ABI 不包含 `create/start/stop/poll/ack/destroy`，不包含事件、channel、序号、分段信息、
权限接口或 recorder phase。

## 3. 上层 Go 接口

本文定义目标接口；迁移 `internal/platform` 时必须一次性删除旧契约及其类型、helper 和测试，
不得保留兼容别名。

```go
package platform

import (
    "context"
    "time"
)

type Capture interface {
    Capture(ctx context.Context, req CaptureRequest) (CaptureResult, error)
}

type CaptureImageFormat string

const (
    CaptureImageJPEG CaptureImageFormat = "jpeg"
)

type CaptureOutcome string

const (
    CaptureWritten CaptureOutcome = "written"
    CaptureBlocked CaptureOutcome = "blocked"
)

type CaptureRequest struct {

    // OutputPath 必须是绝对路径。父目录已存在，最终路径在调用前不存在。
    OutputPath string

    // ImageFormat 在 v1 只允许 CaptureImageJPEG。
    ImageFormat CaptureImageFormat

    // TargetHeight 是输出像素高度。原生层保持宽高比，不裁切、不拉伸。
    TargetHeight int

    // JPEGQuality 取 1..100。
    JPEGQuality int

    ShowsCursor bool

    // 每次调用的完整快照。值是 opaque UTF-8 应用 ID。
    BlockedApplicationIDs []string
}

type CaptureResult struct {
    Outcome CaptureOutcome

    // 以下字段只在 Outcome == CaptureWritten 时有效。
    CapturedAt time.Time
    Width      int
    Height     int
    FileSize   int64
}

type CaptureErrorCode string

const (
    CaptureInvalidArgument    CaptureErrorCode = "invalid_argument"
    CaptureABIMismatch        CaptureErrorCode = "abi_mismatch"
    CaptureUnsupported        CaptureErrorCode = "unsupported"
    CapturePermissionDenied   CaptureErrorCode = "permission_denied"
    CaptureNoDisplay          CaptureErrorCode = "no_display"
    CaptureTimeout            CaptureErrorCode = "timeout"
    CaptureIO                 CaptureErrorCode = "io"
    CapturePrivacyUnsupported CaptureErrorCode = "privacy_unsupported"
    CaptureNative             CaptureErrorCode = "native"
)

type CaptureError struct {
    Code       CaptureErrorCode
    NativeCode int64 // 仅诊断；业务分支只能使用 Code
}
```

`CaptureBlocked` 是成功的控制结果，不是错误。它表示本次调用在读取屏幕像素前被隐私兜底阻止。
Go 收到后负责生成脱敏占位帧；不得把“没有文件”记成丢帧或自动绕过屏蔽规则重试。

### 3.1 调用不变量

1. 同一个调用只对应一个请求路径；成功返回后该路径存在且是完整、可解码的 JPEG；
2. `CaptureWritten` 时图片路径就是 `CaptureRequest.OutputPath`，结果不重复返回路径；
3. `CaptureBlocked` 时最终路径和临时文件都不存在；
4. error 非 nil 时最终路径不存在；原生层尽力删除本次临时文件；
5. 原生层从不覆盖已有最终文件；路径已存在映射为 `CaptureIO`；
6. 调用期间输入为借用内存，函数返回后原生层不保留任何 Go 指针；
7. Go 保证同一个 recorder 最多一个在途调用；原生 ABI 的不同调用之间没有会话状态；
8. 截图成功后 Go 必须再次检查 recorder generation。调用期间若发生暂停或关闭，Go 删除该文件，
   不把过期结果写入业务数据；
9. 图片内容、窗口标题、应用标题和路径不得进入错误、日志或指标；
10. `CapturedAt` 是采集时间。平台不给时间戳时，使用系统截图调用开始与完成墙钟的中点，不使用
    Go 收到返回值的时间。

### 3.2 context 与超时

C ABI 是同步调用，不能持有 Go `context.Context`。Go wrapper 按以下规则适配：

1. 调用前检查 `ctx.Err()`；已取消则不进入 native；
2. 有 deadline 时把剩余毫秒传入 `timeout_ms`，无 deadline 时使用 recorder 的有界默认值；
3. 原生层在超时后取消本次平台任务、清理临时文件并返回 `timeout`；
4. 显式 cancel 若发生在已经进入 C 之后，不另起 goroutine 假装取消。wrapper 等待本次 native 调用
   收尾，返回时优先报告 `ctx.Err()`，并删除可能已发布但不再接受的文件；
5. recorder 关停必须等待这一个在途调用完成或超时，不允许遗留无所有者的 native worker。

## 4. 文件交接与恢复

### 4.1 正常写入

`OutputPath` 由 Go/storage 分配。原生层不得选择业务目录或扫描已有文件。

```text
<output>.daygo-<random>.partial
    ├─ create exclusive
    ├─ encode JPEG
    ├─ flush + close
    └─ atomic rename without replace
         ↓
<output>
```

契约：

- 父目录由 Go 创建；原生层不递归建目录；
- 临时文件与最终文件位于同一目录，保证 rename 不跨文件系统；
- 文件权限为当前用户独占；POSIX 平台使用 `0600`；
- 最终路径必须不存在，rename 不允许覆盖；
- `CaptureWritten` 只在 rename 完成后返回；
- `FileSize` 是最终文件关闭后的实际字节数；
- “完整可读”是调用成功条件；每次截图强制磁盘 `fsync` 不属于 ABI，由 storage 的耐久策略决定。

### 4.2 Go 侧崩溃恢复

正常路径不扫描目录。为了覆盖“文件已发布、SQLite 尚未提交”窗口，Go 在调用前建立带稳定
capture ID 和目标路径的 pending 记录，调用完成后幂等提交或删除 pending。启动时仅对账 pending
记录与 staging 目录：

- pending + 完整最终文件：验证 JPEG 后补交；
- pending + `.partial`：删除临时文件并按 recorder 策略重试；
- pending + 无文件：标记本次未完成；
- 最终文件 + 无 pending/业务行：作为 orphan 隔离或删除，不猜测采集时间；
- 已提交业务行：按 capture ID/路径幂等，不重复插入。

这套恢复属于 Go/storage，不向 native 增加 event journal、sequence 或 ack。

## 5. C ABI v1

两个平台共享 `native/include/daygo_capture.h`。头文件是布局、常量和导出符号的唯一来源；本文
只解释其语义，不维护第二份完整头文件副本。

### 5.1 导出函数

```c
void dg_capture_abi_version(uint32_t *major, uint32_t *minor);

int32_t dg_capture_once(
    uint32_t requested_abi_major,
    const dg_capture_request_v1 *request,
    dg_capture_result_v1 *out_result,
    dg_capture_error_v1 *out_error
);
```

`dg_capture_abi_version` 允许任一输出指针为 `NULL`。`dg_capture_once` 是唯一执行平台能力的函数：
同步截取并发布一张图片，或返回 blocked/error。

### 5.2 返回码

| C 返回码 | Go 映射 | 是否产生文件 | 语义 |
|---|---|---:|---|
| `DG_CAPTURE_OK` | `CaptureWritten` | 是 | 截图、编码、原子发布均成功 |
| `DG_CAPTURE_BLOCKED` | `CaptureBlocked` | 否 | 前台应用命中屏蔽列表，未读取屏幕像素 |
| `DG_CAPTURE_E_INVALID_ARGUMENT` | `CaptureInvalidArgument` | 否 | 字段、UTF-8、长度、范围或路径不合法 |
| `DG_CAPTURE_E_ABI_MISMATCH` | `CaptureABIMismatch` | 否 | major 不兼容或 struct 太小 |
| `DG_CAPTURE_E_UNSUPPORTED` | `CaptureUnsupported` | 否 | 格式、flag 或平台能力不支持 |
| `DG_CAPTURE_E_PERMISSION_DENIED` | `CapturePermissionDenied` | 否 | 调用时无屏幕录制授权 |
| `DG_CAPTURE_E_NO_DISPLAY` | `CaptureNoDisplay` | 否 | 当前没有可用的系统主显示器 |
| `DG_CAPTURE_E_TIMEOUT` | `CaptureTimeout` | 否 | 本次平台调用超过 `timeout_ms` |
| `DG_CAPTURE_E_IO` | `CaptureIO` | 否 | 创建、编码、关闭或 rename 失败 |
| `DG_CAPTURE_E_PRIVACY_UNSUPPORTED` | `CapturePrivacyUnsupported` | 否 | 平台不能满足本次完整屏蔽要求 |
| `DG_CAPTURE_E_INTERNAL` | `CaptureNative` | 否 | 已截获的未知 native 故障 |

`out_error` 只携带数值型 native domain/code，用于本地诊断。它不包含动态字符串，从而没有
跨 runtime 分配、release、路径泄漏或错误字符串分支。

### 5.3 请求字段

| 字段 | 规则 |
|---|---|
| `struct_size` | 调用方设置为 `sizeof(dg_capture_request_v1)` |
| `flags` | v1 只允许 `DG_CAPTURE_SHOWS_CURSOR`；未知 bit 拒绝 |
| `image_format` | v1 只允许 `DG_CAPTURE_IMAGE_JPEG` |
| `target_height` | `1..16384`；产品层当前只下发已允许的高度 |
| `jpeg_quality` | `1..100` |
| `timeout_ms` | `1..60000` |
| `blocked_application_id_count` | `0..4096` |
| `reserved0` | 必须为 0 |
| `output_path` | 非空、合法 UTF-8、绝对路径、最大 32768 bytes |
| `blocked_application_ids` | count 为 0 时可为 NULL；否则指向 count 个借用 string view |

屏蔽 ID 每项非空、合法 UTF-8、最大 4096 bytes，全部字符串总长不得超过 1 MiB。输入字符串
不要求 NUL 结尾。

### 5.4 结果与错误字段

`out_result` 必填；调用方先清零并设置 `struct_size`。只有 `DG_CAPTURE_OK` 时以下字段有效：

- `image_format`：实际文件格式，v1 为 JPEG；
- `captured_at_unix_ns`：Unix epoch 纳秒；
- `file_size`：最终文件字节数；
- `width`、`height`：实际编码像素尺寸。

`out_error` 可为 NULL；非 NULL 时调用方先清零并设置 `struct_size`。失败时：

- `native_domain` 是封闭枚举：none、POSIX errno、Apple NSError code 或 Windows HRESULT；
- `native_code` 是平台数值，只用于诊断；
- 成功和 blocked 时两者必须为 0。

返回非 `DG_CAPTURE_OK` 时，除调用方提供的 `struct_size` 外，`out_result` 保持为 0。

### 5.5 ABI 规则

1. 只支持 64 位目标；头文件用 `_Static_assert`/`static_assert` 固定布局；
2. Windows 导出显式使用 `__cdecl`，其它平台使用默认 C calling convention；
3. major 不同立即失败；同 major 的新增字段只能追加到 struct 尾部并提升 minor；
4. enum 常量不直接作为 struct 字段类型；跨 ABI 字段固定为 `uint32_t`、`int32_t`、`int64_t`；
5. 所有输入指针只借用到函数返回；native 不得保留 Go 指针；
6. 所有 Swift error、Objective-C exception、C++ exception 和 HRESULT 必须在 C 边界内截获；
7. 不从 native 回调 Go，不返回 native 对象、像素 buffer 或所有权不明的字符串；
8. `dg_capture_once` 可从非 UI 线程调用并允许阻塞；Go 不得在 Wails 主线程直接调用；
9. 不同调用互相独立。实现可以为平台 API 串行化短暂临界区，但不得保留 recorder 配置、状态或
   上一次请求作为行为依据；
10. 不根据路径后缀猜格式；`image_format` 决定编码，Go 必须给路径使用一致后缀；
11. 路径和屏蔽 ID 不得出现在 native 日志或错误输出。

## 6. macOS 实现

### 6.1 系统 API

基线为 macOS 14 的 `SCScreenshotManager.captureImage`：

```swift
let content = try await SCShareableContent.excludingDesktopWindows(
    false,
    onScreenWindowsOnly: true
)
guard let display = content.displays.first(where: {
    $0.displayID == CGMainDisplayID()
}) else {
    throw CaptureError.noDisplay
}
let excluded = resolveBlockedApplications(
    content.applications,
    request.blockedApplicationIDs
)
let filter = SCContentFilter(
    display: display,
    excludingApplications: excluded,
    exceptingWindows: []
)
let configuration = SCStreamConfiguration()
configuration.height = request.targetHeight
configuration.width = scaledWidth(display, height: request.targetHeight)
configuration.scalesToFit = true
configuration.showsCursor = request.showsCursor

let image = try await SCScreenshotManager.captureImage(
    contentFilter: filter,
    configuration: configuration
)
```

`captureImage` 返回 `CGImage`，随后用 ImageIO 在 Swift 内编码 JPEG 并写临时文件。像素不跨 ABI。
不用 `captureSampleBuffer`，因为本层不再拥有分段编码器；不用 `SCStream`，因为它是持续捕获会话，
会引入本设计明确排除的 start/stop/首帧状态。

v1 固定捕获系统主显示器。macOS 上它是 `CGMainDisplayID()` 指向的显示器，通常是承载菜单栏的
显示器，不随鼠标位置改变。用户在系统设置中更改主显示器后，下一次调用重新解析，因此不保留
显示器 ID 或跨调用缓存。

### 6.2 隐私执行顺序

同一次调用必须按以下顺序：

1. 获取当前 `SCShareableContent`；
2. 解析屏蔽 bundle ID；
3. 紧邻截图前检查当前前台应用；
4. 前台命中则返回 `DG_CAPTURE_BLOCKED`，不调用 `captureImage`；
5. 未命中则用 `excludingApplications` 构造 filter；
6. 截取一次并立即编码落盘；
7. 原始 `CGImage` 离开作用域后释放。

Go 的前台应用观察可用于 UI 和调度，但不能替代 native 的调用时检查。否则应用在 Go 检查与真正
截图之间切换时会产生隐私竞态。

### 6.3 权限和身份

权限查询和申请继续属于 `platform.System`，不进入本头文件。`dg_capture_once` 仍必须处理权限在
运行期被撤销，并返回 `DG_CAPTURE_E_PERMISSION_DENIED`。

静态库链接进 Wails 主可执行文件，沿用 `io.github.jwz-git.Daygo` 的应用身份，不产生第二个需要
授权的 helper。开发签名、Release 签名和升级签名必须分别做 TCC 实验。

### 6.4 目录建议

```text
native/
├── include/daygo_capture.h
└── darwin/
    └── Sources/
        ├── CaptureABI.swift       C struct 校验、错误映射、同步入口
        ├── Screenshot.swift       单次 ScreenCaptureKit 调用
        ├── Privacy.swift          本次调用的前台兜底与应用排除
        └── JPEGWriter.swift       临时文件与原子发布

internal/platform/darwin/
├── capture.go                     Go 值与 ABI 值转换
├── bridge_darwin.go               唯一 import C 的文件
└── unavailable_darwin.go          darwin && !cgo
```

不存在 `CaptureEngine`、timer、event queue、ack journal 或 native recorder actor。

## 7. Windows 约束

Windows 仍只做候选实验：

- WGC：为当前 primary monitor 创建一次 capture，取第一帧，立即关闭 session/frame pool；
- DXGI：在一个调用内创建/获取/释放一次 duplication 帧；
- GDI `BitBlt`：只作完整性和延迟基线，不作为默认；
- 所有 D3D/COM 对象在函数返回前释放，或只进入无产品语义的短期进程级设备缓存；
- DLL 只导出本头文件中的两个符号；不导出 STL、COM、WinRT、HRESULT 或 D3D handle；
- DLL 搜索必须使用绝对路径和安全搜索 flag，不能从工作目录隐式加载。

Windows 没有与 ScreenCaptureKit `excludingApplications` 等价的公开能力。只要请求包含屏蔽 ID，
实现不能完整保证画面排除时就返回 `DG_CAPTURE_E_PRIVACY_UNSUPPORTED`，不得静默降低为只检查
前台应用。该问题验证前 Windows 捕获不能进入发布构建。

## 8. 验证门禁

### 8.1 C ABI

- C11 与 C++20 都能 include `daygo_capture.h`；
- `_Static_assert`/`static_assert` 的 size 与 offset 全部通过；
- Swift/C++ 实现与 Go bridge 都 include 同一头文件；
- ABI major 不匹配、过小 struct、未知 flags、非法 UTF-8、超长数组全部明确失败；
- 输入数组和字符串在返回后不被访问；
- 生产导出表只有 `dg_capture_abi_version` 和 `dg_capture_once`。

### 8.2 单次调用契约

同一套契约测试运行 fake 与真实适配层：

1. 成功只生成一个 JPEG，路径、尺寸、大小与结果一致；
2. 最终路径已存在时不覆盖；
3. 编码或 rename 失败后没有最终文件和残留 partial；
4. 前台屏蔽时返回 blocked，未调用平台截图，未生成文件；
5. 非前台屏蔽应用的可见窗口不出现在图像中；
6. 权限拒绝、主显示器不可用和超时映射为固定错误；
7. context 在调用前取消时不调用 native；
8. recorder generation 在调用期间变化时，Go 不提交结果并删除文件；
9. 连续 10,000 次调用无无界内存、线程、文件描述符和系统对象增长；
10. 多显示器环境始终只截主显示器；Retina、旋转、SDR/HDR 下尺寸和色彩符合预期。

### 8.3 真实 macOS

| 场景 | 必须观察的结果 |
|---|---|
| 每 1/10/60 秒调用 | 每次最多一张；没有持续 capture session 或常亮录屏指示 |
| 双屏，移动鼠标到副屏 | 仍只截系统主显示器；不产生选择或切换状态 |
| 前台屏蔽 | blocked、无真实截图文件 |
| 非前台屏蔽窗口可见 | 图片中无其内容 |
| sleep/wake、lock/unlock | Go 停止/恢复调用；native 无残留会话 |
| 调用中暂停或退出 | 在途调用收尾，过期文件不入库 |
| 权限运行期撤销 | 固定 permission error，不循环申请 |
| 24 小时 | 无资源增长；失败可按调用计数，无 native 队列积压 |
| 开发/Release/升级签名 | TCC 身份稳定且符合预期 |

这些实验只证明截屏原语。关窗后进程存活、状态栏重开和 recorder 继续调度仍由 G-host 单独验证。

## 9. 迁移和退出条件

实现前需要一个独立的 Go 契约迁移，把现有 `internal/platform` 和公共接口规范从
`Start/Stop/Events/Ack` 一次性切换为本文的 `Capture` 单次调用。本文不授权保留旧接口适配层。

以下条件全部满足后，才可把 macOS 截屏从“待定设计”改为“已决定”：

1. 真实机器证明 `SCScreenshotManager.captureImage` 每次调用只产生离散截图；
2. 隐私前台兜底和 `excludingApplications` 双层测试通过；
3. 原子写入、失败清理和 Go pending 恢复测试通过；
4. 24 小时调用无资源增长；
5. 开发、Release 和升级签名的权限身份通过；
6. `CGO_ENABLED=0 go build ./...` 与 Linux `go test ./internal/...` 不受影响。

回退方式：禁用真实 capture capability，保留 Go recorder、fake 和权限 UI；不得回退到持续
`SCStream`、旧 CoreGraphics API 或弱化隐私规则的实现。

## 10. 资料来源

- [Apple WWDC23: What’s new in ScreenCaptureKit](https://developer.apple.com/videos/play/wwdc2023/10136/)
- [Apple: SCScreenshotManager](https://developer.apple.com/documentation/screencapturekit/scscreenshotmanager)
- [Apple: SCContentFilter](https://developer.apple.com/documentation/screencapturekit/sccontentfilter)
- [Apple: CGPreflightScreenCaptureAccess](https://developer.apple.com/documentation/coregraphics/cgpreflightscreencaptureaccess%28%29)
- [Swift Evolution SE-0495](https://github.com/swiftlang/swift-evolution/blob/main/proposals/0495-cdecl.md)
- [Microsoft: Screen capture](https://learn.microsoft.com/en-us/windows/apps/develop/media-authoring-processing/screen-capture)
- [Microsoft: Desktop Duplication API](https://learn.microsoft.com/en-us/windows/win32/direct3ddxgi/desktop-dup-api)
- [Microsoft: SetWindowDisplayAffinity](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setwindowdisplayaffinity)
