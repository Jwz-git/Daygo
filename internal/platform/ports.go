package platform

import "context"

// Capture performs one caller-driven screenshot of the current primary display.
// It owns no timer, recorder state, event stream, persistence, or segment encoder.
type Capture interface {
	Capture(ctx context.Context, req CaptureRequest) (CaptureResult, error)
}

// Media decodes frames out of segments and encodes timelapses. These signatures
// remain implementation-neutral until the M2 codec decision.
type Media interface {
	DecodeFrame(ctx context.Context, req DecodeRequest) ([]byte, error)
	DecodeFrames(ctx context.Context, reqs []DecodeRequest) ([][]byte, error)
	EncodeVideo(ctx context.Context, req EncodeRequest) (EncodeResult, error)
	ProbeSegment(ctx context.Context, path string) (SegmentInfo, error)
}

// System contains OS integration points other than capture, media and secrets.
type System interface {
	ScreenRecordingPermission(ctx context.Context) (PermissionState, error)
	NotificationsPermission(ctx context.Context) (PermissionState, error)
	RequestScreenRecordingPermission(ctx context.Context) error
	OpenSystemSettings(ctx context.Context, pane SettingsPane) error
	Displays(ctx context.Context) ([]Display, error)
	FrontmostApplication(ctx context.Context) (AppInfo, error)
	InstalledApplications(ctx context.Context) ([]AppInfo, error)
	LaunchAtLogin(ctx context.Context) (bool, error)
	SetLaunchAtLogin(ctx context.Context, enabled bool) error
	SetActivationPolicy(ctx context.Context, p ActivationPolicy) error
	SetStatusItem(ctx context.Context, s StatusItemState) error
	ScheduleNotification(ctx context.Context, n Notification) error
	CancelNotifications(ctx context.Context, ids []string) error
	Events() <-chan SystemEvent
}

// Secrets is the system keychain. Get is for Go's provider client only; no
// binding ever returns the value to the frontend.
type Secrets interface {
	Get(ctx context.Context, provider string) (string, error)
	Set(ctx context.Context, provider, secret string) error
	Delete(ctx context.Context, provider string) error
}

// Updater is provisional until the M5 update decision.
type Updater interface {
	CheckForUpdates(ctx context.Context, interactive bool) error
	State(ctx context.Context) (UpdaterState, error)
	SetAutomaticChecks(ctx context.Context, enabled bool) error
	Events() <-chan UpdaterEvent
}
