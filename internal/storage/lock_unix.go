//go:build !windows

package storage

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// fileLock is an advisory exclusive lock held on a dedicated lock file.
//
// A separate file is used rather than the database itself because the database
// is opened read-only by non-writer instances: locking the database file would
// require a writable descriptor. Keeping locks in their own file also lets the
// write lock and the capture-owner lock be acquired independently.
//
// The lock is released by the kernel when the process exits for any reason, so
// a crash never leaves a stale lock behind. That property is why flock was
// chosen over an in-database lock row, which would need heartbeat expiry and
// could not be claimed at all by a read-only instance.
//
// flock semantics are per open file description, so a second Open in the same
// process still contends. Tests rely on that to exercise the two-instance path.
type fileLock struct {
	file *os.File
	path string
}

// tryLock takes an exclusive, non-blocking lock on path, creating the lock file
// when absent. It returns ErrLockBusy when another instance holds it.
func tryLock(path string) (*fileLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		// Closing here is what makes a failed attempt harmless: the descriptor
		// never escapes, so a subsequent attempt in this process can succeed.
		closeErr := f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("%w: %s", ErrLockBusy, path)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("lock %s: %w (close: %v)", path, err, closeErr)
		}
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return &fileLock{file: f, path: path}, nil
}

// release drops the lock and closes the descriptor. It is safe on a nil
// receiver so callers can defer it unconditionally.
func (l *fileLock) release() error {
	if l == nil || l.file == nil {
		return nil
	}
	// Unlocking explicitly before close keeps the window in which the lock is
	// still held after release() returns as small as possible.
	unlockErr := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	l.file = nil
	if unlockErr != nil {
		return fmt.Errorf("unlock %s: %w", l.path, unlockErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close lock file %s: %w", l.path, closeErr)
	}
	return nil
}
