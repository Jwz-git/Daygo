//go:build windows

package storage

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// fileLock is the Windows implementation of the process-lifetime advisory
// locks used for the single database writer and the single capture owner.
//
// LockFileEx locks a byte range rather than an abstract file. Every Daygo
// instance therefore locks the same first byte. Windows permits the range to
// extend beyond EOF, so the dedicated lock file may remain empty.
type fileLock struct {
	file *os.File
	path string
}

// tryLock takes an exclusive, non-blocking lock on the first byte of path.
// Keeping the file handle open keeps the lock alive; Windows releases it when
// the handle closes or the process terminates.
func tryLock(path string) (*fileLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}

	overlapped := windows.Overlapped{}
	err = windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		&overlapped,
	)
	if err != nil {
		closeErr := f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, fmt.Errorf("%w: %s", ErrLockBusy, path)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("lock %s: %w (close: %v)", path, err, closeErr)
		}
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return &fileLock{file: f, path: path}, nil
}

// release explicitly unlocks the byte range before closing its handle. It is
// safe on a nil receiver so Store.Close can call it unconditionally.
func (l *fileLock) release() error {
	if l == nil || l.file == nil {
		return nil
	}

	overlapped := windows.Overlapped{}
	unlockErr := windows.UnlockFileEx(
		windows.Handle(l.file.Fd()),
		0,
		1,
		0,
		&overlapped,
	)
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
