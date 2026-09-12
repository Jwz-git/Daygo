package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// The recording and analysis schemas ship in the migration chain, so these
// fields have real data sources. An empty schema reports zero — or no capture
// at all — which is a different fact from an unavailable source and must not be
// labelled as one.
//
// This test previously asserted the opposite, back when the tables did not
// exist and the counters could only be reported as unavailable. The expectation
// changed because the schema shipped, not because the assertion was
// inconvenient; the "table absent" branch is covered in internal/storage, where
// that state can actually be constructed.
func TestDiagnosticsReportsRealSourcesWhenSchemaPresent(t *testing.T) {
	store := openTestStore(t, t.TempDir(), false)

	dto, err := newBackend(fixedClock{}, nil, store, false, false).GetDiagnostics()
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}

	for _, field := range []string{
		"recordingsBytes", "lastCaptureAtTs", "pendingBatches", "failedBatches",
	} {
		if reason, ok := dto.Unavailable[field]; ok {
			t.Errorf("%s reported unavailable (%q) although its schema exists", field, reason)
		}
	}

	if dto.RecordingsBytes != 0 {
		t.Errorf("recordingsBytes = %d on an empty recording schema", dto.RecordingsBytes)
	}
	// No frame has been committed, so there is no last capture time. Reporting
	// zero here would claim a capture at the Unix epoch.
	if dto.LastCaptureAtTs != nil {
		t.Errorf("lastCaptureAtTs = %d with no committed frame", *dto.LastCaptureAtTs)
	}
	if dto.PendingBatches != 0 || dto.FailedBatches != 0 {
		t.Errorf("batches = %d pending / %d failed on an empty schema",
			dto.PendingBatches, dto.FailedBatches)
	}
}

// Committed frames must reach the DTO: this is the path that lets the
// diagnostics screen show real disk usage instead of a permanent zero.
func TestDiagnosticsReflectsCommittedFrames(t *testing.T) {
	store := openTestStore(t, t.TempDir(), false)
	ctx := context.Background()

	capturedAt := time.Unix(1700000000, 0)
	id, err := store.Captures().Begin(ctx, "2026/09/12/segment-0001", capturedAt, nil, 1920, 1080, false)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := store.Captures().Commit(ctx, id, 4096); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	dto, err := newBackend(fixedClock{}, nil, store, false, false).GetDiagnostics()
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}
	if dto.RecordingsBytes != 4096 {
		t.Errorf("recordingsBytes = %d, want 4096", dto.RecordingsBytes)
	}
	if dto.LastCaptureAtTs == nil {
		t.Fatal("lastCaptureAtTs is nil after a committed frame")
	}
	if *dto.LastCaptureAtTs != capturedAt.Unix() {
		t.Errorf("lastCaptureAtTs = %d, want %d", *dto.LastCaptureAtTs, capturedAt.Unix())
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

// A healthy open is not a recovery. Reporting one would make the signal
// meaningless the first time it mattered.
func TestDiagnosticsReportsNoRecoveryOnHealthyOpen(t *testing.T) {
	store := openTestStore(t, t.TempDir(), false)

	dto, err := newBackend(fixedClock{}, nil, store, false, false).GetDiagnostics()
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}
	if dto.RecoveredFromBackup != "" {
		t.Fatalf("recoveredFromBackup = %q on a healthy open", dto.RecoveredFromBackup)
	}
}

// Recovery is a loss of data relative to what the user had, so diagnostics must
// surface it. Without this the app would silently show an older timeline.
func TestDiagnosticsReportsRecoveryFromBackup(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	store := openTestStore(t, dir, false)
	if _, err := store.Backup(ctx, filepath.Join(dir, storage.BackupDirName), 7); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Truncating the database and dropping the WAL is the torn write DB-7
	// describes; it makes Open fail with a corruption classification.
	database := filepath.Join(dir, storage.DatabaseFileName)
	info, err := os.Stat(database)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if err := os.Truncate(database, info.Size()/3); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(database + suffix)
	}

	recovered := openTestStore(t, dir, false)
	if recovered.RecoveredFrom() == "" {
		t.Fatal("the store did not report a recovery")
	}

	dto, err := newBackend(fixedClock{}, nil, recovered, false, false).GetDiagnostics()
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}
	if dto.RecoveredFromBackup == "" {
		t.Fatal("diagnostics did not report the recovery")
	}
	// The name carries the timestamp of the restored data; the directory is
	// already reported separately.
	if strings.Contains(dto.RecoveredFromBackup, string(filepath.Separator)) {
		t.Fatalf("recoveredFromBackup = %q, want a bare file name", dto.RecoveredFromBackup)
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
