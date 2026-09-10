//go:build windows

package storage

import "fmt"

// Windows build support is not delivered yet. flock is a POSIX facility; the
// Windows equivalent (LockFileEx) belongs to the same undecided work as the
// rest of the platform adapter (docs/09 §9.8 item 18). This stub keeps
// GOOS=windows compiling without silently pretending a lock was taken.
type fileLock struct {
	path string
}

func tryLock(path string) (*fileLock, error) {
	return nil, fmt.Errorf("storage: tryLock: windows locking not implemented (%s)", path)
}

func (l *fileLock) release() error { return nil }
