package platform

import "context"

// Capture performs one caller-driven screenshot of the current primary display.
// It owns no timer, recorder state, event stream, persistence, or segment encoder.
type Capture interface {
	Capture(ctx context.Context, req CaptureRequest) (CaptureResult, error)
}

// SegmentCloser allows finalizing the active recording segment on pause/stop/shutdown.
type SegmentCloser interface {
	CloseActiveSegment(ctx context.Context) error
}

// CapturePrivacyReporter describes whether the platform can exclude selected
// applications from an image. It is separate from Capture so headless and
// older adapters remain source-compatible while the settings UI can report an
// honest OS gate.
type CapturePrivacyReporter interface {
	CapturePrivacyCompatibility(ctx context.Context) (CapturePrivacyCompatibility, error)
}

// ApplicationInspector resolves application identities for the screenshot
// privacy list. It owns no picker UI and never retains a supplied path.
type ApplicationInspector interface {
	// InspectApplication resolves one user-selected .app bundle.
	InspectApplication(ctx context.Context, path string) (ApplicationIdentity, error)
	// DescribeApplications resolves already-configured bundle identifiers in
	// input order, one result per identifier. An identifier the system cannot
	// resolve comes back with only ID set; only invalid input is an error.
	// Adapters without the capability return ID-only results rather than
	// failing, so the privacy list stays readable and editable everywhere.
	DescribeApplications(ctx context.Context, ids []string) ([]ApplicationIdentity, error)
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
	// InstalledApplications enumerates user-visible applications for the
	// privacy grid. Names resolve in the requested language (BCP-47 tag;
	// empty keeps the platform default) so the grid follows the app's UI
	// language rather than the host's.
	InstalledApplications(ctx context.Context, language string) ([]AppInfo, error)
	LaunchAtLogin(ctx context.Context) (bool, error)
	SetLaunchAtLogin(ctx context.Context, enabled bool) error
	SetActivationPolicy(ctx context.Context, p ActivationPolicy) error
	SetStatusItem(ctx context.Context, s StatusItemState) error
	// RevealPath opens an absolute path in the OS file browser (Finder on
	// macOS, Explorer on Windows). The app layer resolves and creates the path;
	// the adapter only hands it to the system so it never constructs paths of
	// its own. Used by the menu-bar "open recordings folder" action.
	RevealPath(ctx context.Context, path string) error
	ScheduleNotification(ctx context.Context, n Notification) error
	CancelNotifications(ctx context.Context, ids []string) error
	Events() <-chan SystemEvent
}

// Relauncher is an optional System capability: it re-launches the app after the
// current process has exited. A System adapter opts in by implementing it; the
// app layer type-asserts and degrades to a plain quit when it is absent.
//
// It exists for the permission-change restart: macOS caches a screen-recording
// (TCC) grant at process launch, and Daygo is a resident agent whose ordinary
// quit only hides the window. To make a fresh grant take effect the process must
// fully terminate and come back, so the adapter schedules a relaunch that waits
// for this process to exit before starting a new one (docs/decisions/
// recording-screen-recording-permission.md).
type Relauncher interface {
	Relaunch(ctx context.Context) error
}

// StatusItemAvailability confirms that the native recovery entry point exists.
// Unlike SetStatusItem (which may enqueue a repaint), this is an applied-state
// query. It is only called by host lifecycle actions, never recorder callbacks.
type StatusItemAvailability interface {
	StatusItemAvailable(ctx context.Context) (bool, error)
}

// ApplicationMenuCopySink is implemented by hosts with a native main menu.
type ApplicationMenuCopySink interface {
	SetApplicationMenuLabels(ctx context.Context, labels ApplicationMenuLabels) error
}

// StatusMessagePresenter shows a localized message without blocking the system
// event consumer. The host owns dismissal and closes it during teardown.
type StatusMessagePresenter interface {
	ShowStatusMessage(ctx context.Context, message StatusMessage) error
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

// UpdateInstallCoordinator lets the app preserve recorder and lock invariants
// while a platform updater owns download, verification and replacement.
// Callbacks may arrive from a native worker thread and must be thread-safe.
type UpdateInstallCoordinator interface {
	SetInstallCallbacks(canInstall func() bool, prepare func() error, requestShutdown func())
}

// UpdateCopySink receives the localized copy a platform updater shows in its
// own dialogs. The adapter holds no locale: the frontend pushes the translated
// bundle (docs/05 §5.5.1) and the app layer forwards it. Adapters whose update
// UI is entirely rendered by the system do not implement it.
type UpdateCopySink interface {
	// SetInstallRefusedMessage sets the copy shown when an install is refused
	// because this instance cannot own it. An empty message keeps the previous
	// copy rather than blanking the dialog.
	SetInstallRefusedMessage(message string)
}
