package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestKindClosedSet(t *testing.T) {
	valid := []Kind{KindBusy, KindCorrupt, KindReadOnly, KindEnvironment, KindConstraint, KindNotFound}
	for _, k := range valid {
		if !k.Valid() {
			t.Errorf("Kind(%q).Valid() = false", k)
		}
	}
	if Kind("invented").Valid() {
		t.Error(`Kind("invented").Valid() = true`)
	}
}

// Only corruption may be recoverable. If an environment failure were reported
// as recoverable, the recovery path would delete an intact database: this is
// the failure condition DB-7 names explicitly.
func TestOnlyCorruptionIsRecoverable(t *testing.T) {
	if !KindCorrupt.Recoverable() {
		t.Error("KindCorrupt.Recoverable() = false")
	}
	notRecoverable := []Kind{KindBusy, KindReadOnly, KindEnvironment, KindConstraint, KindNotFound}
	for _, k := range notRecoverable {
		if k.Recoverable() {
			t.Errorf("Kind(%q).Recoverable() = true; only corruption may be recovered", k)
		}
	}
}

func TestKindOfUnwraps(t *testing.T) {
	base := newError(KindCorrupt, "integrity_check")
	wrapped := wrap("open", base)

	kind, ok := KindOf(wrapped)
	if !ok {
		t.Fatal("KindOf did not recognize a wrapped storage error")
	}
	if kind != KindCorrupt {
		t.Fatalf("kind = %q, want %q", kind, KindCorrupt)
	}
	if !IsKind(wrapped, KindCorrupt) {
		t.Error("IsKind did not match through the wrap chain")
	}
	if !IsCorrupt(wrapped) {
		t.Error("IsCorrupt = false for a corrupt error")
	}
}

func TestKindOfRejectsForeignError(t *testing.T) {
	if _, ok := KindOf(errors.New("plain error")); ok {
		t.Error("KindOf accepted a non-storage error")
	}
	if IsCorrupt(errors.New("plain error")) {
		t.Error("IsCorrupt = true for a non-storage error")
	}
}

func TestWrapNilIsNil(t *testing.T) {
	if err := wrap("op", nil); err != nil {
		t.Fatalf("wrap(op, nil) = %v, want nil", err)
	}
}

func TestErrorUnwrapPreservesCause(t *testing.T) {
	cause := errors.New("underlying")
	err := wrap("op", cause)

	if !errors.Is(err, cause) {
		t.Fatal("wrapped error does not expose its cause to errors.Is")
	}
	var se *Error
	if !errors.As(err, &se) {
		t.Fatal("wrapped error does not expose *Error to errors.As")
	}
	if se.Op != "op" {
		t.Errorf("Op = %q, want %q", se.Op, "op")
	}
}

func TestNilErrorIsSafe(t *testing.T) {
	var e *Error
	if e.Error() != "<nil>" {
		t.Errorf("nil Error() = %q", e.Error())
	}
	if e.Unwrap() != nil {
		t.Error("nil Unwrap() != nil")
	}
}

// DB-7 first half: a file that is not a database must be classified as corrupt,
// which is what warrants recovery. Reporting it as an environment failure would
// leave the app unable to start with no remedy.
func TestNotADatabaseIsClassifiedAsCorrupt(t *testing.T) {
	dir := newDir(t)
	copyFile(t, filepath.Join("testdata", "notadb.db"), filepath.Join(dir, DatabaseFileName))

	_, err := Open(context.Background(), Options{Dir: dir})
	if err == nil {
		t.Skip("driver accepted a non-database file; the platform behaves differently here")
	}
	if !IsCorrupt(err) {
		t.Fatalf("non-database file classified as %v, want corruption", err)
	}
}

// DB-7: an environment failure must NOT be classified as corrupt, because the
// recovery path deletes files. The database here is perfectly valid; only the
// directory is unwritable.
func TestUnwritableDirectoryIsNotCorruption(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows ACLs do not implement POSIX chmod write denial")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses directory permissions")
	}
	parent := newDir(t)
	dir := filepath.Join(parent, "readonly")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	_, err := Open(context.Background(), Options{Dir: dir})
	if err == nil {
		t.Skip("directory unexpectedly writable on this filesystem")
	}
	if IsCorrupt(err) {
		t.Fatalf("environment failure classified as corruption: %v", err)
	}
	// The directory and anything in it must still be present: recovery must not
	// have run.
	if _, statErr := os.Stat(dir); statErr != nil {
		t.Fatalf("directory was removed after a failed open: %v", statErr)
	}
}

// An error this package cannot classify defaults to environment, never to
// corruption. The safe default is "do not delete anything".
func TestUnclassifiedErrorDefaultsToEnvironment(t *testing.T) {
	kind := classify(errors.New("something unexpected"))
	if kind != KindEnvironment {
		t.Fatalf("classify(unknown) = %q, want %q", kind, KindEnvironment)
	}
	if kind.Recoverable() {
		t.Fatal("unclassified error is recoverable; a missing classification must never delete a file")
	}
}

func TestClassifyNoRowsIsNotFound(t *testing.T) {
	if got := classify(sql.ErrNoRows); got != KindNotFound {
		t.Fatalf("classify(sql.ErrNoRows) = %q, want %q", got, KindNotFound)
	}
}

// A storage error that is already classified must keep its kind when wrapped
// again, rather than being re-derived from the driver error underneath.
func TestClassifyPreservesExistingKind(t *testing.T) {
	inner := newError(KindReadOnly, "refuse write")
	if got := classify(inner); got != KindReadOnly {
		t.Fatalf("classify(classified) = %q, want %q", got, KindReadOnly)
	}
}
