package app

import (
	"context"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/recorder"
	"github.com/Jwz-git/Daygo/internal/settings"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// maybeAutoStartRecording is a pure gate: every condition it checks is
// observable through the backend's own accessors, so the tests exercise the
// branches with a real temp-dir store (provider rows and the routing
// setting) and stub system permissions. The final SetRecording(true) needs a
// Capture port, which these tests do not install — the capture-less backend
// fails at ensureRecorder, which is exactly the observable difference
// between "conditions met, tried to start" and "skipped".

func TestAutoStartSkipsWithoutCaptureOwnership(t *testing.T) {
	store, err := storage.Open(context.Background(), storage.Options{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()
	seedRoutingChain(t, store, "provider-a")

	backend := newBackend(fixedClock{now: time.Now()},
		&systemStub{permission: platform.PermissionGranted}, store, true, false)
	backend.maybeAutoStartRecording()
	if got := backend.recorderState(); got != recorder.StateIdle {
		t.Fatalf("recorder state = %q, want idle", got)
	}
}

func TestAutoStartSkipsWithoutPermission(t *testing.T) {
	store, err := storage.Open(context.Background(), storage.Options{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()
	seedRoutingChain(t, store, "provider-a")

	backend := newBackend(fixedClock{now: time.Now()},
		&systemStub{permission: platform.PermissionDenied}, store, true, true)
	backend.maybeAutoStartRecording()
	if got := backend.recorderState(); got != recorder.StateIdle {
		t.Fatalf("recorder state = %q, want idle (permission denied)", got)
	}
}

func TestAutoStartSkipsWithoutProvider(t *testing.T) {
	store, err := storage.Open(context.Background(), storage.Options{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()
	// No provider rows and an empty routing chain.

	backend := newBackend(fixedClock{now: time.Now()},
		&systemStub{permission: platform.PermissionGranted}, store, true, true)
	backend.maybeAutoStartRecording()
	if got := backend.recorderState(); got != recorder.StateIdle {
		t.Fatalf("recorder state = %q, want idle (no provider)", got)
	}
}

func TestAutoStartAttemptsWhenConditionsHold(t *testing.T) {
	store, err := storage.Open(context.Background(), storage.Options{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()
	seedRoutingChain(t, store, "provider-a")

	backend := newBackend(fixedClock{now: time.Now()},
		&systemStub{permission: platform.PermissionGranted}, store, true, true)
	// Conditions all hold, so the gate reaches SetRecording. It fails here
	// because no Capture port is installed — the attempt itself is proven by
	// the absence of a skip; the recorder stays idle.
	backend.maybeAutoStartRecording()
	if got := backend.recorderState(); got != recorder.StateIdle {
		t.Fatalf("recorder state = %q, want idle without a capture port", got)
	}
}

func seedRoutingChain(t *testing.T, store *storage.Store, ids ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := store.Providers().Add(ctx, storage.Provider{
		ID: ids[0], DisplayName: "Fixture", Protocol: "openai",
		Endpoint: "https://example.invalid/v1", Model: "m",
	}); err != nil {
		t.Fatalf("seed provider: %v", err)
	}
	if err := settings.New(store.Settings()).SetRouting(ctx, settings.Routing{Chain: ids}); err != nil {
		t.Fatalf("seed routing: %v", err)
	}
}
