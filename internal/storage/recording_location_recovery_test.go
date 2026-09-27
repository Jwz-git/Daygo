package storage

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestOldBackupCannotAutomaticallyRestoreAfterWindowsRecordingMove(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows recording location guard")
	}
	dir := t.TempDir()
	store, err := Open(context.Background(), Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	backup, err := store.Backup(context.Background(), filepath.Join(dir, BackupDirName), 7)
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(backup, old, old); err != nil {
		t.Fatal(err)
	}
	guard := filepath.Join(dir, "recordings-location.guard")
	if err := os.WriteFile(guard, []byte("new media location"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.recoverFromNewestBackup(); err == nil {
		t.Fatal("automatic recovery accepted a backup from before the media move")
	}
	if _, err := os.Stat(filepath.Join(dir, DatabaseFileName)); err != nil {
		t.Fatalf("original database was moved: %v", err)
	}
}
