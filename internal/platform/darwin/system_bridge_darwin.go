//go:build darwin && cgo

package darwin

/*
#cgo CFLAGS: -I${SRCDIR}/../../../native/include
#cgo LDFLAGS: -L${SRCDIR}/../../../build/native/darwin/universal -ldaygo_capture -framework Foundation -framework AppKit
#include <stdlib.h>
#include "daygo_system.h"
#include "daygo_status_item.h"
extern void dgSystemEvent(uint32_t kind, int64_t atUnixNS, void *userData);
extern void dgStatusItemAction(uint32_t action, void *userData);
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
	if code := C.dg_system_start(C.DG_SYSTEM_ABI_MAJOR, (C.dg_system_event_callback_v1)(C.dgSystemEvent), nil); code != 0 {
		return fmt.Errorf("system ABI start failed: %d", code)
	}
	return nil
}
func systemStop() { C.dg_system_stop() }

//export dgSystemEvent
func dgSystemEvent(kind C.uint32_t, at C.int64_t, _ unsafe.Pointer) {
	activeSystem.Lock()
	s := activeSystem.value
	activeSystem.Unlock()
	if s != nil {
		s.push(uint32(kind), int64(at))
	}
}
func setStatusItem(state platform.StatusItemState) error {
	title, tooltip, open, pause, quit := C.CString(state.Title), C.CString(state.Tooltip), C.CString(state.OpenLabel), C.CString(state.PauseLabel), C.CString(state.QuitLabel)
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(tooltip))
	defer C.free(unsafe.Pointer(open))
	defer C.free(unsafe.Pointer(pause))
	defer C.free(unsafe.Pointer(quit))
	native := C.dg_status_item_state_v1{visible: C.uint32_t(boolToUint(state.Visible)), pause_enabled: C.uint32_t(boolToUint(state.PauseEnabled)), title: title, tooltip: tooltip, open_label: open, pause_label: pause, quit_label: quit}
	if code := C.dg_status_item_set(C.DG_STATUS_ITEM_ABI_MAJOR, &native, (C.dg_status_item_action_callback_v1)(C.dgStatusItemAction), nil); code != 0 {
		return fmt.Errorf("status item ABI set failed: %d", code)
	}
	return nil
}
func stopStatusItem() { C.dg_status_item_stop() }
func boolToUint(v bool) uint32 {
	if v {
		return 1
	}
	return 0
}

//export dgStatusItemAction
func dgStatusItemAction(action C.uint32_t, _ unsafe.Pointer) {
	activeSystem.Lock()
	s := activeSystem.value
	activeSystem.Unlock()
	if s == nil {
		return
	}
	var kind platform.SystemEventKind
	switch uint32(action) {
	case 1:
		kind = platform.EventStatusItemClick
	case 2:
		kind = platform.EventStatusItemClick
	case 3:
		kind = platform.EventStatusItemClick
	default:
		return
	}
	s.pushAction(kind, uint32(action))
}
