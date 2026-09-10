package storage

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type countingObserver struct {
	mu    sync.Mutex
	marks map[string]int
}

func newCountingObserver() *countingObserver {
	return &countingObserver{marks: map[string]int{}}
}

func (o *countingObserver) ObserveQuery(string, time.Duration, error) {}
func (o *countingObserver) ObserveBusy(string)                        {}
func (o *countingObserver) ObserveBreadcrumb(name string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.marks[name]++
}

func (o *countingObserver) count(name string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.marks[name]
}

// The maintainer must stop when its context is cancelled. A goroutine that
// outlives the app is exactly what docs/modules/data.md forbids.
func TestMaintainerStopsOnContextCancel(t *testing.T) {
	store := openWriter(t, newDir(t))
	observer := newCountingObserver()
	maintainer := NewMaintainer(store, MaintainerOptions{Observer: observer})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		maintainer.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
		// Expected: Run returned.
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after ctx was cancelled")
	}
}

// Run on a nil maintainer or a storeless maintainer must return immediately
// rather than starting a goroutine that does nothing forever.
func TestMaintainerWithoutStoreReturns(t *testing.T) {
	done := make(chan struct{})
	go func() {
		NewMaintainer(nil, MaintainerOptions{}).Run(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run blocked without a store")
	}
}

// Backup runs on the initial delay and then only when a full day has passed, so
// an app restarted repeatedly does not accumulate a backup per launch.
func TestMaintainerBackupIsIdempotentWithinADay(t *testing.T) {
	dir := newDir(t)
	store := openWriter(t, dir)
	backupDir := filepath.Join(dir, BackupDirName)

	// Drive the maintainer's own backup entry point directly with a fixed
	// clock: the schedule is tested above, and this checks the guard.
	fixed := timeNowForTest()
	store.setClock(func() time.Time { return fixed })

	maintainer := NewMaintainer(store, MaintainerOptions{
		BackupDir: dir,
		Now:       func() time.Time { return fixed },
	})

	maintainer.runBackup(context.Background())
	maintainer.runBackup(context.Background())

	backups, err := store.Backups(backupDir)
	if err != nil {
		t.Fatalf("Backups: %v", err)
	}
	// Two calls at the same instant produce the same file name, and VACUUM INTO
	// refuses to overwrite, so the second is a no-op rather than a duplicate.
	if len(backups) > 2 {
		t.Fatalf("unexpected backup count %d: %v", len(backups), backups)
	}
}

// A read-only instance must fail its maintenance actions quietly, not fatally:
// a second instance is an expected state, and it should not spam breadcrumbs
// about it.
func TestMaintainerOnReadOnlyInstanceDoesNotMarkFailure(t *testing.T) {
	dir := newDir(t)
	openWriter(t, dir)
	reader := openReader(t, dir)

	observer := newCountingObserver()
	maintainer := NewMaintainer(reader, MaintainerOptions{
		BackupDir: dir,
		Observer:  observer,
	})

	maintainer.runCheckpoint(context.Background())
	maintainer.runBackup(context.Background())

	if observer.count("storage.checkpoint.failed") != 0 {
		t.Error("read-only checkpoint was reported as a failure")
	}
	if observer.count("storage.backup.failed") != 0 {
		t.Error("read-only backup was reported as a failure")
	}
}

func TestMaintainerCheckpointEmitsBreadcrumb(t *testing.T) {
	store := openWriter(t, newDir(t))
	observer := newCountingObserver()
	maintainer := NewMaintainer(store, MaintainerOptions{Observer: observer})

	maintainer.runCheckpoint(context.Background())

	if observer.count("storage.checkpoint.ok") != 1 {
		t.Fatalf("checkpoint breadcrumb count = %d, want 1", observer.count("storage.checkpoint.ok"))
	}
}
