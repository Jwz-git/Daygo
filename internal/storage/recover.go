package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNoBackup reports that corruption was detected but there is no backup to
// recover from. The database is left exactly as it was: a corruption with no
// replacement is not a reason to destroy anything.
var ErrNoBackup = errors.New("storage: no backup available to recover from")

// recoverFromNewestBackup replaces a corrupt database with the newest backup.
//
// This is the "open and recover" path docs/05 §5.6.2 rule 5 requires, and the
// behaviour DB-7 asserts. It runs only when classification says corruption:
// the caller checks IsCorrupt, so an environment failure — a full disk, an
// unwritable directory — never reaches here. That distinction is the whole
// point; restoring over an intact database on a full disk would destroy it.
//
// Nothing is deleted. The corrupt file is renamed aside so it stays available
// for inspection, and the -wal/-shm files that belong to it are removed so the
// restored database cannot replay frames from the old one.
//
// The returned string is the backup that was restored.
func (s *Store) recoverFromNewestBackup() (string, error) {
	if s == nil {
		return "", newError(KindEnvironment, "recover: no store")
	}
	if s.mode != ModeReadWrite {
		// A read-only instance cannot restore, and must not try: it does not
		// hold the write lock, so another process may be using the file.
		return "", newError(KindReadOnly, "recover: instance is read-only")
	}

	dir := filepath.Dir(s.path)
	backups, err := s.Backups(filepath.Join(dir, BackupDirName))
	if err != nil {
		return "", err
	}
	if len(backups) == 0 {
		return "", ErrNoBackup
	}

	// The failed connection still holds a descriptor on the file about to be
	// replaced, which on Windows alone would block the rename.
	if s.db != nil {
		_ = s.db.Close()
		s.db = nil
	}

	// Backups() returns oldest first, so the newest is the last entry.
	newest := backups[len(backups)-1]
	if err := RestoreFromBackup(newest, dir); err != nil {
		return "", err
	}
	return newest, nil
}

// preservedPath returns a name for the corrupt database that does not already
// exist.
//
// A fixed ".replaced" name would be overwritten by a second recovery, silently
// discarding the first corrupted copy — and that copy is the only evidence of
// what went wrong. A counter keeps every one of them.
func preservedPath(current string) string {
	aside := current + ".replaced"
	if _, err := os.Stat(aside); err != nil {
		return aside
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s.replaced.%d", current, i)
		if _, err := os.Stat(candidate); err != nil {
			return candidate
		}
	}
}
