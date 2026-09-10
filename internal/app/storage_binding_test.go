package app

import (
	"context"
	"testing"

	"github.com/Jwz-git/Daygo/internal/storage"
)

// openTestStore opens a real database in a temporary directory. Using the real
// store rather than a stub is the point: the ownership the backend reports must
// come from the locks that actually exist.
func openTestStore(t *testing.T, dir string, wantCaptureOwner bool) *storage.Store {
	t.Helper()
	store, err := storage.Open(context.Background(), storage.Options{
		Dir:                   dir,
		CaptureOwnerRequested: wantCaptureOwner,
	})
	if err != nil {
		t.Fatalf("storage.Open(%s): %v", dir, err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// A read-only second instance must report canWrite false. Before this wiring
// the value was a constructor argument hardcoded to true, so the UI believed it
// could write while the connection layer would have refused.
func TestCapabilitiesReflectRealWriteLock(t *testing.T) {
	dir := t.TempDir()

	writer := openTestStore(t, dir, false)
	reader := openTestStore(t, dir, false)

	// The fallback is deliberately set to the WRONG values for the reader. If
	// the backend ever returned the fallback instead of consulting the locks,
	// these assertions would pass while the real ownership was the opposite.
	writerBackend := newBackend(fixedClock{}, nil, writer, false, false)
	readerBackend := newBackend(fixedClock{}, nil, reader, true, true)

	writerCaps, err := writerBackend.GetCapabilities()
	if err != nil {
		t.Fatalf("writer GetCapabilities: %v", err)
	}
	if !writerCaps.CanWrite {
		t.Fatal("writer instance reports canWrite false; the fallback leaked through")
	}

	readerCaps, err := readerBackend.GetCapabilities()
	if err != nil {
		t.Fatalf("reader GetCapabilities: %v", err)
	}
	if readerCaps.CanWrite {
		t.Fatal("read-only instance reports canWrite true; ownership is not read from the lock")
	}
}

// Capture ownership is a second, independent lock. A process can hold the write
// lock and still not own capture.
func TestCapabilitiesReflectCaptureOwnerLock(t *testing.T) {
	dir := t.TempDir()

	owner := openTestStore(t, dir, true)
	nonOwner := openTestStore(t, dir, true)

	ownerCaps, err := newBackend(fixedClock{}, nil, owner, false, false).GetCapabilities()
	if err != nil {
		t.Fatalf("owner GetCapabilities: %v", err)
	}
	if !ownerCaps.IsCaptureOwner {
		t.Error("instance holding the capture lock reports isCaptureOwner false")
	}

	nonOwnerCaps, err := newBackend(fixedClock{}, nil, nonOwner, false, false).GetCapabilities()
	if err != nil {
		t.Fatalf("non-owner GetCapabilities: %v", err)
	}
	if nonOwnerCaps.IsCaptureOwner {
		t.Fatal("instance without the capture lock reports isCaptureOwner true")
	}
}

// GetRecordingState must agree with GetCapabilities: both derive ownership from
// one place, so a disagreement would mean a second source of truth crept in.
func TestRecordingStateAgreesWithCapabilities(t *testing.T) {
	dir := t.TempDir()
	openTestStore(t, dir, true)
	reader := openTestStore(t, dir, false)

	backend := newBackend(fixedClock{}, &systemStub{
		permission: "granted",
	}, reader, true, true)

	caps, err := backend.GetCapabilities()
	if err != nil {
		t.Fatalf("GetCapabilities: %v", err)
	}
	state, err := backend.GetRecordingState()
	if err != nil {
		t.Fatalf("GetRecordingState: %v", err)
	}
	if state.IsCaptureOwner != caps.IsCaptureOwner {
		t.Fatalf("recording state reports isCaptureOwner=%v but capabilities report %v",
			state.IsCaptureOwner, caps.IsCaptureOwner)
	}
}

// Without a database the app must not claim persistence. Advertising "storage"
// would make the frontend render settings it cannot save.
func TestCapabilitiesOmitStorageWithoutStore(t *testing.T) {
	backend := newBackend(fixedClock{}, nil, nil, false, false)

	caps, err := backend.GetCapabilities()
	if err != nil {
		t.Fatalf("GetCapabilities: %v", err)
	}
	for _, feature := range caps.Features {
		if feature == "storage" {
			t.Fatal(`"storage" advertised without an open store`)
		}
	}
	if caps.CanWrite {
		t.Fatal("canWrite true without an open store")
	}
}

func TestCapabilitiesAdvertiseStorageWithStore(t *testing.T) {
	store := openTestStore(t, t.TempDir(), false)
	backend := newBackend(fixedClock{}, nil, store, false, false)

	caps, err := backend.GetCapabilities()
	if err != nil {
		t.Fatalf("GetCapabilities: %v", err)
	}
	found := false
	for _, feature := range caps.Features {
		if feature == "storage" {
			found = true
		}
	}
	if !found {
		t.Fatalf("open store not advertised; features = %v", caps.Features)
	}
}

func TestStorageFailureIsRecordedNotThrown(t *testing.T) {
	backend := newBackend(fixedClock{}, nil, nil, false, false)
	if backend.storageFailure() != nil {
		t.Fatal("fresh backend already reports a storage failure")
	}

	cause := context.DeadlineExceeded
	backend.setStorageError(cause)
	if backend.storageFailure() != cause {
		t.Fatal("recorded storage failure did not round trip")
	}
}
