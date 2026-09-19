//go:build windows

package windows

import (
	"context"
	"fmt"
	"sync"
	"unsafe"

	"github.com/Jwz-git/Daygo/internal/platform"
	"golang.org/x/sys/windows"
)

const monitorInfoFlagPrimary = 0x1

var (
	modUser32               = windows.NewLazySystemDLL("user32.dll")
	procEnumDisplayMonitors = modUser32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW     = modUser32.NewProc("GetMonitorInfoW")
)

type winRect struct {
	Left, Top, Right, Bottom int32
}

// monitorInfoEx mirrors MONITORINFOEXW. CbSize must be set before
// GetMonitorInfoW or the call returns FALSE. SzDevice is CCHDEVICENAME (32).
type monitorInfoEx struct {
	CbSize    uint32
	RcMonitor winRect
	RcWork    winRect
	DwFlags   uint32
	SzDevice  [32]uint16
}

// EnumDisplayMonitors runs the callback synchronously on the calling goroutine,
// so a single package-level callback plus a mutex-guarded accumulator is enough
// and avoids leaking a windows.NewCallback slot per Displays call.
var (
	displayEnumMu       sync.Mutex
	displayEnumResult   []platform.Display
	displayEnumCallback = windows.NewCallback(monitorEnumProc)
)

func monitorEnumProc(hMonitor uintptr, _ uintptr, _ uintptr, _ uintptr) uintptr {
	var info monitorInfoEx
	info.CbSize = uint32(unsafe.Sizeof(info))
	ok, _, _ := procGetMonitorInfoW.Call(hMonitor, uintptr(unsafe.Pointer(&info)))
	if ok != 0 {
		name := windows.UTF16ToString(info.SzDevice[:])
		displayEnumResult = append(displayEnumResult, platform.Display{
			ID:      name,
			Name:    name,
			Width:   int(info.RcMonitor.Right - info.RcMonitor.Left),
			Height:  int(info.RcMonitor.Bottom - info.RcMonitor.Top),
			Primary: info.DwFlags&monitorInfoFlagPrimary != 0,
		})
	}
	return 1 // TRUE: keep enumerating
}

// Displays enumerates attached monitors via EnumDisplayMonitors. ID/Name are the
// GDI device name (e.g. \\.\DISPLAY1); a friendlier product name would need
// EnumDisplayDevices/SetupAPI and is out of this basic slice. Capture still
// targets the system primary display internally, independent of this list.
func (*System) Displays(ctx context.Context) ([]platform.Display, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	displayEnumMu.Lock()
	defer displayEnumMu.Unlock()
	displayEnumResult = nil
	ok, _, callErr := procEnumDisplayMonitors.Call(0, 0, displayEnumCallback, 0)
	if ok == 0 {
		return nil, fmt.Errorf("windows displays: EnumDisplayMonitors: %w", callErr)
	}
	out := make([]platform.Display, len(displayEnumResult))
	copy(out, displayEnumResult)
	displayEnumResult = nil
	return out, nil
}
