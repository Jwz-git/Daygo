package app

// EventName is a stable Wails event identifier. Event delivery is never the data
// source: views render from Get* methods and use these only to invalidate or
// replace local state (docs/05 §5.5.3).
type EventName string

const (
	EventTimelineUpdated     EventName = "timeline:updated"
	EventJournalUpdated      EventName = "journal:updated"
	EventGoalUpdated         EventName = "goal:updated"
	EventSettingsChanged     EventName = "settings:changed"
	EventRecordingState      EventName = "recording:state"
	EventCapabilitiesChanged EventName = "capabilities:changed"
	EventPermissionChanged   EventName = "permission:changed"
	EventBatchProgress       EventName = "batch:progress"
	EventBatchFailed         EventName = "batch:failed"
	EventRecordingWarning    EventName = "recording:warning"
	EventUpdateAvailable     EventName = "update:available"
)

var eventNames = [...]EventName{
	EventTimelineUpdated,
	EventJournalUpdated,
	EventGoalUpdated,
	EventSettingsChanged,
	EventRecordingState,
	EventCapabilitiesChanged,
	EventPermissionChanged,
	EventBatchProgress,
	EventBatchFailed,
	EventRecordingWarning,
	EventUpdateAvailable,
}

// EventNames returns a defensive copy of the complete event-name contract.
func EventNames() []EventName {
	result := make([]EventName, len(eventNames))
	copy(result, eventNames[:])
	return result
}
