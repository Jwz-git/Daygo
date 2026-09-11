package app

import (
	"errors"
	"testing"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// Diagnostics must work without a database: it is the method that explains why
// things are degraded, so it cannot itself depend on everything working.
func TestDiagnosticsWithoutStore(t *testing.T) {
	backend := newBackend(fixedClock{}, nil, nil, false, false)

	dto, err := backend.GetDiagnostics()
	if err != nil {
		t.Fatalf("GetDiagnostics without a store: %v", err)
	}
	if dto.DBStatus != DBStatusUnavailable {
		t.Fatalf("DBStatus = %q, want %q", dto.DBStatus, DBStatusUnavailable)
	}
	if dto.Unavailable["database"] == "" {
		t.Fatal("no reason reported for an unavailable database")
	}
}

func TestDiagnosticsReportsOpenFailureReason(t *testing.T) {
	backend := newBackend(fixedClock{}, nil, nil, false, false)
	backend.setStorageError(&storage.Error{Kind: storage.KindCorrupt, Op: "open"})

	dto, err := backend.GetDiagnostics()
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}
	reason := dto.Unavailable["database"]
	if reason != "database file is damaged" {
		t.Fatalf("reason = %q, want the corruption explanation", reason)
	}
	// The driver's own message can quote a path, so it must not be forwarded.
	if dto.DatabasePath != "" {
		t.Fatalf("DatabasePath = %q for a database that never opened", dto.DatabasePath)
	}
}

func TestDiagnosticsWithStoreReportsSizes(t *testing.T) {
	store := openTestStore(t, t.TempDir(), true)
	backend := newBackend(fixedClock{}, nil, store, false, false)

	dto, err := backend.GetDiagnostics()
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}
	if dto.DBStatus != DBStatusOK {
		t.Fatalf("DBStatus = %q, want %q", dto.DBStatus, DBStatusOK)
	}
	if dto.DatabaseBytes <= 0 {
		t.Fatalf("DatabaseBytes = %d, want a positive size", dto.DatabaseBytes)
	}
	if dto.DatabasePath != store.Path() {
		t.Fatalf("DatabasePath = %q, want %q", dto.DatabasePath, store.Path())
	}
}

func TestDiagnosticsReportsReadOnlyInstance(t *testing.T) {
	dir := t.TempDir()
	openTestStore(t, dir, false)
	reader := openTestStore(t, dir, false)

	dto, err := newBackend(fixedClock{}, nil, reader, false, false).GetDiagnostics()
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}
	if dto.DBStatus != DBStatusReadOnly {
		t.Fatalf("DBStatus = %q, want %q", dto.DBStatus, DBStatusReadOnly)
	}
}

// Counters whose data source does not exist must say so rather than report a
// zero that reads as "nothing happened".
func TestDiagnosticsNamesUnavailableDataSources(t *testing.T) {
	store := openTestStore(t, t.TempDir(), false)

	dto, err := newBackend(fixedClock{}, nil, store, false, false).GetDiagnostics()
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}
	for _, field := range []string{"pendingBatches", "failedBatches", "lastCaptureAtTs"} {
		if dto.Unavailable[field] == "" {
			t.Errorf("%s has no data source yet but is not listed as unavailable", field)
		}
	}
	if _, ok := dto.Unavailable["recordingsBytes"]; ok {
		t.Fatal("recordingsBytes is unavailable despite the recording schema")
	}
}

func TestDiagnosticsReportsCaptureOwnerPID(t *testing.T) {
	owner := openTestStore(t, t.TempDir(), true)

	dto, err := newBackend(fixedClock{}, nil, owner, false, false).GetDiagnostics()
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}
	if dto.CaptureOwnerPID == nil {
		t.Fatal("CaptureOwnerPID is nil for the instance holding the capture lock")
	}
	if *dto.CaptureOwnerPID <= 0 {
		t.Fatalf("CaptureOwnerPID = %d", *dto.CaptureOwnerPID)
	}
}

func TestDiagnosticsOmitsCaptureOwnerPIDForNonOwner(t *testing.T) {
	dir := t.TempDir()
	openTestStore(t, dir, true)
	nonOwner := openTestStore(t, dir, true)

	dto, err := newBackend(fixedClock{}, nil, nonOwner, false, false).GetDiagnostics()
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}
	if dto.CaptureOwnerPID != nil {
		t.Fatalf("CaptureOwnerPID = %d for an instance with no capture lock", *dto.CaptureOwnerPID)
	}
}

func TestDiagnosticsNativeStateReflectsSystemAvailability(t *testing.T) {
	store := openTestStore(t, t.TempDir(), false)

	withoutSystem, err := newBackend(fixedClock{}, nil, store, false, false).GetDiagnostics()
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}
	if withoutSystem.NativeState != NativeStateUnavailable {
		t.Fatalf("NativeState = %q without a system, want %q",
			withoutSystem.NativeState, NativeStateUnavailable)
	}

	withSystem, err := newBackend(fixedClock{}, &systemStub{}, store, false, false).GetDiagnostics()
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}
	if withSystem.NativeState != NativeStateOK {
		t.Fatalf("NativeState = %q with a system, want %q", withSystem.NativeState, NativeStateOK)
	}
}

// The storage-to-apperr mapping is the single point docs/05 §5.6.1 requires.
// Each storage kind must land on a specific code rather than a generic one.
func TestStorageErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want apperr.Code
	}{
		{"busy", &storage.Error{Kind: storage.KindBusy, Op: "x"}, apperr.Conflict},
		{"corrupt", &storage.Error{Kind: storage.KindCorrupt, Op: "x"}, apperr.DatabaseError},
		{"read only", &storage.Error{Kind: storage.KindReadOnly, Op: "x"}, apperr.DatabaseError},
		{"not found", &storage.Error{Kind: storage.KindNotFound, Op: "x"}, apperr.NotFound},
		{"constraint", &storage.Error{Kind: storage.KindConstraint, Op: "x"}, apperr.InvalidArgument},
		{"environment", &storage.Error{Kind: storage.KindEnvironment, Op: "x"}, apperr.DatabaseError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mapped := mapStorageError("op", tc.err)
			var appErr *apperr.Error
			if !errors.As(mapped, &appErr) {
				t.Fatalf("mapped error is not an apperr: %v", mapped)
			}
			if appErr.Code != tc.want {
				t.Fatalf("code = %q, want %q", appErr.Code, tc.want)
			}
			// The original cause must stay in the chain for logging.
			if !errors.Is(mapped, tc.err) {
				t.Fatal("mapping dropped the original error from the chain")
			}
		})
	}
}

func TestStorageErrorMappingPassesNilThrough(t *testing.T) {
	if err := mapStorageError("op", nil); err != nil {
		t.Fatalf("mapStorageError(nil) = %v, want nil", err)
	}
}

// An error storage did not originate must not be silently reported as a
// database problem.
func TestStorageErrorMappingOfForeignError(t *testing.T) {
	mapped := mapStorageError("op", errors.New("unclassified"))
	var appErr *apperr.Error
	if !errors.As(mapped, &appErr) {
		t.Fatalf("mapped error is not an apperr: %v", mapped)
	}
	if appErr.Code != apperr.Internal {
		t.Fatalf("code = %q, want %q", appErr.Code, apperr.Internal)
	}
}
