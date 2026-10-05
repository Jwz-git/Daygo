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
	updateSession
	dll *xwindows.LazyDLL

	checkUI       *xwindows.LazyProc
	checkSilent   *xwindows.LazyProc
	setAutomatic  *xwindows.LazyProc
	getAutomatic  *xwindows.LazyProc
	cleanup       *xwindows.LazyProc
	callbackAddrs []uintptr
	closeOnce     sync.Once
	initOnce      sync.Once
}

var _ platform.Updater = (*Updater)(nil)
var _ platform.UpdateInstallCoordinator = (*Updater)(nil)

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
		updateSession: updateSession{events: make(chan platform.UpdaterEvent, 8)},
		dll:           dll,
		checkUI:       dll.NewProc("win_sparkle_check_update_with_ui"),
		checkSilent:   dll.NewProc("win_sparkle_check_update_without_ui"),
		setAutomatic:  dll.NewProc("win_sparkle_set_automatic_check_for_updates"),
		getAutomatic:  dll.NewProc("win_sparkle_get_automatic_check_for_updates"),
		cleanup:       dll.NewProc("win_sparkle_cleanup"),
	}
	u.launch = func(payload string) error {
		return launchUpdateInstaller(payload, filepath.Dir(executable))
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
		u.reportFound()
		return 0
	}
	canShutdown := func() uintptr {
		if u.canShutdown() {
			return 1
		}
		return 0
	}
	shutdown := func() uintptr {
		u.requestShutdown()
		return 0
	}
	interrupted := func() uintptr {
		u.interruptInstall()
		return 0
	}
	launch := func(path uintptr) uintptr {
		if path != 0 && u.launchInstaller(xwindows.UTF16PtrToString((*uint16)(unsafe.Pointer(path)))) {
			return 1 // Handled; never fall back to WinSparkle's default launch.
		}
		return ^uintptr(0) // WINSPARKLE_RETURN_ERROR (-1).
	}
	u.callbackAddrs = []uintptr{
		xwindows.NewCallbackCDecl(done), xwindows.NewCallbackCDecl(found),
		xwindows.NewCallbackCDecl(canShutdown), xwindows.NewCallbackCDecl(shutdown),
		xwindows.NewCallbackCDecl(interrupted), xwindows.NewCallbackCDecl(launch),
	}
	u.dll.NewProc("win_sparkle_set_did_not_find_update_callback").Call(u.callbackAddrs[0])
	u.dll.NewProc("win_sparkle_set_did_find_update_callback").Call(u.callbackAddrs[1])
	u.dll.NewProc("win_sparkle_set_can_shutdown_callback").Call(u.callbackAddrs[2])
	u.dll.NewProc("win_sparkle_set_shutdown_request_callback").Call(u.callbackAddrs[3])
	u.dll.NewProc("win_sparkle_set_error_callback").Call(u.callbackAddrs[4])
	u.dll.NewProc("win_sparkle_set_update_cancelled_callback").Call(u.callbackAddrs[4])
	u.dll.NewProc("win_sparkle_set_update_dismissed_callback").Call(u.callbackAddrs[4])
	u.dll.NewProc("win_sparkle_set_user_run_installer_callback").Call(u.callbackAddrs[5])
}

func updateInstallerArguments(directory string) string {
	// NSIS requires /D= last and unquoted, including paths containing spaces.
	return "/DAYGO_UPDATE /D=" + directory
}

func launchUpdateInstaller(payload, directory string) error {
	file, err := xwindows.UTF16PtrFromString(payload)
	if err != nil {
		return fmt.Errorf("encode update installer: %w", err)
	}
	args, err := xwindows.UTF16PtrFromString(updateInstallerArguments(directory))
	if err != nil {
		return fmt.Errorf("encode update arguments: %w", err)
	}
	if err := xwindows.ShellExecute(0, nil, file, args, nil, xwindows.SW_SHOWNORMAL); err != nil {
		return fmt.Errorf("launch update installer: %w", err)
	}
	return nil
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
	u.available = nil
	u.lastChecked = &now
	u.mu.Unlock()
	if interactive {
		u.checkUI.Call()
	} else {
		u.checkSilent.Call()
	}
	return nil
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

func (u *Updater) Close() error {
	u.closeOnce.Do(func() {
		u.closeState()
		u.cleanup.Call()
	})
	return nil
}
