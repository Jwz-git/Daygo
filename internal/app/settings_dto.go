package app

// Settings DTOs mirror docs/05 §5.5.2. Every field carries an explicit json tag
// in lowerCamelCase (docs/05 §5.3.1); the frontend reads these names directly,
// so a change here is a breaking wire change.

// SettingsDTO is the full effective settings state, with defaults applied and
// values normalized.
type SettingsDTO struct {
	Capture       CaptureSettingsDTO      `json:"capture"`
	Privacy       PrivacySettingsDTO      `json:"privacy"`
	Storage       StorageSettingsDTO      `json:"storage"`
	Notifications NotificationSettingsDTO `json:"notifications"`
	Appearance    AppearanceSettingsDTO   `json:"appearance"`
	LLM           LLMSettingsDTO          `json:"llm"`
	Chat          ChatSettingsDTO         `json:"chat"`
	System        SystemSettingsDTO       `json:"system"`
	Telemetry     TelemetrySettingsDTO    `json:"telemetry"`
}

type CaptureSettingsDTO struct {
	IntervalSeconds int `json:"intervalSeconds"`
	CaptureHeight   int `json:"captureHeight"`
}

type PrivacySettingsDTO struct {
	BlockedApplicationIDs []string `json:"blockedApplicationIds"`
}

type StorageSettingsDTO struct {
	RecordingsLimitBytes int64 `json:"recordingsLimitBytes"`
}

type NotificationSettingsDTO struct {
	JournalReminderEnabled bool   `json:"journalReminderEnabled"`
	JournalReminderTime    string `json:"journalReminderTime"`
}

type AppearanceSettingsDTO struct {
	Theme string `json:"theme"`
	// Language is the one documented sentinel (docs/05 §5.3.3): an empty string
	// means "follow the system", which is a real preference, not a missing
	// value. It is therefore a plain string, not a pointer.
	Language string `json:"language"`
}

// LLMSettingsDTO is independent of AppearanceSettingsDTO.Language: this one
// decides what language the model writes card titles and summaries in, the
// other only affects interface text.
type LLMSettingsDTO struct {
	OutputLanguage                string `json:"outputLanguage"`
	RecognitionEnhancementEnabled bool   `json:"recognitionEnhancementEnabled"`
}

type SystemSettingsDTO struct {
	LaunchAtLogin     bool `json:"launchAtLogin"`
	ShowDockIcon      bool `json:"showDockIcon"`
	AgentEditsEnabled bool `json:"agentEditsEnabled"`
}

// ChatSettingsDTO carries the global chat memory: user-authored free text
// (like a CLAUDE.md) injected into every conversation's system prompt
// (decisions/chat-session-model). EditMode is the chat agent sandbox gate
// (docs/05 §5.12): "readonly" (default) or "edits"; it is independent of
// SystemSettingsDTO.AgentEditsEnabled, which gates the agent.sock channel.
type ChatSettingsDTO struct {
	Memory   string `json:"memory"`
	EditMode string `json:"editMode"`
}

type TelemetrySettingsDTO struct {
	AnalyticsOptIn      bool `json:"analyticsOptIn"`
	CrashReportingOptIn bool `json:"crashReportingOptIn"`
}

// SettingsPatchDTO is a partial update. Every field is a pointer because nil is
// the only representation of "this call does not change this setting": a plain
// bool or string could not distinguish "leave it" from "set it to false/empty"
// (docs/05 §5.5.1).
type SettingsPatchDTO struct {
	IntervalSeconds               *int      `json:"intervalSeconds"`
	CaptureHeight                 *int      `json:"captureHeight"`
	BlockedApplicationIDs         *[]string `json:"blockedApplicationIds"`
	RecordingsLimitBytes          *int64    `json:"recordingsLimitBytes"`
	JournalReminderEnabled        *bool     `json:"journalReminderEnabled"`
	JournalReminderTime           *string   `json:"journalReminderTime"`
	Theme                         *string   `json:"theme"`
	Language                      *string   `json:"language"`
	OutputLanguage                *string   `json:"outputLanguage"`
	RecognitionEnhancementEnabled *bool     `json:"recognitionEnhancementEnabled"`
	ChatMemory                    *string   `json:"chatMemory"`
	ChatEditMode                  *string   `json:"chatEditMode"`
	LaunchAtLogin                 *bool     `json:"launchAtLogin"`
	ShowDockIcon                  *bool     `json:"showDockIcon"`
	AgentEditsEnabled             *bool     `json:"agentEditsEnabled"`
	AnalyticsOptIn                *bool     `json:"analyticsOptIn"`
	CrashReportingOptIn           *bool     `json:"crashReportingOptIn"`
}
