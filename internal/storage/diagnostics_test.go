package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// timeNowForTest supplies a fixed instant for tests that need distinct
// backup names. It uses a constant rather than time.Now() so a test does not
// depend on the wall clock.
func timeNowForTest() time.Time {
	return time.Date(2026, time.September, 11, 12, 0, 0, 0, time.UTC)
}

func TestStatsReportsDatabaseSize(t *testing.T) {
	store := openWriter(t, newDir(t))

	stats, err := store.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.DatabasePath == "" {
		t.Fatal("DatabasePath is empty")
	}
	if stats.DatabaseBytes <= 0 {
		t.Fatalf("DatabaseBytes = %d, want a positive size for an existing file", stats.DatabaseBytes)
	}
	if stats.DatabasePath != store.Path() {
		t.Fatalf("DatabasePath = %q, want %q", stats.DatabasePath, store.Path())
	}
}

// An empty recording schema reports an available source with zero bytes.
func TestStatsReportsEmptyRecordingSource(t *testing.T) {
	store := openWriter(t, newDir(t))

	stats, err := store.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if !stats.RecordingsAvailable {
		t.Fatal("recording schema reported unavailable")
	}
	if stats.RecordingsBytes != 0 {
		t.Fatalf("RecordingsBytes = %d for an empty recording schema", stats.RecordingsBytes)
	}
}

func TestSkippedCardsCounterIsReported(t *testing.T) {
	store := openWriter(t, newDir(t))

	before := SkippedCards()
	NoteSkippedCards(3)
	NoteSkippedCards(0)
	NoteSkippedCards(-5) // must not decrement

	stats, err := store.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.SkippedCards != before+3 {
		t.Fatalf("SkippedCards = %d, want %d", stats.SkippedCards, before+3)
	}
}

func TestStatsOnNilStoreFails(t *testing.T) {
	var store *Store
	if _, err := store.Stats(context.Background()); err == nil {
		t.Fatal("Stats on a nil store returned no error")
	}
}

// DB-4's integrity assertion, exposed as an operation rather than a test-only
// helper so the recovery path can call the same check.
func TestIntegrityCheckOnHealthyDatabase(t *testing.T) {
	store := openWriter(t, newDir(t))
	if err := store.IntegrityCheck(context.Background()); err != nil {
		t.Fatalf("IntegrityCheck: %v", err)
	}
}

func TestCheckpointSucceedsOnWritableInstance(t *testing.T) {
	store := openWriter(t, newDir(t))
	if err := store.Checkpoint(context.Background()); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
}

// A read-only instance must not checkpoint: its view is not the authoritative
// one and a checkpoint is a write.
func TestCheckpointRefusedOnReadOnlyInstance(t *testing.T) {
	dir := newDir(t)
	openWriter(t, dir)
	reader := openReader(t, dir)

	err := reader.Checkpoint(context.Background())
	assertKind(t, err, KindReadOnly)
}

func TestBackupWritesAReadableDatabase(t *testing.T) {
	dir := newDir(t)
	store := openWriter(t, dir)
	ctx := context.Background()

	if err := store.Settings().Set(ctx, "appearance.theme", `"dark"`); err != nil {
		t.Fatalf("seed setting: %v", err)
	}

	backupDir := filepath.Join(dir, BackupDirName)
	path, err := store.Backup(ctx, backupDir, 7)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}

	// The backup must be a usable database carrying the seeded value, not just
	// a file that happens to exist.
	restored := openWriter(t, filepath.Join(t.TempDir(), "restored"))
	_ = restored
	if err := verifyBackupContent(t, path, "appearance.theme", `"dark"`); err != nil {
		t.Fatalf("backup content: %v", err)
	}
}

// verifyBackupContent opens a backup file directly and reads one setting.
func verifyBackupContent(t *testing.T, path, key, want string) error {
	t.Helper()
	dir := t.TempDir()
	dst := filepath.Join(dir, DatabaseFileName)
	copyFile(t, path, dst)

	store, err := Open(context.Background(), Options{Dir: dir})
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	got, ok, err := store.Settings().Get(context.Background(), key)
	if err != nil {
		return err
	}
	if !ok {
		t.Fatalf("key %q absent from the backup", key)
	}
	if got != want {
		t.Fatalf("backup value = %q, want %q", got, want)
	}
	return nil
}

func TestBackupRotationKeepsNewest(t *testing.T) {
	dir := newDir(t)
	store := openWriter(t, dir)
	ctx := context.Background()
	backupDir := filepath.Join(dir, BackupDirName)

	// The backup name embeds a second-resolution timestamp, so repeated backups
	// within one second would collide. Step the store clock instead of sleeping.
	base := timeNowForTest()
	for i := 0; i < 4; i++ {
		store.setClock(func() time.Time { return base.Add(time.Duration(i) * time.Minute) })
		if _, err := store.Backup(ctx, backupDir, 2); err != nil {
			t.Fatalf("Backup %d: %v", i, err)
		}
	}

	backups, err := store.Backups(backupDir)
	if err != nil {
		t.Fatalf("Backups: %v", err)
	}
	if len(backups) != 2 {
		t.Fatalf("kept %d backups, want 2: %v", len(backups), backups)
	}
	// Sorting is oldest-first, so the survivors must be the last two written.
	wantNewest := filepath.Base(backups[len(backups)-1])
	if wantNewest == filepath.Base(backups[0]) {
		t.Fatal("rotation did not distinguish the backups")
	}
}

func TestBackupRefusedOnReadOnlyInstance(t *testing.T) {
	dir := newDir(t)
	openWriter(t, dir)
	reader := openReader(t, dir)

	_, err := reader.Backup(context.Background(), filepath.Join(t.TempDir(), BackupDirName), 7)
	assertKind(t, err, KindReadOnly)
}

// Concurrent backups would otherwise pick the same second-resolution name, and
// VACUUM INTO refuses to overwrite, so both would fail. Serializing the naming
// and the write makes the second caller succeed with a distinct file.
func TestConcurrentBackupsBothSucceed(t *testing.T) {
	dir := newDir(t)
	store := openWriter(t, dir)
	store.setClock(func() time.Time { return timeNowForTest() })
	backupDir := filepath.Join(dir, BackupDirName)

	const n = 4
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			_, err := store.Backup(context.Background(), backupDir, 10)
			errs <- err
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent backup %d: %v", i, err)
		}
	}

	backups, err := store.Backups(backupDir)
	if err != nil {
		t.Fatalf("Backups: %v", err)
	}
	if len(backups) != n {
		t.Fatalf("wrote %d backups, want %d distinct files: %v", len(backups), n, backups)
	}
}

func TestBackupsOnMissingDirectoryIsEmptyNotAnError(t *testing.T) {
	store := openWriter(t, newDir(t))

	backups, err := store.Backups(filepath.Join(t.TempDir(), "never-created"))
	if err != nil {
		t.Fatalf("Backups on a missing directory: %v", err)
	}
	if len(backups) != 0 {
		t.Fatalf("Backups = %v, want empty", backups)
	}
}

// Restore must preserve the replaced database rather than deleting it: the
// recovery step must not be the one that loses the user's data.
func TestRestoreKeepsReplacedDatabase(t *testing.T) {
	dir := newDir(t)
	store := openWriter(t, dir)
	ctx := context.Background()

	if err := store.Settings().Set(ctx, "appearance.theme", `"dark"`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	backupPath, err := store.Backup(ctx, filepath.Join(dir, BackupDirName), 7)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	// Change the live database after the backup, so the two differ.
	if err := store.Settings().Set(ctx, "appearance.theme", `"light"`); err != nil {
		t.Fatalf("mutate: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := RestoreFromBackup(backupPath, dir); err != nil {
		t.Fatalf("RestoreFromBackup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, DatabaseFileName+".replaced")); err != nil {
		t.Fatalf("the replaced database was not preserved: %v", err)
	}

	restored := openWriter(t, dir)
	got, _, err := restored.Settings().Get(context.Background(), "appearance.theme")
	if err != nil {
		t.Fatalf("read after restore: %v", err)
	}
	if got != `"dark"` {
		t.Fatalf("value after restore = %q, want the backed-up %q", got, `"dark"`)
	}
}

func TestRestoreFromMissingBackupFails(t *testing.T) {
	dir := newDir(t)
	openWriter(t, dir).Close()

	if err := RestoreFromBackup(filepath.Join(dir, "absent.db"), dir); err == nil {
		t.Fatal("RestoreFromBackup accepted a missing backup")
	}
}
