//go:build windows

package windows

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/updateconfig"
	xwindows "golang.org/x/sys/windows"
)

type Updater struct {
	mu          sync.Mutex
	dll         *xwindows.LazyDLL
	automatic   bool
	checking    bool
	lastChecked *time.Time
	events      chan platform.UpdaterEvent

	checkUI       *xwindows.LazyProc
	checkSilent   *xwindows.LazyProc
	setAutomatic  *xwindows.LazyProc
	getAutomatic  *xwindows.LazyProc
	cleanup       *xwindows.LazyProc
	callbackAddrs []uintptr
	canInstall    func() bool
	prepare       func() error
	shutdown      func()
	closeOnce     sync.Once
	initOnce      sync.Once
}

var _ platform.Updater = (*Updater)(nil)

func NewUpdater() (*Updater, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate executable: %w", err)
	}
	dllPath := filepath.Join(filepath.Dir(executable), "WinSparkle.dll")
	if _, err := os.Stat(dllPath); err != nil {
		return nil, fmt.Errorf("locate WinSparkle.dll: %w", err)
	}
	dll := xwindows.NewLazyDLL(dllPath)
	if err := dll.Load(); err != nil {
		return nil, fmt.Errorf("load WinSparkle.dll: %w", err)
	}
	u := &Updater{
		dll:          dll,
		events:       make(chan platform.UpdaterEvent, 8),
		checkUI:      dll.NewProc("win_sparkle_check_update_with_ui"),
		checkSilent:  dll.NewProc("win_sparkle_check_update_without_ui"),
		setAutomatic: dll.NewProc("win_sparkle_set_automatic_check_for_updates"),
		getAutomatic: dll.NewProc("win_sparkle_get_automatic_check_for_updates"),
		cleanup:      dll.NewProc("win_sparkle_cleanup"),
	}
	feed, _ := xwindows.BytePtrFromString(updateconfig.FeedURL)
	key, _ := xwindows.BytePtrFromString(updateconfig.Ed25519PublicKey)
	dll.NewProc("win_sparkle_set_appcast_url").Call(uintptr(unsafe.Pointer(feed)))
	accepted, _, callErr := dll.NewProc("win_sparkle_set_eddsa_public_key").Call(uintptr(unsafe.Pointer(key)))
	if accepted != 1 {
		return nil, fmt.Errorf("configure WinSparkle Ed25519 key: %v", callErr)
	}
	u.installCallbacks()
	return u, nil
}

func (u *Updater) installCallbacks() {
	done := func() uintptr {
		u.mu.Lock()
		u.checking = false
		u.mu.Unlock()
		return 0
	}
	found := func() uintptr {
		done()
		return 0
	}
	canShutdown := func() uintptr {
		u.mu.Lock()
		canInstall, prepare := u.canInstall, u.prepare
		u.mu.Unlock()
		if canInstall == nil || !canInstall() || prepare == nil || prepare() != nil {
			return 0
		}
		return 1
	}
	shutdown := func() uintptr {
		u.mu.Lock()
		request := u.shutdown
		u.mu.Unlock()
		if request != nil {
			request()
		}
		return 0
	}
	u.callbackAddrs = []uintptr{
		xwindows.NewCallbackCDecl(done), xwindows.NewCallbackCDecl(found),
		xwindows.NewCallbackCDecl(canShutdown), xwindows.NewCallbackCDecl(shutdown),
	}
	u.dll.NewProc("win_sparkle_set_did_not_find_update_callback").Call(u.callbackAddrs[0])
	u.dll.NewProc("win_sparkle_set_did_find_update_callback").Call(u.callbackAddrs[1])
	u.dll.NewProc("win_sparkle_set_can_shutdown_callback").Call(u.callbackAddrs[2])
	u.dll.NewProc("win_sparkle_set_shutdown_request_callback").Call(u.callbackAddrs[3])
}

func (u *Updater) SetInstallCallbacks(canInstall func() bool, prepare func() error, requestShutdown func()) {
	u.mu.Lock()
	u.canInstall = canInstall
	u.prepare = prepare
	u.shutdown = requestShutdown
	u.mu.Unlock()
	u.initOnce.Do(func() {
		u.dll.NewProc("win_sparkle_init").Call()
		automatic, _, _ := u.getAutomatic.Call()
		u.mu.Lock()
		u.automatic = automatic != 0
		u.mu.Unlock()
	})
}

func (u *Updater) CheckForUpdates(ctx context.Context, interactive bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := time.Now()
	u.mu.Lock()
	u.checking = true
	u.lastChecked = &now
	u.mu.Unlock()
	if interactive {
		u.checkUI.Call()
	} else {
		u.checkSilent.Call()
	}
	return nil
}

func (u *Updater) State(ctx context.Context) (platform.UpdaterState, error) {
	if err := ctx.Err(); err != nil {
		return platform.UpdaterState{}, err
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	state := platform.UpdaterState{Automatic: u.automatic, Checking: u.checking}
	if u.lastChecked != nil {
		at := *u.lastChecked
		state.LastCheckedAt = &at
	}
	return state, nil
}

func (u *Updater) SetAutomaticChecks(ctx context.Context, enabled bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	value := uintptr(0)
	if enabled {
		value = 1
	}
	u.setAutomatic.Call(value)
	u.mu.Lock()
	u.automatic = enabled
	u.mu.Unlock()
	return nil
}

func (u *Updater) Events() <-chan platform.UpdaterEvent { return u.events }

func (u *Updater) Close() error {
	u.closeOnce.Do(func() {
		u.cleanup.Call()
		close(u.events)
	})
	return nil
}
