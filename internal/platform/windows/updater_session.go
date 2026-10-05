package windows

import (
	"context"
	"sync"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
)

// updateSession owns the callback state independently of the DLL so failed
// launches and repeated terminal callbacks can be exercised without native UI.
type updateSession struct {
	mu           sync.Mutex
	installMu    sync.Mutex
	automatic    bool
	checking     bool
	available    *string
	closed       bool
	lastChecked  *time.Time
	events       chan platform.UpdaterEvent
	canInstall   func() bool
	prepare      func() error
	launch       func(string) error
	shutdown     func()
	cancel       func()
	prepared     bool
	installing   bool
	shuttingDown bool
}

func (u *updateSession) canShutdown() bool {
	u.installMu.Lock()
	defer u.installMu.Unlock()
	u.mu.Lock()
	closed, prepared := u.closed, u.prepared
	canInstall, prepare, shutdown := u.canInstall, u.prepare, u.shutdown
	u.mu.Unlock()
	if closed || canInstall == nil || !canInstall() || prepare == nil || shutdown == nil {
		return false
	}
	if prepared {
		return true
	}
	if err := prepare(); err != nil {
		return false
	}
	u.mu.Lock()
	u.prepared = true
	u.mu.Unlock()
	return true
}

func (u *updateSession) launchInstaller(path string) bool {
	u.installMu.Lock()
	defer u.installMu.Unlock()
	u.mu.Lock()
	ready := !u.closed && u.prepared && !u.installing
	launch := u.launch
	u.mu.Unlock()
	if !ready {
		return false
	}
	if launch == nil || launch(path) != nil {
		// WinSparkle does not call its error callback when ShellExecute fails
		// (including UAC cancellation). Restore recording before returning.
		u.interruptInstallLocked()
		return false
	}
	u.mu.Lock()
	u.installing = true
	u.mu.Unlock()
	return true
}

func (u *updateSession) requestShutdown() {
	u.mu.Lock()
	request := u.shutdown
	ready := !u.closed && u.installing && !u.shuttingDown && request != nil
	if ready {
		u.shuttingDown = true
	}
	u.mu.Unlock()
	if ready {
		request()
	}
}

func (u *updateSession) SetInstallCancelled(cancel func()) {
	u.mu.Lock()
	u.cancel = cancel
	u.mu.Unlock()
}

func (u *updateSession) interruptInstall() {
	u.installMu.Lock()
	defer u.installMu.Unlock()
	u.interruptInstallLocked()
}

func (u *updateSession) interruptInstallLocked() {
	u.mu.Lock()
	u.checking = false
	resume := !u.closed && u.prepared && !u.installing
	cancel := u.cancel
	if resume {
		u.prepared = false
	}
	u.mu.Unlock()
	if resume && cancel != nil {
		cancel()
	}
}

// WinSparkle's did-find callback has no version argument. An empty, non-nil
// AvailableVersion records the known update without inventing a version string.
func (u *updateSession) reportFound() {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed {
		return
	}
	u.checking = false
	unknownVersion := ""
	u.available = &unknownVersion
	state := u.stateLocked()
	select {
	case u.events <- platform.UpdaterEvent{State: state}:
	default:
		select {
		case <-u.events:
		default:
		}
		u.events <- platform.UpdaterEvent{State: state}
	}
}

func (u *updateSession) State(ctx context.Context) (platform.UpdaterState, error) {
	if err := ctx.Err(); err != nil {
		return platform.UpdaterState{}, err
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.stateLocked(), nil
}

func (u *updateSession) stateLocked() platform.UpdaterState {
	state := platform.UpdaterState{Automatic: u.automatic, Checking: u.checking}
	if u.available != nil {
		version := *u.available
		state.AvailableVersion = &version
	}
	if u.lastChecked != nil {
		at := *u.lastChecked
		state.LastCheckedAt = &at
	}
	return state
}

func (u *updateSession) Events() <-chan platform.UpdaterEvent { return u.events }

func (u *updateSession) closeState() {
	u.installMu.Lock()
	defer u.installMu.Unlock()
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.closed {
		u.closed = true
		u.checking = false
		close(u.events)
	}
}
