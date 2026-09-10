package app

// DiagnosticsDTO mirrors docs/05 §5.5.2. It exists so a storage or platform
// problem is visible to the user instead of silently degrading behavior.
//
// Every field is a count, a size, a path or a status word. No field may carry
// screen content, window titles, LLM payloads or credentials (docs/07 §7.5).
type DiagnosticsDTO struct {
	DatabasePath      string `json:"databasePath"`
	DatabaseBytes     int64  `json:"databaseBytes"`
	RecordingsBytes   int64  `json:"recordingsBytes"`
	LastCaptureAtTs   *int64 `json:"lastCaptureAtTs"`
	PendingBatches    int    `json:"pendingBatches"`
	FailedBatches     int    `json:"failedBatches"`
	NativeState       string `json:"nativeState"`
	CaptureOwnerPID   *int   `json:"captureOwnerPid"`
	SkippedCardsToday int    `json:"skippedCardsToday"`

	// DBStatus reports the storage layer's own state: "ok", "read_only" or
	// "unavailable". Together with Unavailable it lets the frontend tell a
	// genuinely empty counter from one whose data source does not exist.
	// Both fields were added to docs/05 §5.5.2 alongside this implementation.
	DBStatus string `json:"dbStatus"`

	// Unavailable lists diagnostics whose data source does not exist yet, with
	// the reason. A zero in a counter is otherwise indistinguishable from a
	// feature that has not shipped, and reporting a bare zero would be a
	// silent lie.
	Unavailable map[string]string `json:"unavailable,omitempty"`
}

// DBStatus values.
const (
	DBStatusOK          = "ok"
	DBStatusReadOnly    = "read_only"
	DBStatusUnavailable = "unavailable"
)

// NativeState values, from docs/05 §5.5.2.
const (
	NativeStateOK          = "ok"
	NativeStateRestarting  = "restarting"
	NativeStateUnavailable = "unavailable"
)
