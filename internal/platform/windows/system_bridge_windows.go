//go:build windows && cgo

package windows

/*
#cgo CFLAGS: -I${SRCDIR}/../../../native/include
#cgo LDFLAGS: -lwtsapi32 -lshell32
#include <stdlib.h>
#include "daygo_system.h"
#include "daygo_status_item.h"
extern void dgWindowsSystemEvent(uint32_t kind, int64_t atUnixNS, void *userData);
extern void dgWindowsStatusItemAction(uint32_t action, void *userData);
*/
import "C"

import (
	"fmt"
	"github.com/Jwz-git/Daygo/internal/platform"
	"sync"
	"unsafe"
)

var activeSystem struct {
	sync.Mutex
	value *System
}

func systemStart() error {
	if code := C.dg_system_start(C.DG_SYSTEM_ABI_MAJOR, (C.dg_system_event_callback_v1)(C.dgWindowsSystemEvent), nil); code != 0 {
		return fmt.Errorf("windows system event ABI start failed: %d", int32(code))
	}
	return nil
}

func systemStop() { C.dg_system_stop() }

func setStatusItem(state platform.StatusItemState) error {
	title := C.CString(state.Title)
	tooltip := C.CString(state.Tooltip)
	open := C.CString(state.OpenLabel)
	pause := C.CString(state.PauseLabel)
	quit := C.CString(state.QuitLabel)
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(tooltip))
	defer C.free(unsafe.Pointer(open))
	defer C.free(unsafe.Pointer(pause))
	defer C.free(unsafe.Pointer(quit))

	native := C.dg_status_item_state_v1{
		visible:       C.uint32_t(boolToUint(state.Visible)),
		pause_enabled: C.uint32_t(boolToUint(state.PauseEnabled)),
		title:         title,
		tooltip:       tooltip,
		open_label:    open,
		pause_label:   pause,
		quit_label:    quit,
	}
	if code := C.dg_status_item_set(
		C.DG_STATUS_ITEM_ABI_MAJOR,
		&native,
		(C.dg_status_item_action_callback_v1)(C.dgWindowsStatusItemAction),
		nil,
	); code != 0 {
		return fmt.Errorf("windows status item ABI set failed: %d", int32(code))
	}
	return nil
}

func stopStatusItem() { C.dg_status_item_stop() }

func boolToUint(value bool) uint32 {
	if value {
		return 1
	}
	return 0
}

//export dgWindowsSystemEvent
func dgWindowsSystemEvent(kind C.uint32_t, at C.int64_t, _ unsafe.Pointer) {
	activeSystem.Lock()
	system := activeSystem.value
	activeSystem.Unlock()
	if system != nil {
		system.push(uint32(kind), int64(at))
	}
}

//export dgWindowsStatusItemAction
func dgWindowsStatusItemAction(action C.uint32_t, _ unsafe.Pointer) {
	activeSystem.Lock()
	system := activeSystem.value
	activeSystem.Unlock()
	if system != nil {
		system.pushAction(uint32(action))
	}
}
