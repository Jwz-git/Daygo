package platform

// PermissionState is a system authorization status. docs/05 §5.3.3.
type PermissionState string

const (
	PermissionGranted       PermissionState = "granted"
	PermissionDenied        PermissionState = "denied"
	PermissionNotDetermined PermissionState = "not_determined"
)

func (p PermissionState) Valid() bool {
	switch p {
	case PermissionGranted, PermissionDenied, PermissionNotDetermined:
		return true
	default:
		return false
	}
}

// SettingsPane is the closed set of system-settings panes the app may open.
type SettingsPane string

const (
	PaneScreenRecording SettingsPane = "screen_recording"
	PaneNotifications   SettingsPane = "notifications"
	PaneLoginItems      SettingsPane = "login_items"
)

func (p SettingsPane) Valid() bool {
	switch p {
	case PaneScreenRecording, PaneNotifications, PaneLoginItems:
		return true
	default:
		return false
	}
}

// ActivationPolicy controls whether the process occupies the Dock.
type ActivationPolicy string

const (
	ActivationRegular    ActivationPolicy = "regular"
	ActivationAccessory  ActivationPolicy = "accessory"
	ActivationProhibited ActivationPolicy = "prohibited"
)

func (p ActivationPolicy) Valid() bool {
	switch p {
	case ActivationRegular, ActivationAccessory, ActivationProhibited:
		return true
	default:
		return false
	}
}

// CaptureEventKind identifies one ordered durable capture event.
type CaptureEventKind string

const (
	CaptureEventFrame         CaptureEventKind = "frame"
	CaptureEventSegmentClosed CaptureEventKind = "segment_closed"
)

func (k CaptureEventKind) Valid() bool {
	return k == CaptureEventFrame || k == CaptureEventSegmentClosed
}

// CapturePhase is the adapter's current capture lifecycle phase.
type CapturePhase string

const (
	CaptureIdle      CapturePhase = "idle"
	CaptureStarting  CapturePhase = "starting"
	CaptureCapturing CapturePhase = "capturing"
	CapturePaused    CapturePhase = "paused"
)

func (p CapturePhase) Valid() bool {
	switch p {
	case CaptureIdle, CaptureStarting, CaptureCapturing, CapturePaused:
		return true
	default:
		return false
	}
}

// SystemEventKind tags a message on System.Events(). Paired transitions must
// never be collapsed against one another (docs/05 §5.7.2).
type SystemEventKind string

const (
	EventSleep             SystemEventKind = "sleep"
	EventWake              SystemEventKind = "wake"
	EventScreenLocked      SystemEventKind = "screen_locked"
	EventScreenUnlocked    SystemEventKind = "screen_unlocked"
	EventScreensaverStart  SystemEventKind = "screensaver_start"
	EventScreensaverStop   SystemEventKind = "screensaver_stop"
	EventDisplaysChanged   SystemEventKind = "displays_changed"
	EventDeepLink          SystemEventKind = "deep_link"
	EventStatusItemClick   SystemEventKind = "status_item_clicked"
	EventNotificationClick SystemEventKind = "notification_clicked"
)

var pairedEvents = map[SystemEventKind]SystemEventKind{
	EventSleep: EventWake, EventWake: EventSleep,
	EventScreenLocked: EventScreenUnlocked, EventScreenUnlocked: EventScreenLocked,
	EventScreensaverStart: EventScreensaverStop, EventScreensaverStop: EventScreensaverStart,
}

func (k SystemEventKind) Paired() bool {
	_, ok := pairedEvents[k]
	return ok
}
