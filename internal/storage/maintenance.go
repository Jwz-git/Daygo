package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// BackupDirName is the subdirectory holding database backups (docs/03 §3.1).
const BackupDirName = "backups"

// DefaultBackupRetention is how many daily backups are kept.
//
// This is decision 09 §9.8 item 10, whose owner is data engineering and whose
// deadline is "before the maintenance task is implemented". Seven gives a week
// of history, which covers the realistic recovery case (a problem noticed a few
// days after it started) at a bounded cost of seven database copies. The choice
// and its alternatives are recorded in docs/decisions/data-backup-retention.md;
// it is a default rather than a constant so a caller can lower it.
const DefaultBackupRetention = 7

// Checkpoint folds the WAL back into the main database. docs/03 §3.6 runs it
// every 300 seconds.
//
// PASSIVE is used deliberately: it checkpoints as much as it can without
// blocking readers or writers. A TRUNCATE would stall a concurrent capture
// write, and the WAL being large for a while is much cheaper than a dropped
// frame.
func (s *Store) Checkpoint(ctx context.Context) error {
	if s == nil {
		return newError(KindEnvironment, "checkpoint: no store")
	}
	if err := s.requireWritable("checkpoint"); err != nil {
		return err
	}

	// PRAGMA cannot take a bound parameter, and the mode is a constant from this
	// package rather than user input.
	if _, err := s.db.ExecContext(ctx, "PRAGMA wal_checkpoint(PASSIVE)"); err != nil {
		return wrap("wal checkpoint", err)
	}
	return nil
}

// Backup writes a consistent copy of the database into dir and rotates old
// copies down to retain.
//
// VACUUM INTO is used rather than copying the file: a plain copy of a live
// database can capture a torn WAL and produce a backup that will not open.
// VACUUM INTO reads through a transaction and writes a self-contained database.
//
// Backup requires the write lock. A read-only instance cannot checkpoint
// consistently and must not create backups that look authoritative.
func (s *Store) Backup(ctx context.Context, dir string, retain int) (string, error) {
	if s == nil {
		return "", newError(KindEnvironment, "backup: no store")
	}
	if err := s.requireWritable("backup"); err != nil {
		return "", err
	}
	if retain <= 0 {
		retain = DefaultBackupRetention
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", wrap("create backup directory", err)
	}

	// Two backups in the same second would otherwise pick the same name, and
	// VACUUM INTO refuses to overwrite: both would fail. Serialize naming and
	// the write so a concurrent caller queues rather than collides.
	s.backupMu.Lock()
	defer s.backupMu.Unlock()

	// The name carries a monotonic sequence number rather than relying on clock
	// resolution: two calls in the same instant (a fixed test clock, or a
	// coarse system clock) would otherwise still collide. The leading timestamp
	// field keeps names lexically sortable by age; the sequence only breaks
	// ties within one instant.
	now := s.now().UTC()
	s.backupSeq++
	name := fmt.Sprintf("daygo-%s-%04d.db", now.Format("20060102-150405"), s.backupSeq)
	dest := filepath.Join(dir, name)

	// VACUUM INTO fails if the target exists, which is the behavior we want:
	// silently overwriting an earlier backup would destroy the history it is
	// there to preserve.
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO ?", dest); err != nil {
		return "", wrap("vacuum into backup", err)
	}

	if err := s.rotateBackups(dir, retain); err != nil {
		// The backup itself succeeded; rotation failing is worth reporting but
		// must not discard the backup we just made.
		return dest, err
	}
	return dest, nil
}

// rotateBackups deletes the oldest backups until at most retain remain. It is
// best-effort per file: one undeletable backup must not stop the others, and
// must not fail an otherwise successful backup run.
func (s *Store) rotateBackups(dir string, retain int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return wrap("list backups", err)
	}

	var backups []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".db" {
			continue
		}
		backups = append(backups, entry.Name())
	}
	if len(backups) <= retain {
		return nil
	}
	// Names embed a sortable UTC timestamp, so lexical order is age order.
	sort.Strings(backups)

	var firstErr error
	for _, name := range backups[:len(backups)-retain] {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && firstErr == nil {
			firstErr = wrap("remove old backup "+name, err)
		}
	}
	return firstErr
}

// Backups lists the backup files currently on disk, oldest first. It exists so
// maintenance and diagnostics share one definition of what a backup is.
func (s *Store) Backups(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, wrap("list backups", err)
	}
	var out []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".db" {
			continue
		}
		out = append(out, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(out)
	return out, nil
}

// IntegrityCheck runs PRAGMA integrity_check and returns an error when the
// database is not sound. It is the check the recovery path and DB-4 rely on.
func (s *Store) IntegrityCheck(ctx context.Context) error {
	if s == nil {
		return newError(KindEnvironment, "integrity check: no store")
	}
	var result string
	if err := s.db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return wrap("integrity_check", err)
	}
	if result != "ok" {
		return &Error{Kind: KindCorrupt, Op: "integrity_check: " + result}
	}
	return nil
}

// RestoreFromBackup replaces the database at dir with a backup, keeping the
// previous file beside it under a .replaced name.
//
// Corruption recovery calls this automatically (see recoverFromNewestBackup);
// it is exported because the documented manual procedure uses the same code
// path rather than a second, untested one.
//
// Nothing is deleted. The replaced database is renamed aside, and a name
// collision from an earlier recovery is resolved by suffixing rather than
// overwriting, so no previous copy is lost (docs/modules/data.md: 故障先停止
// 写入、保留原库与备份).
//
// The caller must have closed the store first; this function operates on files.
func RestoreFromBackup(backupPath, dir string) error {
	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("storage: restore: backup %s: %w", backupPath, err)
	}
	current := filepath.Join(dir, DatabaseFileName)

	if _, err := os.Stat(current); err == nil {
		if err := os.Rename(current, preservedPath(current)); err != nil {
			return fmt.Errorf("storage: restore: preserve current database: %w", err)
		}
	}
	// The WAL belongs to the file that was just moved aside; leaving it would
	// make the restored database replay frames from the old one.
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(current + suffix); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("storage: restore: remove %s: %w", current+suffix, err)
		}
	}

	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("storage: restore: read backup: %w", err)
	}
	if err := os.WriteFile(current, data, 0o600); err != nil {
		return fmt.Errorf("storage: restore: write database: %w", err)
	}
	return nil
}
