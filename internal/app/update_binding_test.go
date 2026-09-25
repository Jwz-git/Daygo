package app

import (
	"sync"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform/fake"
)

// syncEmitter is a concurrency-safe emitter: the updater event pump runs on its
// own goroutine, so the recordingEmitter (append without a lock) would race.
type syncEmitter struct {
	mu     sync.Mutex
	events []recordedEvent
}

type coordinatedUpdater struct {
	*fake.Updater
	canInstall func() bool
	prepare    func() error
	shutdown   func()
	cancel     func()
}

func (u *coordinatedUpdater) SetInstallCancelled(cancel func()) { u.cancel = cancel }

func (u *coordinatedUpdater) SetInstallCallbacks(canInstall func() bool, prepare func() error, shutdown func()) {
	u.canInstall = canInstall
	u.prepare = prepare
	u.shutdown = shutdown
}

func (e *syncEmitter) Emit(name EventName, payload any) {
	e.mu.Lock()
	e.events = append(e.events, recordedEvent{name: name, payload: payload})
	e.mu.Unlock()
}

func (e *syncEmitter) find(name EventName) (recordedEvent, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, event := range e.events {
		if event.name == name {
			return event, true
		}
	}
	return recordedEvent{}, false
}

func TestGetUpdaterStateNativeUnavailableWithoutAdapter(t *testing.T) {
	backend := newBackend(fixedClock{}, nil, nil, false, false)
	_, err := backend.GetUpdaterState()
	requireCode(t, err, apperr.NativeUnavailable)
}

func TestCheckForUpdatesNativeUnavailableWithoutAdapter(t *testing.T) {
	backend := newBackend(fixedClock{}, nil, nil, false, false)
	err := backend.CheckForUpdates(true)
	requireCode(t, err, apperr.NativeUnavailable)
}

func TestGetUpdaterStateReadsAdapter(t *testing.T) {
	backend := newBackend(fixedClock{}, nil, nil, false, false)
	updater := fake.NewUpdater()
	updater.SetNextVersion("2.0.0")
	backend.setUpdater(updater)

	if err := updater.SetAutomaticChecks(t.Context(), true); err != nil {
		t.Fatalf("SetAutomaticChecks: %v", err)
	}
	if err := backend.CheckForUpdates(false); err != nil {
		t.Fatalf("CheckForUpdates: %v", err)
	}
	state, err := backend.GetUpdaterState()
	if err != nil {
		t.Fatalf("GetUpdaterState: %v", err)
	}
	if !state.Automatic {
		t.Errorf("Automatic = false, want true")
	}
	if state.AvailableVersion == nil || *state.AvailableVersion != "2.0.0" {
		t.Errorf("AvailableVersion = %v, want 2.0.0", state.AvailableVersion)
	}
	if state.LastCheckedAtTs == nil {
		t.Errorf("LastCheckedAtTs is nil after a check")
	}
}

func TestUpdaterEventPumpEmitsUpdateAvailable(t *testing.T) {
	backend := newBackend(fixedClock{}, nil, nil, false, false)
	emitter := &syncEmitter{}
	backend.setEventEmitter(emitter)
	updater := fake.NewUpdater()
	backend.setUpdater(updater)
	backend.startUpdaterEventPump(t.Context())

	updater.SetNextVersion("3.1.4")
	if err := backend.CheckForUpdates(true); err != nil {
		t.Fatalf("CheckForUpdates: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		if event, ok := emitter.find(EventUpdateAvailable); ok {
			payload, ok := event.payload.(UpdaterStateDTO)
			if !ok {
				t.Fatalf("update:available payload type = %T, want UpdaterStateDTO", event.payload)
			}
			if payload.AvailableVersion == nil || *payload.AvailableVersion != "3.1.4" {
				t.Fatalf("payload AvailableVersion = %v, want 3.1.4", payload.AvailableVersion)
			}
			return
		}
		select {
		case <-deadline:
			t.Fatal("update:available was not emitted within 2s")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func TestSetAutomaticUpdateChecksWritesAdapter(t *testing.T) {
	backend := newBackend(fixedClock{}, nil, nil, false, false)
	updater := fake.NewUpdater()
	backend.setUpdater(updater)

	if err := backend.SetAutomaticUpdateChecks(true); err != nil {
		t.Fatalf("SetAutomaticUpdateChecks: %v", err)
	}
	state, err := backend.GetUpdaterState()
	if err != nil {
		t.Fatalf("GetUpdaterState: %v", err)
	}
	if !state.Automatic {
		t.Fatal("Automatic = false, want true")
	}
}

func TestSetAutomaticUpdateChecksNativeUnavailableWithoutAdapter(t *testing.T) {
	backend := newBackend(fixedClock{}, nil, nil, false, false)
	requireCode(t, backend.SetAutomaticUpdateChecks(true), apperr.NativeUnavailable)
}

func TestUpdateInstallRequiresBothInstanceLocks(t *testing.T) {
	for _, tc := range []struct {
		name         string
		canWrite     bool
		captureOwner bool
		want         bool
	}{
		{name: "both", canWrite: true, captureOwner: true, want: true},
		{name: "read only", canWrite: false, captureOwner: true, want: false},
		{name: "not capture owner", canWrite: true, captureOwner: false, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			updater := &coordinatedUpdater{Updater: fake.NewUpdater()}
			backend := newBackend(fixedClock{}, nil, nil, tc.canWrite, tc.captureOwner)
			backend.setUpdater(updater)
			backend.configureUpdateInstall(func() {})
			if updater.canInstall == nil || updater.canInstall() != tc.want {
				t.Fatalf("canInstall = %v, want %v", updater.canInstall != nil && updater.canInstall(), tc.want)
			}
			if updater.prepare == nil || updater.prepare() != nil {
				t.Fatal("idle backend prepare must succeed")
			}
		})
	}
}

func TestUpdateCancellationReleasesRecordingStartGate(t *testing.T) {
	updater := &coordinatedUpdater{Updater: fake.NewUpdater()}
	b := newBackend(fixedClock{}, nil, nil, true, true)
	b.setUpdater(updater)
	b.configureUpdateInstall(func() {})
	if err := updater.prepare(); err != nil {
		t.Fatal(err)
	}
	if !b.updatePrepared.Load() {
		t.Fatal("preparation must gate recording starts")
	}
	if updater.cancel == nil {
		t.Fatal("cancellation callback is required")
	}
	updater.cancel()
	if b.updatePrepared.Load() {
		t.Fatal("cancelled update must release recording start gate")
	}
}

func requireCode(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want code %q", code)
	}
	appErr, ok := err.(*apperr.Error)
	if !ok {
		t.Fatalf("error = %T, want *apperr.Error", err)
	}
	if appErr.Code != code {
		t.Fatalf("error code = %q, want %q", appErr.Code, code)
	}
}
