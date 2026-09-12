package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// corruptDatabase truncates a database and drops its WAL, which is how a torn
// write presents itself: the header survives, the page tree does not.
func corruptDatabase(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, DatabaseFileName)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat database: %v", err)
	}
	if err := os.Truncate(path, info.Size()/3); err != nil {
		t.Fatalf("truncate database: %v", err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(path + suffix); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove %s: %v", suffix, err)
		}
	}
}

// DB-7: a truncated database triggers backup recovery. docs/08 §8.4 states it,
// and docs/05 §5.6.2 rule 5 is the rule it comes from.
func TestCorruptDatabaseRecoversFromBackup(t *testing.T) {
	dir := newDir(t)
	ctx := context.Background()

	// Build history: a value that is in the backup, and a newer one that
	// deliberately is not. Recovery restores the backup, so the newer value must
	// be gone — that loss is the reason recovery has to be visible.
	store := openWriter(t, dir)
	if err := store.Settings().Set(ctx, "appearance.theme", `"dark"`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	backupPath, err := store.Backup(ctx, filepath.Join(dir, BackupDirName), DefaultBackupRetention)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if err := store.Settings().Set(ctx, "appearance.language", `"en"`); err != nil {
		t.Fatalf("seed newer value: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	corruptDatabase(t, dir)

	recovered, err := Open(ctx, Options{Dir: dir})
	if err != nil {
		t.Fatalf("Open after corruption: %v", err)
	}
	t.Cleanup(func() { _ = recovered.Close() })

	// The recovery must be reported: the user is looking at older data.
	if got := recovered.RecoveredFrom(); got != backupPath {
		t.Fatalf("RecoveredFrom() = %q, want %q", got, backupPath)
	}

	// The backed-up value is back.
	value, ok, err := recovered.Settings().Get(ctx, "appearance.theme")
	if err != nil {
		t.Fatalf("read restored value: %v", err)
	}
	if !ok || value != `"dark"` {
		t.Fatalf("restored theme = (%q, %v), want (\"dark\", true)", value, ok)
	}

	// The value written after the backup is gone, as it must be: a backup is a
	// point in time.
	if _, ok, err := recovered.Settings().Get(ctx, "appearance.language"); err != nil {
		t.Fatalf("read newer value: %v", err)
	} else if ok {
		t.Fatal("a value written after the backup survived the restore")
	}

	// The corrupt file is preserved, not deleted.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	preserved := false
	for _, e := range entries {
		if strings.Contains(e.Name(), ".replaced") {
			preserved = true
		}
	}
	if !preserved {
		t.Fatalf("the corrupt database was not preserved; dir contains %v", dirNames(entries))
	}
}

// Corruption with no backup must change nothing. Destroying the only copy of
// the user's data because it cannot be replaced would be worse than the
// corruption itself.
func TestCorruptDatabaseWithoutBackupIsReportedNotRepaired(t *testing.T) {
	dir := newDir(t)
	ctx := context.Background()

	store := openWriter(t, dir)
	if err := store.Settings().Set(ctx, "appearance.theme", `"dark"`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	corruptDatabase(t, dir)
	before, err := os.Stat(filepath.Join(dir, DatabaseFileName))
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}

	_, openErr := Open(ctx, Options{Dir: dir})
	if openErr == nil {
		t.Fatal("Open succeeded on a corrupt database with no backup")
	}
	if !IsCorrupt(openErr) {
		t.Fatalf("error = %v, want a corruption classification", openErr)
	}

	after, err := os.Stat(filepath.Join(dir, DatabaseFileName))
	if err != nil {
		t.Fatalf("stat after: %v — the database was removed without a backup", err)
	}
	if before.Size() != after.Size() {
		t.Fatalf("database size changed from %d to %d with no backup to restore",
			before.Size(), after.Size())
	}
}

// A read-only instance must not attempt recovery: it holds no write lock, so
// another process may be using the file.
func TestReadOnlyInstanceDoesNotRecover(t *testing.T) {
	dir := newDir(t)
	ctx := context.Background()

	// The holder keeps the write lock for the duration of the test, which is
	// what forces the next instance to be read-only.
	holder := openWriter(t, dir)
	if _, err := holder.Backup(ctx, filepath.Join(dir, BackupDirName), DefaultBackupRetention); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	reader := openReader(t, dir)
	if got := reader.RecoveredFrom(); got != "" {
		t.Fatalf("a read-only instance reported recovery from %q", got)
	}
}

// Recovery must be reported only when it happened. A healthy open is not a
// recovery, and saying otherwise would make the signal useless.
func TestHealthyOpenReportsNoRecovery(t *testing.T) {
	store := openWriter(t, newDir(t))
	if got := store.RecoveredFrom(); got != "" {
		t.Fatalf("RecoveredFrom() = %q on a healthy open", got)
	}
}

// A second recovery must not overwrite the copy the first one preserved. That
// file is the only evidence of what went wrong.
func TestRepeatedRecoveryPreservesEveryCopy(t *testing.T) {
	dir := newDir(t)
	ctx := context.Background()

	store := openWriter(t, dir)
	if _, err := store.Backup(ctx, filepath.Join(dir, BackupDirName), DefaultBackupRetention); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	store.Close()

	// Two cycles of corrupt → recover.
	for round := 1; round <= 2; round++ {
		corruptDatabase(t, dir)
		recovered, err := Open(ctx, Options{Dir: dir})
		if err != nil {
			t.Fatalf("round %d: Open after corruption: %v", round, err)
		}
		if recovered.RecoveredFrom() == "" {
			t.Fatalf("round %d: recovery was not reported", round)
		}
		if err := recovered.Close(); err != nil {
			t.Fatalf("round %d: Close: %v", round, err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	copies := 0
	for _, e := range entries {
		if strings.Contains(e.Name(), ".replaced") {
			copies++
		}
	}
	if copies != 2 {
		t.Fatalf("preserved %d corrupt copies, want 2 (dir: %v)", copies, dirNames(entries))
	}
}

// recoveredFromNewestBackup refuses to run on a read-only instance or without
// a store, rather than half-performing a file replacement.
func TestRecoverRefusesWithoutWriteAccess(t *testing.T) {
	var nilStore *Store
	if _, err := nilStore.recoverFromNewestBackup(); err == nil {
		t.Fatal("recoverFromNewestBackup succeeded on a nil store")
	}

	dir := newDir(t)
	openWriter(t, dir)
	reader := openReader(t, dir)

	if _, err := reader.recoverFromNewestBackup(); !IsKind(err, KindReadOnly) {
		t.Fatalf("error = %v, want a read-only refusal", err)
	}
}

// A store with no backups reports ErrNoBackup rather than failing obscurely, so
// the caller can tell "nothing to restore" from "restore failed".
func TestRecoverWithoutBackupsReportsErrNoBackup(t *testing.T) {
	store := openWriter(t, newDir(t))

	if _, err := store.recoverFromNewestBackup(); !errors.Is(err, ErrNoBackup) {
		t.Fatalf("error = %v, want ErrNoBackup", err)
	}
}

func dirNames(entries []os.DirEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}
