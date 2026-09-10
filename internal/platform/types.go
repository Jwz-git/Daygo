package platform

import (
	"path"
	"sort"
	"strings"
	"time"
)

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

// CanonicalizeCaptureConfig deep-copies pointer/slice fields and sorts blocked
// IDs, defining the "same config" rule without retaining caller-owned memory.
func CanonicalizeCaptureConfig(cfg CaptureConfig) CaptureConfig {
	result := cfg
	result.BlockedApplicationIDs = append([]string(nil), cfg.BlockedApplicationIDs...)
	sort.Strings(result.BlockedApplicationIDs)
	if cfg.PreferredDisplayID != nil {
		id := *cfg.PreferredDisplayID
		result.PreferredDisplayID = &id
	}
	return result
}

func CaptureConfigEqual(left, right CaptureConfig) bool {
	left = CanonicalizeCaptureConfig(left)
	right = CanonicalizeCaptureConfig(right)
	if left.Interval != right.Interval || left.CaptureHeight != right.CaptureHeight ||
		left.SegmentDirectory != right.SegmentDirectory || left.ShowsCursor != right.ShowsCursor ||
		left.SegmentMaxFrames != right.SegmentMaxFrames || left.SegmentMaxDuration != right.SegmentMaxDuration ||
		!equalOptionalString(left.PreferredDisplayID, right.PreferredDisplayID) ||
		len(left.BlockedApplicationIDs) != len(right.BlockedApplicationIDs) {
		return false
	}
	for i := range left.BlockedApplicationIDs {
		if left.BlockedApplicationIDs[i] != right.BlockedApplicationIDs[i] {
			return false
		}
	}
	return true
}

func equalOptionalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// CaptureEvent keeps frame and segment-close messages in one durable order.
type CaptureEvent struct {
	Seq     uint64
	Kind    CaptureEventKind
	Frame   *CapturedFrame
	Segment *SegmentClosed
}

type CapturedFrame struct {
	SegmentPath string
	FrameIndex  int
	CapturedAt  time.Time
	IdleSeconds *int
	DisplayID   string
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

type CaptureStatus struct {
	Phase           CapturePhase
	Permission      PermissionState
	ActiveDisplayID *string
	LastFrameAt     *time.Time
	Fault           *CaptureFault
}

type CaptureFault struct {
	Code      string
	Retryable bool
	Message   string
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
