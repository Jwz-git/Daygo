package platform

import (
	"path"
	"strings"
	"time"
)

// CaptureRequest is the complete input for one screenshot attempt. OutputPath
// is an absolute, not-yet-existing JPEG path allocated by Go storage.
type CaptureRequest struct {
	OutputPath            string
	ImageFormat           CaptureImageFormat
	TargetHeight          int
	JPEGQuality           int
	ShowsCursor           bool
	BlockedApplicationIDs []string
}

// CaptureResult describes one completed call. Image metadata is valid only
// when Outcome is CaptureWritten.
type CaptureResult struct {
	Outcome    CaptureOutcome
	CapturedAt time.Time
	Width      int
	Height     int
	FileSize   int64
}

// CaptureError is the stable Go-facing classification of a failed screenshot.
// NativeCode is diagnostic only; callers branch on Code.
type CaptureError struct {
	Code       CaptureErrorCode
	NativeCode int64
}

func (e *CaptureError) Error() string {
	if e == nil {
		return "capture failed"
	}
	return "capture: " + string(e.Code)
}

// ValidSegmentPath reports whether value is a canonical relative path below a
// capture's SegmentDirectory. Resolution against that root is the caller's next
// check; an absolute path or traversal is never accepted from an adapter.
func ValidSegmentPath(value string) bool {
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return false
	}
	clean := path.Clean(value)
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, "../") && clean == value
}

// Media value shapes are provisional until the M2 codec decision.
type DecodeRequest struct {
	SegmentPath  string
	FrameIndex   int
	MaxPixelSize int
}

type EncodeRequest struct {
	SegmentPath string
	FromFrame   int
	ToFrame     int
	OutputPath  string
}

type EncodeResult struct {
	OutputPath string
	Bytes      int64
}

type SegmentInfo struct {
	FrameCount int
	Width      int
	Height     int
	Readable   bool
}

type Display struct {
	ID      string
	Name    string
	Width   int
	Height  int
	Primary bool
}

type AppInfo struct {
	ID   string
	Name string
}

// Native UI and updater shapes remain provisional until their decisions land.
type StatusItemState struct {
	Visible bool
	Title   string
	Tooltip string
}

type Notification struct {
	ID        string
	Title     string
	Body      string
	DeliverAt *time.Time
}

type SystemEvent struct {
	Kind SystemEventKind
	At   time.Time
	Data SystemEventData
}

type SystemEventData struct {
	DeepLinkURL    *string
	StatusItemID   *string
	NotificationID *string
}

type UpdaterState struct {
	Automatic        bool
	Checking         bool
	AvailableVersion *string
	LastCheckedAt    *time.Time
}

type UpdaterEvent struct {
	State UpdaterState
	Err   error
}
