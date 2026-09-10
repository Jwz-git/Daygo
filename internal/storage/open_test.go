package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesDirectoryAndDefaultsToReadWrite(t *testing.T) {
	dir := filepath.Join(newDir(t), "nested", "Daygo")

	store := openWriter(t, dir)

	if _, err := os.Stat(filepath.Join(dir, DatabaseFileName)); err != nil {
		t.Fatalf("database file not created: %v", err)
	}
	inst := store.Instance()
	if inst.Mode != ModeReadWrite {
		t.Errorf("Instance().Mode = %q, want %q", inst.Mode, ModeReadWrite)
	}
	if inst.CaptureOwner {
		t.Error("Instance().CaptureOwner = true, but capture was not requested")
	}
}

func TestOpenRequiresDir(t *testing.T) {
	if _, err := Open(context.Background(), Options{}); err == nil {
		t.Fatal("Open with empty Dir returned nil error")
	}
}

// IT-13: with two instances started, exactly one holds the write lock and the
// other degrades to read-only. docs/08 §8.6.1.
func TestOpenSecondInstanceDegradesToReadOnly(t *testing.T) {
	dir := newDir(t)
	writer := openWriter(t, dir)
	reader := openReader(t, dir)

	if writer.Mode() != ModeReadWrite || reader.Mode() != ModeReadOnly {
		t.Fatalf("modes = %q/%q, want %q/%q",
			writer.Mode(), reader.Mode(), ModeReadWrite, ModeReadOnly)
	}
}

func TestOpenCaptureOwnerIsExclusive(t *testing.T) {
	dir := newDir(t)

	first, err := Open(context.Background(), Options{Dir: dir, CaptureOwnerRequested: true})
	if err != nil {
		t.Fatalf("Open first: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })
	if !first.Instance().CaptureOwner {
		t.Fatal("first instance did not become capture owner")
	}

	// A second instance cannot write the database, but it must still be able to
	// observe that the capture-owner lock is taken. Losing the lock is not an
	// error: the instance simply must not drive capture.
	second, err := Open(context.Background(), Options{Dir: dir, CaptureOwnerRequested: true})
	if err != nil {
		t.Fatalf("Open second: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })
	if second.Instance().CaptureOwner {
		t.Fatal("second instance also became capture owner; the lock is not exclusive")
	}
}

func TestCloseReleasesLocks(t *testing.T) {
	dir := newDir(t)

	first := openWriter(t, dir)
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// With the first instance closed, a new instance must be able to become the
	// writer. A lock that survived Close would deadlock the next launch.
	second := openWriter(t, dir)
	if second.Mode() != ModeReadWrite {
		t.Fatalf("Mode() = %q after reopening, want %q", second.Mode(), ModeReadWrite)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	store := openWriter(t, newDir(t))

	if err := store.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestNilStoreIsSafe(t *testing.T) {
	var store *Store
	if store.Mode() != "" {
		t.Error("nil Mode() should be empty")
	}
	if store.Path() != "" {
		t.Error("nil Path() should be empty")
	}
	if store.Instance() != (Instance{}) {
		t.Error("nil Instance() should be zero")
	}
	if err := store.Close(); err != nil {
		t.Errorf("nil Close() = %v, want nil", err)
	}
}

// A failed lock attempt must leave nothing behind, so this process can take the
// lock once the holder releases it.
func TestFailedLockAttemptDoesNotBlockLaterAcquisition(t *testing.T) {
	dir := newDir(t)
	holder := openWriter(t, dir)

	// While the holder lives, a second instance degrades instead of failing.
	second, err := Open(context.Background(), Options{Dir: dir})
	if err != nil {
		t.Fatalf("Open while held: %v", err)
	}
	if second.Mode() != ModeReadOnly {
		t.Fatalf("Mode() = %q while held, want %q", second.Mode(), ModeReadOnly)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("Close second: %v", err)
	}

	// Once the holder releases, the lock must be available again. A failed
	// attempt that left a descriptor behind would make this hang or fail.
	if err := holder.Close(); err != nil {
		t.Fatalf("Close holder: %v", err)
	}
	reopened := openWriter(t, dir)
	if reopened.Mode() != ModeReadWrite {
		t.Fatalf("Mode() = %q, want %q", reopened.Mode(), ModeReadWrite)
	}
}

// The capture lock is a separate file from the write lock. Holding the write
// lock is not the same as owning capture, which docs/05 §5.6.2 rule 7 keeps
// distinct.
func TestCaptureLockIsSeparateFromWriteLock(t *testing.T) {
	dir := newDir(t)

	store, err := Open(context.Background(), Options{Dir: dir, CaptureOwnerRequested: true})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	for _, name := range []string{writeLockName, ownerLockName} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("lock file %s missing: %v", name, err)
		}
	}
}

func TestOpenReportsErrorsFromUnwritableDirectory(t *testing.T) {
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
		t.Skip("directory unexpectedly writable; permission semantics differ on this filesystem")
	}
	// A permission problem is an environment failure, never corruption. This is
	// the distinction DB-7 asserts: the file must not be treated as damaged.
	if IsCorrupt(err) {
		t.Fatalf("permission failure classified as corruption: %v", err)
	}
}

func TestStoreKeepsDatabasePathInsideDir(t *testing.T) {
	dir := newDir(t)
	store := openWriter(t, dir)

	want := filepath.Join(dir, DatabaseFileName)
	if store.Path() != want {
		t.Fatalf("Path() = %q, want %q", store.Path(), want)
	}
}
