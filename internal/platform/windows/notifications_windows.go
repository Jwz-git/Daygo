//go:build windows

package windows

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"github.com/Jwz-git/Daygo/internal/platform"
	xwindows "golang.org/x/sys/windows"
)

const notificationAppID = "io.github.jwz-git.Daygo"
const notificationCLSID = "{EF69D18B-ED0F-4B85-9FDB-1A9458027B0C}"
const notificationABIMajor = 1

type nativeNotificationDriver struct {
	dll                                                 *xwindows.LazyDLL
	handle                                              uintptr
	permissionProc, scheduleProc, cancelProc, closeProc *xwindows.LazyProc
}

func newNativeNotifications() (*notificationClient, error) {
	n := &notificationClient{}
	executable, err := os.Executable()
	if err != nil {
		return n, fmt.Errorf("windows notifications: executable unavailable")
	}
	dll := xwindows.NewLazyDLL(filepath.Join(filepath.Dir(executable), "daygo_windows_native.dll"))
	if err := dll.Load(); err != nil {
		return n, fmt.Errorf("windows notifications: native DLL unavailable: %w", platform.ErrCapabilityUnavailable)
	}
	d := &nativeNotificationDriver{dll: dll,
		permissionProc: dll.NewProc("dg_notification_permission"),
		scheduleProc:   dll.NewProc("dg_notification_schedule"),
		cancelProc:     dll.NewProc("dg_notification_cancel"),
		closeProc:      dll.NewProc("dg_notification_close"),
	}
	open := dll.NewProc("dg_notification_open")
	for _, proc := range []*xwindows.LazyProc{open, d.permissionProc, d.scheduleProc, d.cancelProc, d.closeProc} {
		if err := proc.Find(); err != nil {
			return n, fmt.Errorf("windows notifications: native ABI unavailable: %w", platform.ErrCapabilityUnavailable)
		}
	}
	appID, _ := xwindows.UTF16PtrFromString(notificationAppID)
	clsid, _ := xwindows.UTF16PtrFromString(notificationCLSID)
	path, err := xwindows.UTF16PtrFromString(executable)
	if err != nil {
		return n, fmt.Errorf("windows notifications: invalid executable")
	}
	var nativeError int32
	status, _, _ := open.Call(notificationABIMajor, uintptr(unsafe.Pointer(appID)),
		uintptr(unsafe.Pointer(clsid)), uintptr(unsafe.Pointer(path)),
		uintptr(unsafe.Pointer(&d.handle)), uintptr(unsafe.Pointer(&nativeError)))
	runtime.KeepAlive(appID)
	runtime.KeepAlive(clsid)
	runtime.KeepAlive(path)
	if err := notificationNativeError("initialize", status, nativeError); err != nil {
		return n, err
	}
	if d.handle == 0 {
		return n, fmt.Errorf("windows notifications: native ABI returned no session")
	}
	n.driver = d
	return n, nil
}

func notificationNativeError(operation string, status uintptr, code int32) error {
	switch status {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("windows notification %s: %w", operation, platform.ErrCapabilityUnavailable)
	default:
		return fmt.Errorf("windows notification %s: status %d (HRESULT 0x%08x)", operation, status, uint32(code))
	}
}

func (d *nativeNotificationDriver) permission() (platform.PermissionState, error) {
	var permission uint32
	var nativeError int32
	status, _, _ := d.permissionProc.Call(notificationABIMajor, d.handle, uintptr(unsafe.Pointer(&permission)), uintptr(unsafe.Pointer(&nativeError)))
	if err := notificationNativeError("permission", status, nativeError); err != nil {
		return "", err
	}
	switch permission {
	case 1:
		return platform.PermissionGranted, nil
	case 2:
		return platform.PermissionDenied, nil
	default:
		return "", fmt.Errorf("windows notification permission: invalid native state")
	}
}

func (d *nativeNotificationDriver) schedule(n platform.Notification) error {
	id, _ := xwindows.UTF16PtrFromString(n.ID)
	token, _ := xwindows.UTF16PtrFromString(notificationToken(n.ID))
	title, _ := xwindows.UTF16PtrFromString(n.Title)
	body, _ := xwindows.UTF16PtrFromString(n.Body)
	var ticks int64
	if n.DeliverAt != nil {
		ticks = (n.DeliverAt.Unix()+11644473600)*10000000 + int64(n.DeliverAt.Nanosecond()/100)
	}
	var nativeError int32
	status, _, _ := d.scheduleProc.Call(notificationABIMajor, d.handle, uintptr(unsafe.Pointer(id)), uintptr(unsafe.Pointer(token)),
		uintptr(unsafe.Pointer(title)), uintptr(unsafe.Pointer(body)), uintptr(ticks), uintptr(unsafe.Pointer(&nativeError)))
	runtime.KeepAlive(id)
	runtime.KeepAlive(token)
	runtime.KeepAlive(title)
	runtime.KeepAlive(body)
	return notificationNativeError("schedule", status, nativeError)
}

func (d *nativeNotificationDriver) cancel(value string) error {
	id, _ := xwindows.UTF16PtrFromString(value)
	token, _ := xwindows.UTF16PtrFromString(notificationToken(value))
	var nativeError int32
	status, _, _ := d.cancelProc.Call(notificationABIMajor, d.handle, uintptr(unsafe.Pointer(id)), uintptr(unsafe.Pointer(token)), uintptr(unsafe.Pointer(&nativeError)))
	runtime.KeepAlive(id)
	runtime.KeepAlive(token)
	return notificationNativeError("cancel", status, nativeError)
}
func (d *nativeNotificationDriver) close() { d.closeProc.Call(d.handle); d.handle = 0 }
