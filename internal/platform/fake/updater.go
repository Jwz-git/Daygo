package fake

import (
	"context"
	"sync"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
)

// Updater is a deterministic implementation of platform.Updater. It owns no
// network access, no Sparkle framework and no relaunch: its controls exist only
// so contract tests can drive update outcomes (docs/decisions/delivery-auto-update.md).
type Updater struct {
	mu            sync.Mutex
	automatic     bool
	checking      bool
	available     *string
	lastCheckedAt *time.Time
	events        chan platform.UpdaterEvent

	// Test controls.
	nextVersion     *string
	checkErr        error
	lastInteractive *bool
	checkCount      int
}

// NewUpdater returns a fake with automatic checks disabled, matching the
// opt-in default: the app does not silently check for updates until the user
// enables it (docs/decisions/delivery-auto-update.md §2).
func NewUpdater() *Updater {
	return &Updater{events: make(chan platform.UpdaterEvent, 8)}
}

var _ platform.Updater = (*Updater)(nil)

// SetNextVersion controls the version a subsequent check discovers. A nil value
// (the zero state) means the check finds no update.
func (u *Updater) SetNextVersion(version string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.nextVersion = &version
}

// SetCheckError forces the next CheckForUpdates to fail, modeling an
// unreachable feed or a signature-verification failure.
func (u *Updater) SetCheckError(err error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.checkErr = err
}

// LastInteractive reports the interactive flag of the most recent check and
// whether any check has run. It exists so tests can assert the interactive vs
// background mapping (docs/decisions/delivery-auto-update.md §2).
func (u *Updater) LastInteractive() (interactive bool, checked bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.lastInteractive == nil {
		return false, false
	}
	return *u.lastInteractive, true
}

// CheckCount reports how many times CheckForUpdates has run.
func (u *Updater) CheckCount() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.checkCount
}

func (u *Updater) CheckForUpdates(ctx context.Context, interactive bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	u.mu.Lock()
	u.checkCount++
	u.lastInteractive = &interactive
	if u.checkErr != nil {
		err := u.checkErr
		u.mu.Unlock()
		return err
	}
	now := time.Now()
	u.lastCheckedAt = &now
	u.available = u.nextVersion
	snapshot := u.snapshotLocked()
	discovered := u.available != nil
	u.mu.Unlock()

	// A discovered update is a state broadcast the app fans out as
	// update:available; "no update found" is not an event (§5.5.3).
	if discovered {
		u.emit(platform.UpdaterEvent{State: snapshot})
	}
	return nil
}

func (u *Updater) State(ctx context.Context) (platform.UpdaterState, error) {
	if err := ctx.Err(); err != nil {
		return platform.UpdaterState{}, err
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.snapshotLocked(), nil
}

func (u *Updater) SetAutomaticChecks(ctx context.Context, enabled bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	u.mu.Lock()
	u.automatic = enabled
	u.mu.Unlock()
	return nil
}

func (u *Updater) Events() <-chan platform.UpdaterEvent { return u.events }

// snapshotLocked builds a value copy of the current state. The caller holds mu.
func (u *Updater) snapshotLocked() platform.UpdaterState {
	state := platform.UpdaterState{Automatic: u.automatic, Checking: u.checking}
	if u.available != nil {
		version := *u.available
		state.AvailableVersion = &version
	}
	if u.lastCheckedAt != nil {
		at := *u.lastCheckedAt
		state.LastCheckedAt = &at
	}
	return state
}

// emit delivers a state broadcast. The channel is a state carrier (docs/05
// §5.7.2 "合并"): when full it drops the oldest so the latest state always
// lands rather than blocking the caller.
func (u *Updater) emit(event platform.UpdaterEvent) {
	for {
		select {
		case u.events <- event:
			return
		default:
			select {
			case <-u.events:
			default:
				return
			}
		}
	}
}
