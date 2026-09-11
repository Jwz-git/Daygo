// Package settings is the typed layer over the app_settings table. It owns the
// meaning of every setting key: its default, the shape of its stored JSON, and
// the normalization and clamping that turn a caller's value into the value that
// actually takes effect (docs/05 §5.6.3 rule 2).
//
// It does NOT own SQL. Every read and write goes through storage.SettingsRepo,
// so internal/storage stays the only package that touches the database
// (docs/05 §5.6.2 rule 1).
//
// It also does not own policy. Which settings a given user may change, and
// when, is decided by the app layer (docs/02 §2.1); this package answers only
// "what is this setting, and what values are valid for it".
package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Keys are the contract (docs/05 §5.6.3 rule 1, docs/03 §3.3.5). Renaming one
// requires a migration because the stored name is what other modules read.
const (
	KeyCaptureIntervalSeconds       = "capture.intervalSeconds"
	KeyCaptureHeightPixels          = "capture.heightPixels"
	KeyPrivacyBlockedApplicationIDs = "privacy.blockedApplicationIds"
	KeyStorageRecordingsLimitBytes  = "storage.recordingsLimitBytes"
	KeyNotificationsReminderEnabled = "notifications.journalReminderEnabled"
	KeyNotificationsReminderTime    = "notifications.journalReminderTime"
	KeyAppearanceTheme              = "appearance.theme"
	KeyAppearanceLanguage           = "appearance.language"
	KeySystemLaunchAtLogin          = "system.launchAtLogin"
	KeySystemShowDockIcon           = "system.showDockIcon"
	KeySystemAgentEditsEnabled      = "system.agentEditsEnabled"
	KeyTelemetryAnalyticsOptIn      = "telemetry.analyticsOptIn"
	KeyTelemetryCrashReportingOptIn = "telemetry.crashReportingOptIn"
	KeyProvidersRouting             = "providers.routing"
	KeyLLMOutputLanguage            = "llm.outputLanguage"
	KeyLLMRecognitionEnhancement    = "llm.recognitionEnhancementEnabled"
)

// AllKeys lists every setting key. It exists so a test can assert the stored
// key set matches the contract rather than trusting the two to stay in step by
// hand.
func AllKeys() []string {
	return []string{
		KeyCaptureIntervalSeconds,
		KeyCaptureHeightPixels,
		KeyPrivacyBlockedApplicationIDs,
		KeyStorageRecordingsLimitBytes,
		KeyNotificationsReminderEnabled,
		KeyNotificationsReminderTime,
		KeyAppearanceTheme,
		KeyAppearanceLanguage,
		KeySystemLaunchAtLogin,
		KeySystemShowDockIcon,
		KeySystemAgentEditsEnabled,
		KeyTelemetryAnalyticsOptIn,
		KeyTelemetryCrashReportingOptIn,
		KeyProvidersRouting,
		KeyLLMOutputLanguage,
		KeyLLMRecognitionEnhancement,
	}
}

// Allowed values from docs/03 §3.3.5 and docs/05 §5.3.3. These are the closed
// sets the normalizer clamps into; a value outside them is replaced by the
// default rather than stored, so the database never holds a value the rest of
// the system would have to defend against.
var (
	AllowedCaptureIntervals = []int{1, 5, 10, 20, 30, 60}
	AllowedCaptureHeights   = []int{720, 1080}
	AllowedThemes           = []string{"system", "light", "dark"}
)

// DefaultCaptureIntervalSeconds and friends are the defaults from
// docs/03 §3.3.5. They live here rather than in storage because a default is a
// statement about the setting's meaning, not about how it is stored.
const (
	DefaultCaptureIntervalSeconds = 10
	DefaultCaptureHeightPixels    = 1080
	DefaultRecordingsLimitBytes   = 0
	DefaultReminderEnabled        = false
	DefaultReminderTime           = "18:00"
	DefaultTheme                  = "system"
	DefaultLanguage               = "zh-CN"
	DefaultLaunchAtLogin          = false
	DefaultShowDockIcon           = true
	DefaultAgentEditsEnabled      = false
	DefaultAnalyticsOptIn         = false
	DefaultCrashReportingOptIn    = false
	DefaultOutputLanguage         = ""
	DefaultRecognitionEnhancement = false
)

// repo is the storage side of this package. It is an interface defined here,
// at the consumer, so settings does not depend on storage's concrete type and a
// test double needs no database (docs/02 §2.1).
type repo interface {
	Get(ctx context.Context, key string) (string, bool, error)
	GetAll(ctx context.Context) (map[string]string, error)
	Set(ctx context.Context, key, value string) error
	SetMany(ctx context.Context, values map[string]string) error
	Delete(ctx context.Context, key string) error
}

// Settings is the typed accessor. It holds no state beyond its repository, so
// the same value serves every caller and there is no cache to invalidate.
type Settings struct {
	repo repo
}

// New builds a typed accessor over a repository.
func New(r repo) *Settings {
	return &Settings{repo: r}
}

// Snapshot is every setting with defaults applied. A key absent from the
// database takes its default rather than reporting as missing, because callers
// ask "what is this setting" and the answer is never "unknown".
type Snapshot struct {
	CaptureIntervalSeconds int
	CaptureHeightPixels    int
	BlockedApplicationIDs  []string
	RecordingsLimitBytes   int64
	ReminderEnabled        bool
	ReminderTime           string
	Theme                  string
	Language               string
	LaunchAtLogin          bool
	ShowDockIcon           bool
	AgentEditsEnabled      bool
	AnalyticsOptIn         bool
	CrashReportingOptIn    bool
	ProvidersRouting       Routing
	OutputLanguage         string
	RecognitionEnhancement bool
}

// Routing is the stored form of providers.routing (docs/03 §3.3.5). An empty
// secondary means no fallback, which is why it is a pointer-free empty string
// rather than a sentinel.
type Routing struct {
	Primary   string `json:"primary"`
	Secondary string `json:"secondary"`
}

// Load reads every setting and applies defaults and normalization.
//
// Normalization runs on read as well as on write. A value written by an older
// build, or edited outside the app, must not reach the rest of the system
// unvalidated just because it is already stored.
func (s *Settings) Load(ctx context.Context) (Snapshot, error) {
	raw, err := s.repo.GetAll(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	return s.snapshotFrom(raw), nil
}

// Get reads one key with its default applied.
func (s *Settings) Get(ctx context.Context, key string) (string, error) {
	raw, ok, err := s.repo.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if !ok {
		return defaultFor(key), nil
	}
	return normalizeScalar(key, raw), nil
}

// Patch is a partial update. A nil field means "not provided by this call", so
// a caller cannot accidentally overwrite a setting it did not mention
// (docs/05 §5.5.1). This is the only representation that distinguishes
// "unchanged" from "set to empty/false".
type Patch struct {
	IntervalSeconds        *int
	CaptureHeight          *int
	BlockedApplicationIDs  *[]string
	RecordingsLimitBytes   *int64
	JournalReminderEnabled *bool
	JournalReminderTime    *string
	Theme                  *string
	Language               *string
	OutputLanguage         *string
	RecognitionEnhancement *bool
	LaunchAtLogin          *bool
	ShowDockIcon           *bool
	AgentEditsEnabled      *bool
	AnalyticsOptIn         *bool
	CrashReportingOptIn    *bool
}

// Apply writes the keys the patch actually carries and returns the full
// snapshot that takes effect afterwards.
//
// The return is the post-normalization state, which may differ from what the
// caller sent: docs/05 §5.3.4 makes UpdateSettings the one write method that
// returns a full snapshot precisely because cross-key normalization means the
// frontend cannot predict the result.
//
// All provided keys are written in one transaction, so a settings group cannot
// be left half-applied.
func (s *Settings) Apply(ctx context.Context, p Patch) (Snapshot, []string, error) {
	values, changed, err := s.encodePatch(p)
	if err != nil {
		return Snapshot{}, nil, err
	}
	if len(values) > 0 {
		if err := s.repo.SetMany(ctx, values); err != nil {
			return Snapshot{}, nil, err
		}
	}
	snapshot, err := s.Load(ctx)
	if err != nil {
		return Snapshot{}, nil, err
	}
	return snapshot, changed, nil
}

// encodePatch validates and encodes the provided fields, returning the stored
// values and the list of keys they map to.
func (s *Settings) encodePatch(p Patch) (map[string]string, []string, error) {
	values := make(map[string]string)
	changed := make([]string, 0, 15)

	put := func(key string, value any) error {
		encoded, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("settings: encode %s: %w", key, err)
		}
		values[key] = string(encoded)
		changed = append(changed, key)
		return nil
	}

	if p.IntervalSeconds != nil {
		if err := put(KeyCaptureIntervalSeconds, clampInt(*p.IntervalSeconds, AllowedCaptureIntervals, DefaultCaptureIntervalSeconds)); err != nil {
			return nil, nil, err
		}
	}
	if p.CaptureHeight != nil {
		if err := put(KeyCaptureHeightPixels, clampInt(*p.CaptureHeight, AllowedCaptureHeights, DefaultCaptureHeightPixels)); err != nil {
			return nil, nil, err
		}
	}
	if p.BlockedApplicationIDs != nil {
		ids := dedupeStrings(*p.BlockedApplicationIDs)
		if ids == nil {
			// Store an empty array, not null: the stored form must be a list so
			// a reader never has to distinguish two kinds of "nothing blocked".
			ids = []string{}
		}
		if err := put(KeyPrivacyBlockedApplicationIDs, ids); err != nil {
			return nil, nil, err
		}
	}
	if p.RecordingsLimitBytes != nil {
		limit := *p.RecordingsLimitBytes
		if limit < 0 {
			// Negative is meaningless as a byte count. Clamp to "no limit"
			// rather than rejecting: the user's intent (stop limiting) is clear.
			limit = DefaultRecordingsLimitBytes
		}
		if err := put(KeyStorageRecordingsLimitBytes, limit); err != nil {
			return nil, nil, err
		}
	}
	if p.JournalReminderEnabled != nil {
		if err := put(KeyNotificationsReminderEnabled, *p.JournalReminderEnabled); err != nil {
			return nil, nil, err
		}
	}
	if p.JournalReminderTime != nil {
		// An unparseable time falls back to the default rather than being
		// rejected: a reminder at the wrong time is recoverable, a settings
		// write that fails with no explanation is not.
		if err := put(KeyNotificationsReminderTime, normalizeClockTime(*p.JournalReminderTime)); err != nil {
			return nil, nil, err
		}
	}
	if p.Theme != nil {
		if err := put(KeyAppearanceTheme, normalizeMember(*p.Theme, AllowedThemes, DefaultTheme)); err != nil {
			return nil, nil, err
		}
	}
	if p.Language != nil {
		if err := put(KeyAppearanceLanguage, normalizeLanguage(*p.Language)); err != nil {
			return nil, nil, err
		}
	}
	if p.OutputLanguage != nil {
		// Empty means "follow the interface language" and is a real value, not
		// a missing one, so it is preserved rather than replaced by a default.
		if err := put(KeyLLMOutputLanguage, normalizeLanguage(*p.OutputLanguage)); err != nil {
			return nil, nil, err
		}
	}
	if p.RecognitionEnhancement != nil {
		if err := put(KeyLLMRecognitionEnhancement, *p.RecognitionEnhancement); err != nil {
			return nil, nil, err
		}
	}
	if p.LaunchAtLogin != nil {
		if err := put(KeySystemLaunchAtLogin, *p.LaunchAtLogin); err != nil {
			return nil, nil, err
		}
	}
	if p.ShowDockIcon != nil {
		if err := put(KeySystemShowDockIcon, *p.ShowDockIcon); err != nil {
			return nil, nil, err
		}
	}
	if p.AgentEditsEnabled != nil {
		if err := put(KeySystemAgentEditsEnabled, *p.AgentEditsEnabled); err != nil {
			return nil, nil, err
		}
	}
	if p.AnalyticsOptIn != nil {
		if err := put(KeyTelemetryAnalyticsOptIn, *p.AnalyticsOptIn); err != nil {
			return nil, nil, err
		}
	}
	if p.CrashReportingOptIn != nil {
		if err := put(KeyTelemetryCrashReportingOptIn, *p.CrashReportingOptIn); err != nil {
			return nil, nil, err
		}
	}
	return values, changed, nil
}

// snapshotFrom applies defaults and per-key normalization to stored values.
func (s *Settings) snapshotFrom(raw map[string]string) Snapshot {
	return Snapshot{
		CaptureIntervalSeconds: clampInt(decodeInt(raw[KeyCaptureIntervalSeconds], DefaultCaptureIntervalSeconds), AllowedCaptureIntervals, DefaultCaptureIntervalSeconds),
		CaptureHeightPixels:    clampInt(decodeInt(raw[KeyCaptureHeightPixels], DefaultCaptureHeightPixels), AllowedCaptureHeights, DefaultCaptureHeightPixels),
		BlockedApplicationIDs:  decodeStrings(raw[KeyPrivacyBlockedApplicationIDs]),
		RecordingsLimitBytes:   decodeInt64(raw[KeyStorageRecordingsLimitBytes], DefaultRecordingsLimitBytes),
		ReminderEnabled:        decodeBool(raw[KeyNotificationsReminderEnabled], DefaultReminderEnabled),
		ReminderTime:           normalizeClockTime(decodeString(raw[KeyNotificationsReminderTime], DefaultReminderTime)),
		Theme:                  normalizeMember(decodeString(raw[KeyAppearanceTheme], DefaultTheme), AllowedThemes, DefaultTheme),
		Language:               normalizeLanguage(decodeString(raw[KeyAppearanceLanguage], DefaultLanguage)),
		LaunchAtLogin:          decodeBool(raw[KeySystemLaunchAtLogin], DefaultLaunchAtLogin),
		ShowDockIcon:           decodeBool(raw[KeySystemShowDockIcon], DefaultShowDockIcon),
		AgentEditsEnabled:      decodeBool(raw[KeySystemAgentEditsEnabled], DefaultAgentEditsEnabled),
		AnalyticsOptIn:         decodeBool(raw[KeyTelemetryAnalyticsOptIn], DefaultAnalyticsOptIn),
		CrashReportingOptIn:    decodeBool(raw[KeyTelemetryCrashReportingOptIn], DefaultCrashReportingOptIn),
		ProvidersRouting:       decodeRouting(raw[KeyProvidersRouting]),
		OutputLanguage:         normalizeLanguage(decodeString(raw[KeyLLMOutputLanguage], DefaultOutputLanguage)),
		RecognitionEnhancement: decodeBool(raw[KeyLLMRecognitionEnhancement], DefaultRecognitionEnhancement),
	}
}

// clockTimePattern matches the stored HH:mm form.
var clockTimePattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):([0-5][0-9])$`)

// normalizeClockTime returns value when it is a valid HH:mm, otherwise the
// default. A value like "9:5" is corrected rather than rejected, because the
// user's intent is unambiguous.
func normalizeClockTime(value string) string {
	value = strings.TrimSpace(value)
	if clockTimePattern.MatchString(value) {
		return value
	}
	if hour, minute, ok := parseLooseClockTime(value); ok {
		return fmt.Sprintf("%02d:%02d", hour, minute)
	}
	return DefaultReminderTime
}

// parseLooseClockTime accepts H:mm and H:m in addition to HH:mm.
func parseLooseClockTime(value string) (int, int, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	var hour, minute int
	if _, err := fmt.Sscanf(parts[0], "%d", &hour); err != nil {
		return 0, 0, false
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &minute); err != nil {
		return 0, 0, false
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, false
	}
	return hour, minute, true
}

// normalizeLanguage applies the BCP 47 folding from docs/02 §2.5.1. The empty
// string is preserved: it is the one documented sentinel (docs/05 §5.3.3) and
// means "follow the system".
func normalizeLanguage(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lowered := strings.ToLower(value)
	switch {
	case lowered == "zh" || strings.HasPrefix(lowered, "zh-hans") || strings.HasPrefix(lowered, "zh-cn"):
		return "zh-CN"
	case strings.HasPrefix(lowered, "en"):
		return "en"
	default:
		return DefaultLanguage
	}
}

// normalizeMember returns value when it belongs to allowed, otherwise fallback.
func normalizeMember(value string, allowed []string, fallback string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return fallback
}

// clampInt returns value when it appears in allowed, otherwise fallback.
// Settings with a closed value set snap to a valid value rather than rejecting
// the write, so a stray value cannot make the setting unusable.
func clampInt(value int, allowed []int, fallback int) int {
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return fallback
}

// dedupeStrings removes empty and repeated entries while preserving order, so
// the stored list is stable and a golden comparison is meaningful.
func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

// defaultFor returns the default stored form of a key, used when the key has
// never been written.
func defaultFor(key string) string {
	switch key {
	case KeyCaptureIntervalSeconds:
		return encodeScalar(DefaultCaptureIntervalSeconds)
	case KeyCaptureHeightPixels:
		return encodeScalar(DefaultCaptureHeightPixels)
	case KeyPrivacyBlockedApplicationIDs:
		return "[]"
	case KeyStorageRecordingsLimitBytes:
		return encodeScalar(DefaultRecordingsLimitBytes)
	case KeyNotificationsReminderEnabled:
		return encodeScalar(DefaultReminderEnabled)
	case KeyNotificationsReminderTime:
		return encodeScalar(DefaultReminderTime)
	case KeyAppearanceTheme:
		return encodeScalar(DefaultTheme)
	case KeyAppearanceLanguage:
		return encodeScalar(DefaultLanguage)
	case KeySystemLaunchAtLogin:
		return encodeScalar(DefaultLaunchAtLogin)
	case KeySystemShowDockIcon:
		return encodeScalar(DefaultShowDockIcon)
	case KeySystemAgentEditsEnabled:
		return encodeScalar(DefaultAgentEditsEnabled)
	case KeyTelemetryAnalyticsOptIn:
		return encodeScalar(DefaultAnalyticsOptIn)
	case KeyTelemetryCrashReportingOptIn:
		return encodeScalar(DefaultCrashReportingOptIn)
	case KeyProvidersRouting:
		return `{"primary":"","secondary":""}`
	case KeyLLMOutputLanguage:
		return encodeScalar(DefaultOutputLanguage)
	case KeyLLMRecognitionEnhancement:
		return encodeScalar(DefaultRecognitionEnhancement)
	default:
		return ""
	}
}

// normalizeScalar applies a key's normalization to a stored string. It exists
// so a single-key read agrees with a full load.
func normalizeScalar(key, raw string) string {
	switch key {
	case KeyCaptureIntervalSeconds:
		value := clampInt(decodeInt(raw, DefaultCaptureIntervalSeconds), AllowedCaptureIntervals, DefaultCaptureIntervalSeconds)
		return encodeScalar(value)
	case KeyCaptureHeightPixels:
		value := clampInt(decodeInt(raw, DefaultCaptureHeightPixels), AllowedCaptureHeights, DefaultCaptureHeightPixels)
		return encodeScalar(value)
	case KeyAppearanceTheme:
		return encodeScalar(normalizeMember(decodeString(raw, DefaultTheme), AllowedThemes, DefaultTheme))
	case KeyAppearanceLanguage, KeyLLMOutputLanguage:
		return encodeScalar(normalizeLanguage(decodeString(raw, "")))
	case KeyNotificationsReminderTime:
		return encodeScalar(normalizeClockTime(decodeString(raw, DefaultReminderTime)))
	default:
		return raw
	}
}

func encodeScalar(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// decodeString reads a JSON string, tolerating a value written without quotes.
func decodeString(raw, fallback string) string {
	if raw == "" {
		return fallback
	}
	var value string
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		// Not JSON: accept the raw text rather than discarding a value the user
		// may have set through an older build.
		return raw
	}
	return value
}

func decodeInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	var value int
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return fallback
	}
	return value
}

func decodeInt64(raw string, fallback int64) int64 {
	if raw == "" {
		return fallback
	}
	var value int64
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return fallback
	}
	return value
}

func decodeBool(raw string, fallback bool) bool {
	if raw == "" {
		return fallback
	}
	var value bool
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return fallback
	}
	return value
}

func decodeStrings(raw string) []string {
	if raw == "" {
		return []string{}
	}
	var value []string
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return []string{}
	}
	if value == nil {
		return []string{}
	}
	return value
}

func decodeRouting(raw string) Routing {
	if raw == "" {
		return Routing{}
	}
	var value Routing
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return Routing{}
	}
	return value
}
