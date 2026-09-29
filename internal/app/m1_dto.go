package app

// RecordingState is the closed set of recording lifecycle states exposed across
// the application boundary.
type RecordingState string

const (
	RecordingStateIdle      RecordingState = "idle"
	RecordingStateStarting  RecordingState = "starting"
	RecordingStateCapturing RecordingState = "capturing"
	RecordingStatePaused    RecordingState = "paused"
)

// Valid reports whether the state is part of the recording state contract.
func (s RecordingState) Valid() bool {
	switch s {
	case RecordingStateIdle, RecordingStateStarting, RecordingStateCapturing, RecordingStatePaused:
		return true
	default:
		return false
	}
}

type DayContextDTO struct {
	Day             string `json:"day"`
	StandupDay      string `json:"standupDay"`
	WeekStart       string `json:"weekStart"` // Monday of the week containing the logical day
	DayStartTs      int64  `json:"dayStartTs"`
	DayEndTs        int64  `json:"dayEndTs"`
	NowTs           int64  `json:"nowTs"`
	TimeZone        string `json:"timeZone"`
	DayBoundaryHour int    `json:"dayBoundaryHour"`
}

type CapabilitiesDTO struct {
	CanWrite       bool     `json:"canWrite"`
	IsCaptureOwner bool     `json:"isCaptureOwner"`
	Features       []string `json:"features"`
	AppVersion     string   `json:"appVersion"`
	APIRevision    int      `json:"apiRevision"`
}

type RecordingStateDTO struct {
	State           RecordingState `json:"state"`
	Reason          *string        `json:"reason"`
	StopCause       *string        `json:"stopCause"`
	UserPaused      bool           `json:"userPaused"`
	PauseEndsAtTs   *int64         `json:"pauseEndsAtTs"`
	Permission      string         `json:"permission"`
	IsCaptureOwner  bool           `json:"isCaptureOwner"`
	ActiveDisplayID *string        `json:"activeDisplayId"`
	LastFrameAtTs   *int64         `json:"lastFrameAtTs"`
}

type PermissionDTO struct {
	ScreenRecording string `json:"screenRecording"`
	Notifications   string `json:"notifications"`
	CanRequest      bool   `json:"canRequest"`
}

// UpdaterStateDTO is the update status the frontend reads (docs/05 §5.5.2).
// Download progress and "ready to relaunch" stay out of this shape: the Sparkle
// standard UI carries them, so the frozen port and this DTO need no richer
// fields (docs/decisions/delivery-auto-update.md §5).
type UpdaterStateDTO struct {
	Automatic        bool    `json:"automatic"`
	Checking         bool    `json:"checking"`
	AvailableVersion *string `json:"availableVersion"`
	LastCheckedAtTs  *int64  `json:"lastCheckedAtTs"`
}
