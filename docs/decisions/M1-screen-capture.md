# M1 屏幕捕获：技术调研、数据契约与原生 ABI

> **状态：M1 实验规格。** 本文冻结上层捕获数据契约和 ABI v1 的边界，给出 macOS 的首选实现与
> Windows 的候选实现。它不等于完成 [M1 原生适配层总决策](../06-native-integration.md#66-选型时要回答的问题)：
> 真实授权、捕获指示、隐私遮蔽、签名、公证和长时间运行实验通过前，`09` 中的平台实现仍是
>「待定设计」。
>
> **当前代码事实。** `internal/platform` 端口、值类型与封闭集校验已落盘（接口，无实现）；
> `internal/app` 的 M1 绑定骨架已挂入 Wails `Bind`（权限方法在真实适配层落地前返回
> `native_unavailable`）。`internal/platform/fake`、`platformtest.Suite`、真实捕获、
> 分段编码和本决策描述的原生目录均尚未实现。以下路径与命令是实施规格，不是现状。

## 1. 范围与结论

本文只决定以下内容：

1. Go 上层如何接收已持久化帧和分段收尾事件；
2. Go 与进程内原生模块之间的 C ABI；
3. macOS 和 Windows 各自优先实验的捕获 API、实现语言和构建方式；
4. 哪些实验通过后才能把候选方案标成已决定。

不在本文决定：分段容器与编码格式、帧解码、timelapse、状态栏、钥匙串、自动更新，以及
 Daygo 是否正式发布 Windows 版本。

结论：

| 项 | 决定 / 建议 | 原因 |
|---|---|---|
| 上层 Go 契约 | 有序 `CaptureEvent` + 累计 `Ack`；像素不进入 Go | 保持 SQLite 单写入方，同时避免 1080p BGRA 在 Go 与原生编码器之间往返复制 |
| ABI 形态 | C ABI v1，固定宽度整数、长度显式的 UTF-8、opaque handle、轮询而非回调 | Swift、MSVC C++ 与 cgo 都能稳定消费；没有 Go 指针留在原生侧 |
| macOS | Swift 6.3 + ScreenCaptureKit `SCScreenshotManager.captureSampleBuffer`，静态库进程内链接 | 真正的一次性截图；支持按应用排除；`CMSampleBuffer` 可直接进入后续原生编码器 |
| macOS 导出 | 手写 C 头 + Swift `@c @implementation` | Swift 6.3 已正式实现 SE-0495；不用未公开的 `@_cdecl` |
| Windows 语言 | C++20 + C++/WinRT + D3D11，MSVC 构建 DLL | Windows SDK 原生投影；无 GC；最接近捕获帧和 GPU 编码面 |
| Windows API | 首测 Windows.Graphics.Capture；并行对照 DXGI Desktop Duplication；GDI `BitBlt` 只作基线 | WGC 是微软当前截图指导；DXGI 可验证无常亮边框方案；GDI 完整性和性能不足 |
| Windows 发布状态 | **阻塞**，直到隐私双层保护和捕获指示实验通过 | Windows 没有让捕获方任意排除第三方应用窗口的等价公开 API |
| 原生产物 | macOS 通用静态 `.a`；Windows 每架构一个 `.dll` | macOS 避免嵌套签名和 `@rpath`；Windows 隔离 MSVC C++ 与 cgo 的工具链 ABI |

## 2. 继承的硬约束

本设计不得弱化现有规范：

- 捕获是每个间隔一次的**离散截图**，不是连续录屏流；
- 每次只捕获光标所在的活跃显示器，并保留现有滞后切换规则；
- 屏蔽应用必须同时有「从画面排除」和「前台命中时生成脱敏占位帧」两层保护；
- Go 是 SQLite 唯一写入方；原生模块不打开数据库；
- 配置由 Go 全量下发；原生模块不自行读取产品设置；
- 未确认事件必须落盘并可在重启后重放；
- 分段必须在睡眠、锁定、屏保、更新、退出和关机路径收尾或安全移交；
- Go Core 仍须通过 `CGO_ENABLED=0 go build ./...` 和 Linux 无头测试；
- 原生错误、日志和诊断不得包含屏幕内容、窗口标题、文件路径或 API key；
- 捕获二进制的代码身份在发布后必须稳定。

## 3. 技术调研

### 3.1 macOS 候选

#### A. ScreenCaptureKit `SCScreenshotManager`（首选）

`SCScreenshotManager` 从 macOS 14.0 提供一次性截图 API。Apple 明确把它描述为 one-off
 screenshot，而不是 `SCStream`。`SCContentFilter(display:excludingApplications:exceptingWindows:)`
 可以按 `SCRunningApplication.bundleIdentifier` 排除应用，并保留桌面与 Dock。

生产路径使用 `captureSampleBuffer`，输出 SDR BGRA `CMSampleBuffer`：

```swift
let content = try await SCShareableContent.excludingDesktopWindows(
    false,
    onScreenWindowsOnly: true
)
let display = try resolveDisplay(content.displays, requestedID)
let blocked = content.applications.filter {
    blockedApplicationIDs.contains($0.bundleIdentifier)
}
let filter = SCContentFilter(
    display: display,
    excludingApplications: blocked,
    exceptingWindows: []
)
let configuration = SCStreamConfiguration()
configuration.width = outputWidth
configuration.height = outputHeight
configuration.pixelFormat = kCVPixelFormatType_32BGRA
configuration.showsCursor = showsCursor
configuration.preservesAspectRatio = true

let sample = try await SCScreenshotManager.captureSampleBuffer(
    contentFilter: filter,
    configuration: configuration
)
```

选择 sample buffer 而不是 `CGImage`，因为后续分段编码器可直接消费 `CVPixelBuffer`，避免
 `CGImage → CPU bytes → Go []byte → native encoder` 的重复复制。ABI 不暴露
 `CMSampleBufferRef`；它只在原生模块内部由 opaque frame 生命周期管理。

部署下限因此是 **macOS 14.0**。macOS 26 的新
 `captureScreenshot(..., SCScreenshotConfiguration)` 支持 JPEG/PNG/HEIC、HDR 和直接写文件，
但不能作为 v1 基线，否则部署下限会被抬到 macOS 26。

#### B. ScreenCaptureKit `SCStream` 取首帧（拒绝作为默认）

可在 macOS 12.3–13 上启动 `SCStream`、收到首帧后立即停止，但它属于屏幕共享流。系统会把
活跃 stream 纳入屏幕共享体验；反复启停也引入更多状态、首帧和停止竞态。它与「一次调用只取
一张离散截图」的产品约束不如 `SCScreenshotManager` 一致。

只在产品明确要求支持 macOS 12/13，且真实机器证明不会造成持续指示或明显闪烁时，才重新评估。

#### C. CoreGraphics 旧截图 API（拒绝）

`CGDisplayCreateImage`、`CGWindowListCreateImage` 及其变体在 macOS 14.x 标记 obsolete，并在
 macOS 15.0 不可用。新项目不能以已淘汰接口作为主路径。

#### D. `/usr/sbin/screencapture` 子进程（拒绝）

它会增加进程启动、临时文件、取消、路径校验和授权身份问题，也没有满足屏蔽应用第一层保护的
结构化过滤器。它只适合人工诊断，不进入适配层。

### 3.2 macOS 权限与身份

- 用 `CGPreflightScreenCaptureAccess()` 无副作用地检查授权；
- 只有显式用户操作才调用 `CGRequestScreenCaptureAccess()`；拒绝后引导系统设置，不循环弹窗；
- 实际截图仍必须处理权限在运行期被撤销；
- 静态库链接进 Wails 主可执行文件，捕获代码沿用 `io.github.jwz-git.Daygo` 的应用身份，不产生
  第二个需要单独授权的 helper；
- 开发签名、Release 签名和升级签名必须分别完成 TCC 回归实验，不能从源码 API 推断身份稳定。

### 3.3 Windows 捕获 API 候选

#### A. Windows.Graphics.Capture（第一候选，未通过门禁）

微软当前文档明确用 `Windows.Graphics.Capture` 获取显示器或窗口帧，并给出保存单张截图的
流程。Win32 桌面进程可通过
 `IGraphicsCaptureItemInterop::CreateForMonitor(HMONITOR, ...)` 创建显示器捕获项；该 interop
 从 Windows 10 1903（build 18362）可用。

实现使用：

- C++/WinRT；
- D3D11 device；
- `Direct3D11CaptureFramePool::CreateFreeThreaded`；
- `B8G8R8A8UIntNormalized` 的一帧池；
- `StartCapture()` 后只取第一帧，立即关闭 session 和 frame pool；
- 使用 `ContentSize` 裁掉 surface 中未定义区域；
- 用 `SystemRelativeTime` 记录 QPC 时间，再映射到墙上时间；
- HDR 显示器实验时同时测试 `R16G16B16A16_FLOAT` 和 SDR tone mapping。

风险：WGC 默认绘制捕获边框。`IsBorderRequired = false` 需要请求 borderless access，并不等价于
排除应用。必须在打包与非打包、Windows 10/11、单次 session 每 10 秒启停的组合上观察是否
闪烁、常亮或重复授权。

#### B. DXGI Desktop Duplication（第二候选）

`IDXGIOutput1::DuplicateOutput` + `AcquireNextFrame` 返回每显示器的 BGRA D3D11 surface。
它没有 WGC 的标准捕获边框，适合验证用户观感；但必须自行处理：

- `DXGI_ERROR_WAIT_TIMEOUT` 和 `DXGI_ERROR_ACCESS_LOST`；
- 每次成功后严格 `ReleaseFrame`；
- 显示器旋转；
- 光标形状与合成；
- 每个输出单独 duplication；
- 锁屏、桌面切换、全屏应用和显示模式变化后的重建。

实验只在离散采样点获取一帧，不持续保存中间帧。即便 API 名称是 duplication，也不得把它实现
成连续录屏管线。

#### C. GDI `BitBlt`（仅对照和最后 fallback）

`BitBlt(SRCCOPY | CAPTUREBLT)` 是真正的同步单帧调用，适合做延迟和资源占用基线；但它不提供
时间戳、光标元数据或 GPU surface，对硬件合成、全屏 DirectX、受保护内容和不同显示设备的
覆盖不可靠，并会增加 GPU 到系统内存的复制。不能仅因实现短就选它做默认。

### 3.4 Windows 隐私阻塞项

Windows 公开 API 没有 ScreenCaptureKit
 `excludingApplications` 的等价能力。`SetWindowDisplayAffinity(WDA_EXCLUDEFROMCAPTURE)` 只能
由窗口所属进程设置，Daygo 不能替第三方应用设置；`IsBorderRequired` 只控制边框。

必须分别验证两层：

1. **前台兜底层可实现。** `GetForegroundWindow → GetWindowThreadProcessId → 进程身份` 命中屏蔽
   ID 时不启动捕获，直接生成占位帧；
2. **画面排除层未证明。** 候选方案是在 GPU 内按所有屏蔽进程的可见窗口区域立即遮蔽，再编码，
   且原始 surface 不落盘、不传 Go、不进入日志。透明窗口、圆角、阴影、DPI、跨屏、owned/child
   window、虚拟桌面和窗口 z-order 都可能使遮蔽不完整。

在 IT-5 的 Windows 扩展用例证明所有这些边界前，Windows 适配层只能是实验代码，不能宣称与
 macOS 隐私契约等价，也不能进入发布构建。若实验失败，应停止 Windows 捕获，而不是静默把
第一层保护降级成「尽量遮挡」。

### 3.5 Windows 实现语言比较

| 语言 / 框架 | 优点 | 代价 | 结论 |
|---|---|---|---|
| C++20 + C++/WinRT | Windows SDK 官方投影；直接持有 D3D11/COM；无 GC；可导出普通 C ABI | COM apartment、设备丢失和资源生命周期需显式处理 | **首选** |
| Rust + `windows-rs` | Microsoft 维护的类型绑定；内存安全；`cdylib` C ABI 成熟 | 新增 Cargo/rustup/target 工具链；D3D 仍有 unsafe；团队维护面扩大 | 若 C++ 原型出现内存安全问题再评估 |
| C# + NativeAOT | WinRT 开发速度快；`UnmanagedCallersOnly` 可导出 C 入口 | AOT/裁剪、GC 对象不能跨 ABI、产物与调试链更复杂 | 不选 |
| C / WRL | ABI 最直接 | 手写 COM 冗长；微软推荐 C++/WinRT 取代 WRL | 不选 |

## 4. 上层 Go 捕获数据接口

本节是 [05 §5.7](../05-interface-contract.md#57-b4platform-端口契约) 中 `Capture` 子契约的完整形状。
真实 macOS、未来 Windows 和 `platform/fake` 必须跑同一套 `platformtest.Suite`。

```go
package platform

import (
    "context"
    "time"
)

type Capture interface {
    // Start 接受完整快照配置。相同配置重复调用是空操作；不同配置必须 Stop 后再 Start。
    // ctx 只限定启动命令，不拥有捕获生命周期。
    Start(ctx context.Context, cfg CaptureConfig) error

    // Stop 停止定时器并收尾当前分段。返回前，收尾事件必须已进入 durable event 队列。
    Stop(ctx context.Context) error

    // Ack 在 Go 提交对应 screenshots/segment 更新之后调用。
    // ack 是累计确认：确认 seq 也确认此前所有连续事件。
    Ack(ctx context.Context, seq uint64) error

    // Events 是单一、有序、持久化的 frame/segment_closed 事件流。
    Events() <-chan CaptureEvent

    // Status 是可合并的瞬时状态流，不参与 Ack。
    Status() <-chan CaptureStatus

    // Close 只能调用一次；它停止捕获、释放原生资源，然后关闭两个 channel。
    Close(ctx context.Context) error
}

type CaptureConfig struct {
    Interval              time.Duration
    CaptureHeight         int
    BlockedApplicationIDs []string
    SegmentDirectory      string
    PreferredDisplayID    *string
    ShowsCursor           bool
    SegmentMaxFrames      int
    SegmentMaxDuration    time.Duration
}

type CaptureEventKind string

const (
    CaptureEventFrame         CaptureEventKind = "frame"
    CaptureEventSegmentClosed CaptureEventKind = "segment_closed"
)

type CaptureEvent struct {
    Seq     uint64
    Kind    CaptureEventKind
    Frame   *CapturedFrame
    Segment *SegmentClosed
}

type CapturedFrame struct {
    SegmentPath string    // 相对 CaptureConfig.SegmentDirectory；Go 再做目录内校验
    FrameIndex  int       // 从 0 开始
    CapturedAt  time.Time // 含时区的时间点；入库时转 Unix 秒
    IdleSeconds *int      // nil 表示不可用，0 表示可用且刚有输入
    DisplayID   string    // 会话内稳定的 opaque ID，不解析平台格式
    Width       int
    Height      int
    Redacted    bool
}

type SegmentClosed struct {
    SegmentPath string
    TotalBytes  int64
    FrameCount  int
    Succeeded   bool
}

type CapturePhase string

const (
    CaptureIdle      CapturePhase = "idle"
    CaptureStarting  CapturePhase = "starting"
    CaptureCapturing CapturePhase = "capturing"
    CapturePaused    CapturePhase = "paused"
)

type PermissionState string

const (
    PermissionGranted       PermissionState = "granted"
    PermissionDenied        PermissionState = "denied"
    PermissionNotDetermined PermissionState = "not_determined"
)

type CaptureStatus struct {
    Phase           CapturePhase
    Permission      PermissionState
    ActiveDisplayID *string
    LastFrameAt     *time.Time
    Fault           *CaptureFault
}

type CaptureFault struct {
    Code      string // 封闭的 platform 错误分类，不是 Wails 错误码
    Retryable bool
    Message   string // 已脱敏
}
```

### 4.1 事件不变量

1. `CaptureEvent.Seq` 在一个安装的数据目录内跨进程重启单调递增；不得在 `Start` 时归零；
2. `Kind == frame` 时只有 `Frame` 非 nil；`Kind == segment_closed` 时只有 `Segment` 非 nil；
3. frame 事件只在像素已追加并落到原生缓冲/分段后发出；Go 提交数据库行后才 `Ack`；
4. segment-closed 事件排在该分段所有 frame 之后；因此单一事件流不能拆成两个无序 channel；
5. 未确认事件及其必要元数据由适配层持久化，重启后原序重放；
6. Go 的重复安全键仍是 `(segment_path, frame_index)`；重复事件不得生成重复行；
7. `Stop` 不关闭 channel，后续允许再次 `Start`；只有 `Close` 关闭 channel；
8. `Events` 满时不得丢 durable 事件；原生侧继续落盘并等待消费。`Status` 满时按类型合并旧状态；
9. `SegmentClosed.Succeeded == false` 要求 Go 丢弃文件并软删除该分段对应行；
10. `CapturedAt` 表示原生帧采集时间。无 OS 采集时间时用请求开始与完成墙钟的中点，不得使用
    Go 收到事件的时间。

### 4.2 ID 与配置

- 应用 ID 对 Go 是 opaque UTF-8 字符串。macOS 当前为 bundle identifier；Windows 实验中，
  packaged app 优先用 package identity，传统 Win32 应用使用规范化可执行文件身份的不可逆摘要；
- 显示器 ID 对 Go 同样 opaque，只要求在一次登录会话和显示器配置不变期间稳定；
- `SegmentDirectory` 由 Go 创建并传绝对路径。ABI 只返回相对路径；Go 解析后再次确认仍在目录内；
- 生产配置沿用 `04 §4.1.1`：高度 720/1080，宽度等比并向上取偶数，分段最多 600 帧或
  600 秒；fake 可传更小上限以做快速轮换测试；
- 前台应用命中屏蔽 ID 时，适配层不调用系统截图 API，生成相同尺寸的脱敏占位帧并设
  `Redacted = true`。

## 5. C ABI v1

两个平台共享一份手写头文件 `native/include/daygo_capture.h`。头文件是 ABI 唯一来源；Swift
通过 `@c @implementation` 实现其中声明，C++ 直接 include 并导出相同符号。

```c
#ifndef DAYGO_CAPTURE_H
#define DAYGO_CAPTURE_H

#include <stdint.h>

#if defined(_WIN32)
#  if defined(DAYGO_CAPTURE_BUILD)
#    define DG_API __declspec(dllexport)
#  else
#    define DG_API __declspec(dllimport)
#  endif
#else
#  define DG_API __attribute__((visibility("default")))
#endif

#define DG_CAPTURE_ABI_MAJOR 1u
#define DG_CAPTURE_ABI_MINOR 0u

typedef struct dg_capture dg_capture;

typedef struct dg_string_view_v1 {
    const uint8_t *data;
    uint64_t len;
} dg_string_view_v1;

typedef struct dg_owned_string_v1 {
    uint8_t *data;
    uint64_t len;
} dg_owned_string_v1;

enum {
    DG_OK = 0,
    DG_TIMEOUT = 1,
    DG_E_INVALID_ARGUMENT = -1,
    DG_E_ABI_MISMATCH = -2,
    DG_E_UNSUPPORTED = -3,
    DG_E_PERMISSION_DENIED = -4,
    DG_E_UNAVAILABLE = -5,
    DG_E_CANCELLED = -6,
    DG_E_INTERNAL = -7
};

enum {
    DG_CAPTURE_SHOWS_CURSOR = 1u << 0
};

enum {
    DG_EVENT_FRAME = 1,
    DG_EVENT_SEGMENT_CLOSED = 2,
    DG_EVENT_STATUS = 3
};

enum {
    DG_PHASE_IDLE = 1,
    DG_PHASE_STARTING = 2,
    DG_PHASE_CAPTURING = 3,
    DG_PHASE_PAUSED = 4
};

enum {
    DG_PERMISSION_GRANTED = 1,
    DG_PERMISSION_DENIED = 2,
    DG_PERMISSION_NOT_DETERMINED = 3
};

typedef struct dg_error_v1 {
    uint32_t struct_size;
    int32_t native_code;
    dg_owned_string_v1 message;
} dg_error_v1;

typedef struct dg_capture_config_v1 {
    uint32_t struct_size;
    uint32_t flags;
    uint64_t interval_ns;
    uint32_t capture_height;
    uint32_t segment_max_frames;
    uint64_t segment_max_duration_ns;
    dg_string_view_v1 segment_directory;
    dg_string_view_v1 preferred_display_id;
    const dg_string_view_v1 *blocked_application_ids;
    uint32_t blocked_application_id_count;
    uint32_t reserved0;
} dg_capture_config_v1;

typedef struct dg_capture_event_v1 {
    uint32_t struct_size;
    uint32_t kind;
    uint64_t seq; /* durable events only; status uses 0 */

    int64_t captured_at_unix_ns;
    int64_t idle_seconds;
    uint32_t idle_seconds_valid;
    uint32_t width;
    uint32_t height;
    uint32_t frame_index;
    uint32_t redacted;

    uint64_t total_bytes;
    uint32_t frame_count;
    uint32_t segment_succeeded;

    uint32_t phase;
    uint32_t permission;
    int32_t native_code;
    uint32_t retryable;

    dg_owned_string_v1 display_id;
    dg_owned_string_v1 segment_path;
    dg_owned_string_v1 message;
} dg_capture_event_v1;

DG_API void dg_capture_abi_version(uint32_t *major, uint32_t *minor);

DG_API int32_t dg_capture_create(
    uint32_t requested_major,
    dg_capture **out_capture,
    dg_error_v1 *out_error
);

DG_API int32_t dg_capture_start(
    dg_capture *capture,
    const dg_capture_config_v1 *config,
    dg_error_v1 *out_error
);

DG_API int32_t dg_capture_stop(
    dg_capture *capture,
    dg_error_v1 *out_error
);

DG_API int32_t dg_capture_poll_event(
    dg_capture *capture,
    uint32_t timeout_ms,
    dg_capture_event_v1 *out_event,
    dg_error_v1 *out_error
);

DG_API int32_t dg_capture_ack(
    dg_capture *capture,
    uint64_t seq,
    dg_error_v1 *out_error
);

DG_API int32_t dg_capture_destroy(
    dg_capture *capture,
    dg_error_v1 *out_error
);

DG_API void dg_capture_event_release(dg_capture_event_v1 *event);
DG_API void dg_capture_error_release(dg_error_v1 *error);

#endif
```

### 5.1 ABI 规则

1. 所有输入 struct 先清零并设置 `struct_size`；接收方只读取 `struct_size` 覆盖的已知字段；
2. major 不同立即 `DG_E_ABI_MISMATCH`；同 major 下新增字段只追加到 struct 尾部并升 minor；
3. enum 在 ABI 上一律是显式 `uint32_t`/`int32_t` 字段，不把 C enum 大小当作契约；
4. 输入 string view 只在调用期间借用；原生实现不得保存 Go 指针；空值是 `{NULL, 0}`；
5. 所有输出字符串由原生库分配，Go 复制后必须调用对应 `release`；不得用 `free()` 跨运行时释放；
6. 字符串是 UTF-8 且不要求 NUL 结尾；路径也使用 UTF-8，不暴露 `NSString`、`HSTRING` 或
   UTF-16 指针；
7. `poll_event` 最长只阻塞 `timeout_ms`；Go 用不超过 100 ms 的轮询间隔并在每次之间检查
   `ctx.Done()`；`stop` 必须唤醒正在等待的 poll；
8. 不从 Swift/C++ 回调 Go。这样避免 Go callback、线程附着、`runtime/cgo.Handle` 和取消竞态；
9. 同一个 handle 的方法是线程安全的；实现内部串行化状态转换。不能依赖调用落在同一个 OS 线程；
10. Swift error、Objective-C exception、C++ exception、HRESULT 和 panic 都必须在 ABI 内截获并映射；
    任何异常不得越过 C 边界；
11. `message` 必须脱敏且不用于程序判断；Go 按结果码和事件 kind 判断；
12. `destroy` 之后 handle 立即失效；重复 destroy、悬空 event 和 release 后再访问都属于调用方错误。

### 5.2 为什么不让像素跨 ABI

一张 1920×1080 BGRA 帧约 7.9 MiB；4K 帧约 31.6 MiB。若 Swift/C++ 返回 bytes 给 Go，再由
 Go 交回原生编码器，每 10 秒会产生两次没有业务价值的大块复制，还迫使不同平台统一 CPU
像素布局。ABI 因此只传已持久化帧的描述信息；原始 `CMSampleBuffer`/`ID3D11Texture2D` 留在
原生模块内直接进入后续分段编码器。

为了单元测试原生色彩和遮蔽，可在 **test-only** 构建增加复制 BGRA 的符号；该符号不进入
生产头文件，不得被上层依赖。

## 6. macOS ABI 适配与构建

### 6.1 目录与边界

```text
native/
├── include/daygo_capture.h
└── darwin/
    ├── Sources/
    │   ├── CaptureABI.swift       @c @implementation，只做 ABI 转换
    │   ├── CaptureEngine.swift    serial actor/queue、状态与轮询队列
    │   ├── Screenshot.swift       ScreenCaptureKit 一次性截图
    │   └── Privacy.swift          bundle ID 过滤与占位帧
    └── build.sh

internal/platform/darwin/
├── capture.go                     纯 Go 端口适配、channel、错误映射
├── bridge_darwin.go               //go:build darwin && cgo
└── unavailable_darwin.go          //go:build darwin && !cgo
```

只有 `bridge_darwin.go` import `C`。业务 service 不 import `darwin`，只依赖消费者侧的
 `platform.Capture` 接口。

### 6.2 Swift 导出

Swift 6.3 使用 SE-0495 的正式语法：

```swift
@c @implementation
func dg_capture_abi_version(
    _ major: UnsafeMutablePointer<UInt32>?,
    _ minor: UnsafeMutablePointer<UInt32>?
) {
    major?.pointee = 1
    minor?.pointee = 0
}
```

编译时通过 bridging header 导入手写 C 声明，编译器会检查 Swift 实现与 C 原型一致。
不要使用 `@_cdecl`；它是历史实验属性，且与正式 `@c` 的符号模型不同。

### 6.3 可复现构建命令

构建工具链固定为 **Xcode 26 / Swift 6.3 或更新的同 major 工具链**。部署下限固定为 macOS 14.0。
每个架构先产出静态 archive，再用 `lipo` 合并：

```bash
ROOT="$(pwd)"
OUT="$ROOT/build/native/darwin"
HEADER="$ROOT/native/include/daygo_capture.h"
SOURCES=("$ROOT"/native/darwin/Sources/*.swift)
mkdir -p "$OUT/arm64" "$OUT/x86_64" "$OUT/universal"

for ARCH in arm64 x86_64; do
  xcrun swiftc \
    -swift-version 6 \
    -parse-as-library \
    -O -whole-module-optimization \
    -target "$ARCH-apple-macos14.0" \
    -module-name DaygoCapture \
    -import-objc-header "$HEADER" \
    -emit-library -static \
    -framework Foundation \
    -framework CoreGraphics \
    -framework CoreMedia \
    -framework CoreVideo \
    -framework ScreenCaptureKit \
    "${SOURCES[@]}" \
    -o "$OUT/$ARCH/libdaygo_capture.a"
done

xcrun lipo -create \
  "$OUT/arm64/libdaygo_capture.a" \
  "$OUT/x86_64/libdaygo_capture.a" \
  -output "$OUT/universal/libdaygo_capture.a"
xcrun lipo -info "$OUT/universal/libdaygo_capture.a"
```

cgo 链接面：

```go
/*
#cgo darwin CFLAGS: -I${SRCDIR}/../../../../native/include
#cgo darwin LDFLAGS: ${SRCDIR}/../../../../build/native/darwin/universal/libdaygo_capture.a
#cgo darwin LDFLAGS: -framework Foundation -framework CoreGraphics -framework CoreMedia
#cgo darwin LDFLAGS: -framework CoreVideo -framework ScreenCaptureKit
#include "daygo_capture.h"
*/
import "C"
```

选择静态库而不是 `.framework`/`.dylib`：最终 App 中没有第二个 Mach-O 原生组件需要设置
 `@rpath`、复制到 `Contents/Frameworks` 并嵌套签名；Swift runtime 使用系统随 macOS 提供的
库。若以后必须动态化，需重新打开 M-3 签名、公证和升级身份实验，不能只改链接参数。

### 6.4 开发与 Wails 构建接入

- `scripts/dev.sh` 在 `wails dev` 前调用 `native/darwin/build.sh`；
- `cmd/daygo/wails.json` 的 `preBuildHooks["darwin/*"]` 调同一脚本，不能维护第二套命令；
- CI 先运行 native build，再运行 Wails/Go build；
- 生成的 `.a`、module cache 和中间对象放 `build/native/`，不提交；
- Release 最终只签名 Daygo.app 主体及 Wails 自带组件；随后在干净机器验证 Gatekeeper 和 TCC。

完整 Xcode 仍是 Release、签名、公证和 UI 集成测试的前置条件。仅安装 Command Line Tools 可以
运行 `swiftc` 编译探针，但不能运行 `xcodebuild archive`；CI 镜像必须显式检查
 `xcode-select -p` 指向完整 Xcode。

## 7. Windows ABI 适配与构建

### 7.1 目录与边界

```text
native/
├── include/daygo_capture.h
└── windows/
    ├── CMakeLists.txt
    ├── daygo_capture.def          固定导出名
    └── src/
        ├── capture_abi.cpp        C ABI、异常截获
        ├── capture_engine.cpp     worker、队列、ack journal
        ├── wgc_capture.cpp        Windows.Graphics.Capture
        ├── dxgi_capture.cpp       Desktop Duplication 实验后备
        └── privacy.cpp            app identity、前台兜底、GPU 遮蔽实验

internal/platform/windows/
├── capture.go
├── bridge_windows.c              LoadLibraryExW/GetProcAddress；include 公共头
├── bridge_windows.go             //go:build windows && cgo
└── unavailable_windows.go        //go:build windows && !cgo
```

C++ DLL 绝不向外暴露 STL、WinRT、COM、HRESULT、exception、`HMONITOR` 或 D3D handle。只有
 `daygo_capture.h` 中的 C 符号可见。

### 7.2 为什么用 DLL + C loader shim

Go cgo 在 Windows 通常使用 MinGW/LLVM 的 C 工具链，而 C++/WinRT 原生模块用 MSVC。直接把
 MSVC C++ 静态库塞给 cgo 会把 C++ runtime、exception model 和链接格式耦合起来。

因此：

1. MSVC 独立构建 `daygo_capture.dll`；
2. 一个不含 C++ 的 `bridge_windows.c` 由 cgo 编译；
3. shim 用绝对路径和 `LoadLibraryExW(..., LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR |
   LOAD_LIBRARY_SEARCH_DEFAULT_DIRS)` 加载 DLL；
4. shim 用 `GetProcAddress` 解析 v1 全部符号，先校验 ABI major，再允许 `create`；
5. Go 仍通过 cgo 生成的 C struct 布局调用，不手写 `unsafe` 对齐，也不需要 GNU import library。

DLL 搜索失败时返回 `native_unavailable`，不得回退到工作目录搜索，避免 DLL hijacking。

### 7.3 C++/WinRT 初始化

- DLL 创建专属 MTA worker；worker 内 `winrt::init_apartment(winrt::apartment_type::multi_threaded)`；
- 使用 `Direct3D11CaptureFramePool::CreateFreeThreaded`，不依赖 Wails UI 线程的 DispatcherQueue；
- D3D device lost、capture item closed、显示器变化和锁屏均转换为状态事件；
- frame、surface 和 frame pool 按微软生命周期及时释放；不能在 frame 归还后保存 surface 引用；
- 所有 C++ exception 在 `capture_abi.cpp` catch，映射为固定结果码和脱敏 message。

### 7.4 CMake/MSVC 构建

构建机安装 Visual Studio 2022 Build Tools、Desktop development with C++、Windows 11 SDK。
 C++/WinRT 来自锁定版本的 Windows SDK，不引入 WinUI 或 Win2D；只使用 WinRT projection、D3D11
和系统库。

关键 CMake 约束：

```cmake
cmake_minimum_required(VERSION 3.28)
project(daygo_capture LANGUAGES CXX)

add_library(daygo_capture SHARED
  src/capture_abi.cpp
  src/capture_engine.cpp
  src/wgc_capture.cpp
  src/dxgi_capture.cpp
  src/privacy.cpp
  daygo_capture.def
)
target_compile_features(daygo_capture PRIVATE cxx_std_20)
target_compile_definitions(daygo_capture PRIVATE
  DAYGO_CAPTURE_BUILD WIN32_LEAN_AND_MEAN NOMINMAX
)
target_include_directories(daygo_capture PUBLIC ../include)
target_link_libraries(daygo_capture PRIVATE
  d3d11 dxgi windowsapp user32 shcore
)
set_property(TARGET daygo_capture PROPERTY
  MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>"
)
```

分别构建 x64 和 arm64，不生成一个混合 DLL：

```powershell
cmake -S native/windows -B build/native/windows-x64 `
  -G "Visual Studio 17 2022" -A x64 `
  -DCMAKE_SYSTEM_VERSION=10.0.26100.0
cmake --build build/native/windows-x64 --config Release

cmake -S native/windows -B build/native/windows-arm64 `
  -G "Visual Studio 17 2022" -A ARM64 `
  -DCMAKE_SYSTEM_VERSION=10.0.26100.0
cmake --build build/native/windows-arm64 --config Release
```

Wails 的 `preBuildHooks["windows/amd64"]` / `["windows/arm64"]` 调用对应构建脚本；post-build
把 DLL 放到主可执行文件旁的固定目录。EXE 与 DLL 都签名，安装包校验二者签名。开发、CI 和
 Release 使用同一个 CMake preset，不能在 Visual Studio GUI 中保存未落盘的隐式设置。

Windows 实验通过前，正式构建应默认排除 DLL；只有显式实验 build tag 才打包，防止用户误以为
隐私契约已经满足。

## 8. 实验计划与退出条件

### 8.1 已完成的本机编译探针

在 arm64 macOS 26、Apple Swift 6.3.3、仅 Command Line Tools 的环境中已验证：

1. 手写 C header + Swift `@c @implementation` 可编译成动态库；
2. Go cgo 可调用导出的 ABI，输出 `status=0 abi=1.0 sum=42`；
3. 同一源码可编译 arm64 与 x86_64 静态 archive，并由 `lipo` 合成通用 `.a`；
4. Go 可直接链接静态 Swift archive 并运行；
5. 以 macOS 14.0 为 deployment target 的
   `SCShareableContent → SCContentFilter(excludingApplications:) →
   SCScreenshotManager.captureSampleBuffer` 探针编译成功。

这些结果只证明编译和链接，不证明真实授权、截图内容、颜色、耗电、指示器或签名身份。
临时探针未写入仓库。

### 8.2 macOS 真实机器矩阵

| ID | 场景 | 必须观察的结果 |
|---|---|---|
| MC-1 | 未授权启动，用户不点击申请 | 无系统弹窗；状态 `not_determined`；无截图 |
| MC-2 | 用户显式申请后授权 | 下一次 Start 成功；升级同签名版本不重复授权 |
| MC-3 | 每 1 秒截图 10 分钟 | 每次只产生一帧；无持续录屏流；无常亮共享指示 |
| MC-4 | 双屏，光标跨边缘 | 遵守 10 pt/400 ms 滞后；只截稳定后的目标屏 |
| MC-5 | Retina/非 Retina/旋转屏 | 高度准确、宽度为偶数、比例正确、无拉伸 |
| MC-6 | SDR/HDR 显示器 | SDR 输出颜色可接受；无洗白或严重裁切 |
| MC-7 | 屏蔽应用前台 | 不调用真实截图，输出占位帧且 `redacted=true` |
| MC-8 | 屏蔽应用非前台但窗口可见 | 画面中无其窗口内容 |
| MC-9 | sleep/wake、lock/unlock、屏保 | 停止前收尾；按 5 s/0.5 s 延迟恢复；无旧显示器列表 |
| MC-10 | capture 中 Stop/Cmd+Q/崩溃 | Stop 收尾；崩溃后重放未 ack 事件；无重复 DB 行 |
| MC-11 | 开发签名、Release 签名、升级 | 权限归属符合预期；Release 升级不重新提示 |
| MC-12 | 24 h，1/10/60 秒间隔 | 无无界内存、线程、句柄增长；看门狗无未解释缺口 |

### 8.3 Windows 真实机器矩阵

| ID | 场景 | 比较 WGC / DXGI / BitBlt 的指标 |
|---|---|---|
| WC-1 | 每 10 秒一帧，10 分钟 | 边框/闪烁/系统提示、首帧延迟、CPU/GPU、句柄增长 |
| WC-2 | packaged / unpackaged | CreateForMonitor、borderless access、签名与升级行为 |
| WC-3 | Windows 10 22H2 / Windows 11 当前版 | API 可用性、捕获指示和输出一致性 |
| WC-4 | 双屏、不同 DPI、旋转、HDR | 目标屏、尺寸、旋转、色彩、tone mapping |
| WC-5 | 全屏 DirectX、浏览器视频、RDP | 黑帧/旧帧/受保护内容行为可诊断，不伪装成功 |
| WC-6 | lock/unlock、用户切换、睡眠、显卡重启 | 设备与 session 可恢复；无 busy loop |
| WC-7 | 前台屏蔽应用 | 完全不截图，只生成占位帧 |
| WC-8 | 非前台屏蔽窗口的全部边界 | GPU 遮蔽在透明、阴影、DPI、跨屏、子窗口下均无泄漏 |
| WC-9 | ABI DLL 缺失、版本不匹配、崩溃 | 致命且可见；无工作目录 DLL 搜索；可监管恢复 |
| WC-10 | 24 h | 无无界内存/VRAM/COM/D3D 对象增长；帧间隔连续 |

Windows 技术选择必须以矩阵结果决定：若 WGC 指示不可接受则比较 DXGI；若两者均无法满足隐私
双层保护，则 Windows 捕获结论是「不支持」，不是回退到更弱契约。

### 8.4 契约与 ABI 门禁

- `platformtest.Suite` 同时跑 fake 与真实适配层：Start/Stop 幂等、Close、ctx 取消、全局 seq、
  事件顺序、累计 Ack、重放、状态合并、权限拒绝；
- C 侧编译 `_Static_assert` 固定每个 v1 struct 的 `sizeof`/关键 `offsetof`；Swift 与 C++ 都 include
  同一 header；
- Go 侧契约测试构造最长合法 UTF-8 ID、空 ID、非法 UTF-8、零长数组、未知尾字段和 ABI major
  不匹配；
- fuzz ABI 输入长度与计数，确认乘法溢出、超大分配、路径越界和重复 release 不会穿透边界；
- Windows DLL 导出表与 `.def` 做 diff；macOS archive 用链接探针验证全部符号；
- Release 产物不得包含 test-only 像素复制符号。

## 9. 与现有文档的同步结果

本决策已同步以下上层契约，避免平台实现按旧草案固化：

1. `CaptureConfig.BlockedBundleIDs` 已改为跨平台的 `BlockedApplicationIDs`；
2. `RecordingStateDTO.ActiveDisplayID` 已从 `*uint32` 改为 `*string`；
3. 原 `Frames()`/缺失的 segment channel 已合并为有序 `Events()`；
4. `Capture` 已增加 `Ack` 和 `Close`；
5. 尚未实现的设置键已从 `privacy.blockedBundleIds` 改为
   `privacy.blockedApplicationIds`，无需用户数据迁移；
6. `09` 的待定项仍只在本文件实验矩阵通过后改为已决定。

## 10. 资料来源

### Apple / Swift

- [Apple WWDC23: What’s new in ScreenCaptureKit](https://developer.apple.com/videos/play/wwdc2023/10136/)
  — 一次性 screenshot、`SCContentFilter`、`captureSampleBuffer`/`captureImage`；
- [Apple: SCScreenshotManager](https://developer.apple.com/documentation/screencapturekit/scscreenshotmanager)
  — macOS 14 一次性 API 与 macOS 26 screenshot configuration；
- [Apple: Importing Swift into Objective-C](https://developer.apple.com/documentation/swift/importing-swift-into-objective-c)
  — Swift 兼容头和 framework 互操作；
- [Swift Evolution SE-0495: C compatible functions and enums](https://github.com/swiftlang/swift-evolution/blob/main/proposals/0495-cdecl.md)
  — Swift 6.3 正式 `@c`、C 头生成与 ABI 规则；
- [Apple: CGPreflightScreenCaptureAccess](https://developer.apple.com/documentation/coregraphics/cgpreflightscreencaptureaccess%28%29)
  — 屏幕录制授权预检。

### Microsoft

- [Microsoft: Screen capture](https://learn.microsoft.com/en-us/windows/apps/develop/media-authoring-processing/screen-capture)
  — WGC 单张截图、frame pool、像素格式、HDR 与 frame 生命周期；
- [Microsoft: IGraphicsCaptureItemInterop::CreateForMonitor](https://learn.microsoft.com/en-us/windows/win32/api/windows.graphics.capture.interop/nf-windows-graphics-capture-interop-igraphicscaptureiteminterop-createformonitor)
  — Win32 monitor interop 与最低版本；
- [Microsoft: Desktop Duplication API](https://learn.microsoft.com/en-us/windows/win32/direct3ddxgi/desktop-dup-api)
  — DXGI 帧、旋转、光标与恢复；
- [Microsoft: SetWindowDisplayAffinity](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setwindowdisplayaffinity)
  — 只能控制调用进程自己的窗口，不能排除任意第三方窗口；
- [Microsoft: Introduction to C++/WinRT](https://learn.microsoft.com/en-us/windows/apps/develop/cpp-winrt/intro-to-using-cpp-with-winrt)
  — Windows SDK 的标准 C++17+ WinRT projection；
- [Microsoft: Native code interop with Native AOT](https://learn.microsoft.com/en-us/dotnet/core/deploying/native-aot/interop)
  — NativeAOT C exports 的能力与限制；
- [Microsoft windows-rs](https://github.com/microsoft/windows-rs)
  — Rust 的 Windows/COM/WinRT 官方绑定候选。

### Wails

- [Wails v2.15 project config](https://wails.io/docs/reference/project-config/)
  — 按 GOOS/GOARCH 的 pre/post build hooks。
