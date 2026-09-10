package app

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// recordingEmitter captures events so a test can assert what was published.
type recordingEmitter struct {
	events []recordedEvent
}

type recordedEvent struct {
	name    EventName
	payload any
}

func (e *recordingEmitter) Emit(name EventName, payload any) {
	e.events = append(e.events, recordedEvent{name: name, payload: payload})
}

func (e *recordingEmitter) count(name EventName) int {
	total := 0
	for _, event := range e.events {
		if event.name == name {
			total++
		}
	}
	return total
}

// backendWithStore builds a backend over a real database with a recording
// emitter, which is the configuration every write method runs in.
func backendWithStore(t *testing.T) (*Backend, *recordingEmitter) {
	t.Helper()
	store := openTestStore(t, t.TempDir(), false)
	backend := newBackend(fixedClock{}, nil, store, false, false)
	emitter := &recordingEmitter{}
	backend.SetEventEmitter(emitter)
	return backend, emitter
}

// GetSettings on an empty database returns every documented default, so the
// settings screen has real values before the user changes anything.
func TestGetSettingsReturnsDefaults(t *testing.T) {
	backend, _ := backendWithStore(t)

	dto, err := backend.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if dto.Capture.IntervalSeconds != 10 {
		t.Errorf("intervalSeconds = %d, want 10", dto.Capture.IntervalSeconds)
	}
	if dto.Capture.CaptureHeight != 1080 {
		t.Errorf("captureHeight = %d, want 1080", dto.Capture.CaptureHeight)
	}
	if dto.Appearance.Theme != "system" {
		t.Errorf("theme = %q, want %q", dto.Appearance.Theme, "system")
	}
	if dto.Appearance.Language != "zh-CN" {
		t.Errorf("language = %q, want %q", dto.Appearance.Language, "zh-CN")
	}
	if !dto.System.ShowDockIcon {
		t.Error("showDockIcon = false, want the default true")
	}
	if dto.Privacy.BlockedApplicationIDs == nil {
		t.Error("blockedApplicationIds is nil; the contract is an array")
	}
}

// Without a database, settings cannot be served. Returning defaults would show
// the user values that vanish on restart.
func TestGetSettingsWithoutStoreFails(t *testing.T) {
	backend := newBackend(fixedClock{}, nil, nil, false, false)

	_, err := backend.GetSettings()
	if err == nil {
		t.Fatal("GetSettings succeeded without a database")
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != apperr.DatabaseError {
		t.Fatalf("error = %v, want a database_error", err)
	}
}

// UpdateSettings returns the effective state, which may differ from what was
// sent because normalization clamps it.
func TestUpdateSettingsReturnsEffectiveValue(t *testing.T) {
	backend, _ := backendWithStore(t)

	dto, err := backend.UpdateSettings(SettingsPatchDTO{IntervalSeconds: ptrInt(999)})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if dto.Capture.IntervalSeconds != 10 {
		t.Fatalf("intervalSeconds = %d, want the clamped default 10", dto.Capture.IntervalSeconds)
	}
}

// A patch that omits a key must not change it. This is the property pointer
// fields exist for.
func TestUpdateSettingsLeavesOmittedKeysAlone(t *testing.T) {
	backend, _ := backendWithStore(t)

	if _, err := backend.UpdateSettings(SettingsPatchDTO{IntervalSeconds: ptrInt(30), Theme: ptrString("dark")}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	dto, err := backend.UpdateSettings(SettingsPatchDTO{Theme: ptrString("light")})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if dto.Appearance.Theme != "light" {
		t.Fatalf("theme = %q, want %q", dto.Appearance.Theme, "light")
	}
	if dto.Capture.IntervalSeconds != 30 {
		t.Fatalf("intervalSeconds = %d, want the untouched 30", dto.Capture.IntervalSeconds)
	}
}

// settings:changed carries only the keys the patch actually wrote, so a
// listener can tell what to re-render.
func TestUpdateSettingsEmitsChangedKeys(t *testing.T) {
	backend, emitter := backendWithStore(t)

	if _, err := backend.UpdateSettings(SettingsPatchDTO{Theme: ptrString("dark"), Language: ptrString("en")}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	if emitter.count(EventSettingsChanged) != 1 {
		t.Fatalf("settings:changed emitted %d times, want 1", emitter.count(EventSettingsChanged))
	}
	payload, ok := emitter.events[0].payload.(SettingsChangedPayload)
	if !ok {
		t.Fatalf("payload type = %T, want SettingsChangedPayload", emitter.events[0].payload)
	}
	if len(payload.Keys) != 2 {
		t.Fatalf("keys = %v, want two entries", payload.Keys)
	}
	want := map[string]bool{"appearance.theme": true, "appearance.language": true}
	for _, key := range payload.Keys {
		if !want[key] {
			t.Errorf("unexpected key %q in the event payload", key)
		}
	}
}

// An empty patch changes nothing, so it must not emit: a listener would
// otherwise re-read settings for no reason.
func TestUpdateSettingsEmptyPatchEmitsNothing(t *testing.T) {
	backend, emitter := backendWithStore(t)

	if _, err := backend.UpdateSettings(SettingsPatchDTO{}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if emitter.count(EventSettingsChanged) != 0 {
		t.Fatalf("empty patch emitted %d events, want 0", emitter.count(EventSettingsChanged))
	}
}

// A failed write must not announce a change. An event would send listeners to
// re-read a value that never moved.
func TestUpdateSettingsFailureEmitsNothing(t *testing.T) {
	dir := t.TempDir()
	openTestStore(t, dir, false)
	reader := openTestStore(t, dir, false) // second instance: read-only

	backend := newBackend(fixedClock{}, nil, reader, false, false)
	emitter := &recordingEmitter{}
	backend.SetEventEmitter(emitter)

	if _, err := backend.UpdateSettings(SettingsPatchDTO{Theme: ptrString("dark")}); err == nil {
		t.Fatal("UpdateSettings succeeded on a read-only instance")
	}
	if emitter.count(EventSettingsChanged) != 0 {
		t.Fatal("a failed write emitted settings:changed")
	}
}

// Settings survive a close and reopen, which is the property that lets the
// frontend stop writing them to localStorage.
func TestSettingsPersistAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	store := openTestStore(t, dir, false)
	backend := newBackend(fixedClock{}, nil, store, false, false)

	if _, err := backend.UpdateSettings(SettingsPatchDTO{
		Theme:           ptrString("dark"),
		Language:        ptrString("en"),
		IntervalSeconds: ptrInt(60),
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened := openTestStore(t, dir, false)
	restarted := newBackend(fixedClock{}, nil, reopened, false, false)

	dto, err := restarted.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings after restart: %v", err)
	}
	if dto.Appearance.Theme != "dark" {
		t.Errorf("theme = %q after restart, want %q", dto.Appearance.Theme, "dark")
	}
	if dto.Appearance.Language != "en" {
		t.Errorf("language = %q after restart, want %q", dto.Appearance.Language, "en")
	}
	if dto.Capture.IntervalSeconds != 60 {
		t.Errorf("intervalSeconds = %d after restart, want 60", dto.Capture.IntervalSeconds)
	}
}

// The wire names are the contract the frontend reads; a rename here would break
// it silently, so the JSON shape is asserted explicitly.
func TestSettingsDTOJSONShape(t *testing.T) {
	backend, _ := backendWithStore(t)

	dto, err := backend.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	encoded, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, group := range []string{
		"capture", "privacy", "storage", "notifications",
		"appearance", "llm", "system", "telemetry",
	} {
		if _, ok := decoded[group]; !ok {
			t.Errorf("group %q missing from the payload", group)
		}
	}

	capture, _ := decoded["capture"].(map[string]any)
	if _, ok := capture["intervalSeconds"]; !ok {
		t.Error("capture.intervalSeconds missing")
	}
	if _, ok := capture["captureHeight"]; !ok {
		t.Error("capture.captureHeight missing")
	}
	// The internal snapshot field is captureHeightPixels; the wire name is
	// captureHeight, per docs/05 §5.5.2.
	if _, ok := capture["captureHeightPixels"]; ok {
		t.Error("internal field name leaked onto the wire")
	}
}

// The map is the single place storage errors become application codes, so a
// read failure must arrive as a database_error rather than a bare Go error.
func TestSettingsReadFailureMapsToDatabaseError(t *testing.T) {
	backend := newBackend(fixedClock{}, nil, nil, false, false)
	backend.setStorageError(&storage.Error{Kind: storage.KindCorrupt, Op: "open"})

	_, err := backend.GetSettings()
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error %v is not an apperr", err)
	}
	if appErr.Code != apperr.DatabaseError {
		t.Fatalf("code = %q, want %q", appErr.Code, apperr.DatabaseError)
	}
}

// A backend with no emitter must not panic when a write produces an event.
func TestWriteWithoutEmitterIsSafe(t *testing.T) {
	store := openTestStore(t, t.TempDir(), false)
	backend := newBackend(fixedClock{}, nil, store, false, false)
	backend.emitter = nil

	if _, err := backend.UpdateSettings(SettingsPatchDTO{Theme: ptrString("dark")}); err != nil {
		t.Fatalf("UpdateSettings without an emitter: %v", err)
	}
}

// The emitter is a no-op until Wails supplies a context, so an event emitted
// during early startup is dropped rather than crashing.
//
// It deliberately does not install a context here. Wails treats a context it
// did not hand out as a programming error and terminates the process, so
// exercising that path would kill the test binary. The property under test is
// that Emit before SetContext is safe.
func TestWailsEmitterDropsEventsBeforeContext(t *testing.T) {
	emitter := NewWailsEmitter()
	emitter.Emit(EventSettingsChanged, SettingsChangedPayload{Keys: []string{"x"}})
	emitter.Emit(EventSettingsChanged, SettingsChangedPayload{Keys: []string{"x"}})
}

func ptrInt(value int) *int      { return &value }
func ptrString(v string) *string { return &v }
