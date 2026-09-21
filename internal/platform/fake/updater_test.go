package fake

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
)

func TestUpdaterDefaultsToOptInAutomatic(t *testing.T) {
	u := NewUpdater()
	state, err := u.State(context.Background())
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if state.Automatic {
		t.Fatalf("Automatic = true, want false (opt-in default)")
	}
	if state.AvailableVersion != nil {
		t.Fatalf("AvailableVersion = %v, want nil before any check", *state.AvailableVersion)
	}
	if state.LastCheckedAt != nil {
		t.Fatalf("LastCheckedAt = %v, want nil before any check", *state.LastCheckedAt)
	}
}

func TestUpdaterCheckWithoutUpdateEmitsNothing(t *testing.T) {
	u := NewUpdater()
	if err := u.CheckForUpdates(context.Background(), true); err != nil {
		t.Fatalf("CheckForUpdates: %v", err)
	}
	state, err := u.State(context.Background())
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if state.AvailableVersion != nil {
		t.Fatalf("AvailableVersion = %v, want nil when no update", *state.AvailableVersion)
	}
	if state.LastCheckedAt == nil {
		t.Fatalf("LastCheckedAt is nil after a check")
	}
	select {
	case event := <-u.Events():
		t.Fatalf("unexpected event when no update: %#v", event)
	default:
	}
}

func TestUpdaterCheckDiscoversVersionAndEmits(t *testing.T) {
	u := NewUpdater()
	u.SetNextVersion("1.2.3")
	if err := u.CheckForUpdates(context.Background(), false); err != nil {
		t.Fatalf("CheckForUpdates: %v", err)
	}
	state, err := u.State(context.Background())
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if state.AvailableVersion == nil || *state.AvailableVersion != "1.2.3" {
		t.Fatalf("AvailableVersion = %v, want 1.2.3", state.AvailableVersion)
	}
	select {
	case event := <-u.Events():
		if event.Err != nil {
			t.Fatalf("event carried error: %v", event.Err)
		}
		if event.State.AvailableVersion == nil || *event.State.AvailableVersion != "1.2.3" {
			t.Fatalf("event AvailableVersion = %v, want 1.2.3", event.State.AvailableVersion)
		}
	default:
		t.Fatalf("no event delivered after discovering an update")
	}
}

func TestUpdaterCheckError(t *testing.T) {
	u := NewUpdater()
	sentinel := errors.New("feed unreachable")
	u.SetCheckError(sentinel)
	err := u.CheckForUpdates(context.Background(), true)
	if !errors.Is(err, sentinel) {
		t.Fatalf("CheckForUpdates error = %v, want %v", err, sentinel)
	}
	select {
	case event := <-u.Events():
		t.Fatalf("failed check emitted an event: %#v", event)
	default:
	}
}

func TestUpdaterHonorsCancellation(t *testing.T) {
	u := NewUpdater()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := u.CheckForUpdates(ctx, true); !errors.Is(err, context.Canceled) {
		t.Fatalf("CheckForUpdates error = %v, want context.Canceled", err)
	}
	if _, err := u.State(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("State error = %v, want context.Canceled", err)
	}
	if err := u.SetAutomaticChecks(ctx, true); !errors.Is(err, context.Canceled) {
		t.Fatalf("SetAutomaticChecks error = %v, want context.Canceled", err)
	}
	if _, checked := u.LastInteractive(); checked {
		t.Fatalf("canceled check was recorded as run")
	}
}

func TestUpdaterInteractiveFlagRecorded(t *testing.T) {
	u := NewUpdater()
	if _, checked := u.LastInteractive(); checked {
		t.Fatalf("LastInteractive reports a check before any ran")
	}
	if err := u.CheckForUpdates(context.Background(), false); err != nil {
		t.Fatalf("CheckForUpdates: %v", err)
	}
	interactive, checked := u.LastInteractive()
	if !checked || interactive {
		t.Fatalf("LastInteractive = (%v, %v), want (false, true)", interactive, checked)
	}
	if err := u.CheckForUpdates(context.Background(), true); err != nil {
		t.Fatalf("CheckForUpdates: %v", err)
	}
	interactive, _ = u.LastInteractive()
	if !interactive {
		t.Fatalf("LastInteractive interactive = false, want true after interactive check")
	}
	if u.CheckCount() != 2 {
		t.Fatalf("CheckCount = %d, want 2", u.CheckCount())
	}
}

func TestUpdaterSetAutomaticChecks(t *testing.T) {
	u := NewUpdater()
	if err := u.SetAutomaticChecks(context.Background(), true); err != nil {
		t.Fatalf("SetAutomaticChecks: %v", err)
	}
	state, err := u.State(context.Background())
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if !state.Automatic {
		t.Fatalf("Automatic = false, want true after enabling")
	}
}

func TestUpdaterEventChannelDropsOldestWhenFull(t *testing.T) {
	u := NewUpdater()
	// Fill past the buffer without draining; the newest state must still land
	// (docs/05 §5.7.2: bounded, merge).
	for i := range 20 {
		u.SetNextVersion(versionFor(i))
		if err := u.CheckForUpdates(context.Background(), false); err != nil {
			t.Fatalf("CheckForUpdates: %v", err)
		}
	}
	var last *string
	for {
		select {
		case event := <-u.Events():
			last = event.State.AvailableVersion
			continue
		default:
		}
		break
	}
	if last == nil || *last != versionFor(19) {
		t.Fatalf("latest buffered event version = %v, want %s", last, versionFor(19))
	}
}

func versionFor(i int) string {
	return time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC).Format("15.04.05")
}

var _ platform.Updater = (*Updater)(nil)
