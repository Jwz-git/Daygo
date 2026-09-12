package app

import (
	"context"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/settings"
)

// settingsTimeout bounds a settings read or write. Writes use the database
// write ceiling from docs/05 §5.6.1.
const settingsTimeout = 10 * time.Second

// settingsAccess returns the typed settings accessor, or an error when no
// database is open.
//
// Settings live in the database (docs/03 §3.1), so without one there is nothing
// to read or write. Reporting unavailable is the honest answer; inventing
// in-memory defaults would let the UI show values that would vanish on restart.
func (b *Backend) settingsAccess() (*settings.Settings, error) {
	store := b.store()
	if store == nil {
		if err := b.storageFailure(); err != nil {
			return nil, mapStorageError("open settings", err)
		}
		return nil, apperr.E(apperr.DatabaseError, "settings require a database", nil)
	}
	return settings.New(store.Settings()), nil
}

// GetSettings returns the effective settings with defaults applied.
func (b *Backend) GetSettings() (SettingsDTO, error) {
	access, err := b.settingsAccess()
	if err != nil {
		return SettingsDTO{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), settingsTimeout)
	defer cancel()

	snapshot, err := access.Load(ctx)
	if err != nil {
		return SettingsDTO{}, mapStorageError("read settings", err)
	}
	return settingsToDTO(snapshot), nil
}

// UpdateSettings applies a partial patch and returns the full effective
// settings afterwards.
//
// Only the keys present in the patch are written, and the return value is the
// post-normalization state: values may differ from what the caller sent,
// because normalization and clamping happen in internal/settings (docs/05
// §5.6.3 rule 2). This is the one write method that returns a full snapshot,
// precisely so the frontend does not have to predict the result (docs/05
// §5.3.4).
//
// The settings:changed event carries only the keys that were written, so a
// listener can tell which settings to re-render.
func (b *Backend) UpdateSettings(patch SettingsPatchDTO) (SettingsDTO, error) {
	access, err := b.settingsAccess()
	if err != nil {
		return SettingsDTO{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), settingsTimeout)
	defer cancel()

	snapshot, changed, err := access.Apply(ctx, patchFromDTO(patch))
	if err != nil {
		return SettingsDTO{}, mapStorageError("update settings", err)
	}

	// The event fires only after the write committed. Emitting before would
	// send listeners to re-read a value that may have rolled back.
	if len(changed) > 0 {
		b.emitSettingsChanged(changed)
	}
	b.recorderMu.Lock()
	activeRecorder := b.recorder
	b.recorderMu.Unlock()
	if activeRecorder != nil {
		activeRecorder.UpdateSettings(snapshot)
	}
	return settingsToDTO(snapshot), nil
}

// settingsToDTO maps the settings snapshot onto the wire shape.
func settingsToDTO(s settings.Snapshot) SettingsDTO {
	blocked := s.BlockedApplicationIDs
	if blocked == nil {
		// A nil slice marshals to null; the contract is an array.
		blocked = []string{}
	}
	return SettingsDTO{
		Capture: CaptureSettingsDTO{
			IntervalSeconds: s.CaptureIntervalSeconds,
			CaptureHeight:   s.CaptureHeightPixels,
		},
		Privacy: PrivacySettingsDTO{
			BlockedApplicationIDs: blocked,
		},
		Storage: StorageSettingsDTO{
			RecordingsLimitBytes: s.RecordingsLimitBytes,
		},
		Notifications: NotificationSettingsDTO{
			JournalReminderEnabled: s.ReminderEnabled,
			JournalReminderTime:    s.ReminderTime,
		},
		Appearance: AppearanceSettingsDTO{
			Theme:    s.Theme,
			Language: s.Language,
		},
		LLM: LLMSettingsDTO{
			OutputLanguage:                s.OutputLanguage,
			RecognitionEnhancementEnabled: s.RecognitionEnhancement,
		},
		Chat: ChatSettingsDTO{
			Memory: s.ChatMemory,
		},
		System: SystemSettingsDTO{
			LaunchAtLogin:     s.LaunchAtLogin,
			ShowDockIcon:      s.ShowDockIcon,
			AgentEditsEnabled: s.AgentEditsEnabled,
		},
		Telemetry: TelemetrySettingsDTO{
			AnalyticsOptIn:      s.AnalyticsOptIn,
			CrashReportingOptIn: s.CrashReportingOptIn,
		},
	}
}

// patchFromDTO maps the wire patch onto the settings layer's patch. The nil
// pointers carry through unchanged: dropping one would turn "leave this alone"
// into "reset to default".
func patchFromDTO(p SettingsPatchDTO) settings.Patch {
	return settings.Patch{
		IntervalSeconds:        p.IntervalSeconds,
		CaptureHeight:          p.CaptureHeight,
		BlockedApplicationIDs:  p.BlockedApplicationIDs,
		RecordingsLimitBytes:   p.RecordingsLimitBytes,
		JournalReminderEnabled: p.JournalReminderEnabled,
		JournalReminderTime:    p.JournalReminderTime,
		Theme:                  p.Theme,
		Language:               p.Language,
		OutputLanguage:         p.OutputLanguage,
		RecognitionEnhancement: p.RecognitionEnhancementEnabled,
		ChatMemory:             p.ChatMemory,
		LaunchAtLogin:          p.LaunchAtLogin,
		ShowDockIcon:           p.ShowDockIcon,
		AgentEditsEnabled:      p.AgentEditsEnabled,
		AnalyticsOptIn:         p.AnalyticsOptIn,
		CrashReportingOptIn:    p.CrashReportingOptIn,
	}
}
