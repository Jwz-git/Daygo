package platform

import "context"

// Capture produces discrete screenshots. Implementations keep pixels inside the
// adapter and report only durable frame metadata and segment lifecycle events.
type Capture interface {
	// Start accepts a complete config snapshot. Repeating the same config is a
	// no-op; a different config requires Stop first. ctx bounds only this command.
	Start(ctx context.Context, cfg CaptureConfig) error
	// Stop is idempotent and finalizes the current segment. It does not close the
	// streams, because a later Start may reuse this object.
	Stop(ctx context.Context) error
	// Ack cumulatively acknowledges seq and every preceding contiguous event after
	// Go commits their database effects.
	Ack(ctx context.Context, seq uint64) error
	// Events is the ordered, durable frame/segment-closed stream.
	Events() <-chan CaptureEvent
	// Status is a mergeable current-state stream; it is not acknowledged.
	Status() <-chan CaptureStatus
	// Close releases native resources and closes both streams. It is called once.
	Close(ctx context.Context) error
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
