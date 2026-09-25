//go:build darwin && cgo && daygo_updater

package darwin

/*
#cgo CFLAGS: -F${SRCDIR}/../../../build/deps/Sparkle-2.10.0
#cgo LDFLAGS: -F${SRCDIR}/../../../build/deps/Sparkle-2.10.0 -Wl,-rpath,${SRCDIR}/../../../build/deps/Sparkle-2.10.0 -framework Sparkle -framework Foundation -framework AppKit
#include <stdlib.h>
#include "updater_bridge.h"
*/
import "C"

import (
	"context"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/Jwz-git/Daygo/internal/platform"
)

type Updater struct {
	mu         sync.RWMutex
	events     chan platform.UpdaterEvent
	available  *string
	canInstall func() bool
	prepare    func() error
	cancel     func()
	closeOnce  sync.Once
	startOnce  sync.Once
}

var activeUpdater *Updater
var activeUpdaterMu sync.RWMutex

var _ platform.Updater = (*Updater)(nil)
var _ platform.UpdateInstallCoordinator = (*Updater)(nil)
var _ platform.UpdateCopySink = (*Updater)(nil)

func NewUpdater() (*Updater, error) {
	u := &Updater{events: make(chan platform.UpdaterEvent, 8)}
	activeUpdaterMu.Lock()
	activeUpdater = u
	activeUpdaterMu.Unlock()
	if status := C.dg_updater_start(); status != 0 {
		return nil, fmt.Errorf("start Sparkle updater: status %d", int(status))
	}
	return u, nil
}

func (u *Updater) CheckForUpdates(ctx context.Context, interactive bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flag := C.int32_t(0)
	if interactive {
		flag = 1
	}
	C.dg_updater_check(flag)
	return nil
}

func (u *Updater) State(ctx context.Context) (platform.UpdaterState, error) {
	if err := ctx.Err(); err != nil {
		return platform.UpdaterState{}, err
	}
	state := platform.UpdaterState{Automatic: C.dg_updater_automatic() != 0, Checking: C.dg_updater_checking() != 0}
	u.mu.RLock()
	if u.available != nil {
		version := *u.available
		state.AvailableVersion = &version
	}
	u.mu.RUnlock()
	if ts := int64(C.dg_updater_last_checked()); ts > 0 {
		at := time.Unix(ts, 0)
		state.LastCheckedAt = &at
	}
	return state, nil
}

func (u *Updater) SetAutomaticChecks(ctx context.Context, enabled bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flag := C.int32_t(0)
	if enabled {
		flag = 1
	}
	C.dg_updater_set_automatic(flag)
	return nil
}
func (u *Updater) Events() <-chan platform.UpdaterEvent { return u.events }
func (u *Updater) SetInstallCallbacks(can func() bool, prepare func() error, _ func()) {
	u.mu.Lock()
	u.canInstall = can
	u.prepare = prepare
	u.mu.Unlock()
	u.startOnce.Do(func() { C.dg_updater_activate() })
}

func (u *Updater) SetInstallCancelled(cancel func()) {
	u.mu.Lock()
	u.cancel = cancel
	u.mu.Unlock()
}

// SetInstallRefusedMessage implements platform.UpdateCopySink. The copy is
// localized by the frontend and pushed through the app layer; an empty message
// keeps the adapter's previous copy (docs/05 §5.5.1).
func (u *Updater) SetInstallRefusedMessage(message string) {
	if message == "" {
		return
	}
	c := C.CString(message)
	defer C.free(unsafe.Pointer(c))
	C.dg_updater_set_install_refused_message(c)
}
func (u *Updater) Close() error {
	u.closeOnce.Do(func() {
		C.dg_updater_stop()
		activeUpdaterMu.Lock()
		if activeUpdater == u {
			activeUpdater = nil
		}
		activeUpdaterMu.Unlock()
		close(u.events)
	})
	return nil
}

//export dgGoUpdaterFound
func dgGoUpdaterFound(version *C.char) {
	activeUpdaterMu.RLock()
	u := activeUpdater
	activeUpdaterMu.RUnlock()
	if u == nil {
		return
	}
	v := C.GoString(version)
	u.mu.Lock()
	u.available = &v
	u.mu.Unlock()
	state, _ := u.State(context.Background())
	select {
	case u.events <- platform.UpdaterEvent{State: state}:
	default:
		select {
		case <-u.events:
		default:
		}
		select {
		case u.events <- platform.UpdaterEvent{State: state}:
		default:
		}
	}
}

//export dgGoUpdaterCanInstall
func dgGoUpdaterCanInstall() C.int32_t {
	activeUpdaterMu.RLock()
	u := activeUpdater
	activeUpdaterMu.RUnlock()
	if u == nil {
		return 0
	}
	u.mu.RLock()
	can := u.canInstall
	u.mu.RUnlock()
	if can != nil && can() {
		return 1
	}
	return 0
}

//export dgGoUpdaterPrepare
func dgGoUpdaterPrepare() C.int32_t {
	activeUpdaterMu.RLock()
	u := activeUpdater
	activeUpdaterMu.RUnlock()
	if u == nil {
		return -1
	}
	u.mu.RLock()
	prepare := u.prepare
	u.mu.RUnlock()
	if prepare == nil || prepare() != nil {
		return -1
	}
	return 0
}

//export dgGoUpdaterCancelled
func dgGoUpdaterCancelled() {
	activeUpdaterMu.RLock()
	u := activeUpdater
	activeUpdaterMu.RUnlock()
	if u == nil {
		return
	}
	u.mu.RLock()
	cancel := u.cancel
	u.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
}
