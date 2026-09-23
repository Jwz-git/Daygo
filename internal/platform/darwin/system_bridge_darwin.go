//go:build darwin && cgo

package darwin

/*
#cgo CFLAGS: -I${SRCDIR}/../../../native/include
#cgo LDFLAGS: -L${SRCDIR}/../../../build/native/darwin/universal -ldaygo_capture -framework Foundation -framework AppKit -framework ServiceManagement
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

func queryScreenRecordingPermission() (platform.PermissionState, error) {
	switch code := C.dg_screen_recording_permission_query(); code {
	case C.DG_PERMISSION_GRANTED:
		return platform.PermissionGranted, nil
	case C.DG_PERMISSION_DENIED:
		return platform.PermissionDenied, nil
	case C.DG_PERMISSION_NOT_DETERMINED:
		return platform.PermissionNotDetermined, nil
	default:
		return "", fmt.Errorf("screen recording permission query failed: %d", int32(code))
	}
}

func requestScreenRecordingPermission() error {
	if code := C.dg_screen_recording_permission_request(); code != 0 {
		return fmt.Errorf("screen recording permission request failed: %d", int32(code))
	}
	return nil
}

func openSystemSettings(pane platform.SettingsPane) error {
	var native C.uint32_t
	switch pane {
	case platform.PaneScreenRecording:
		native = C.uint32_t(C.DG_SETTINGS_PANE_SCREEN_RECORDING)
	case platform.PaneNotifications:
		native = C.uint32_t(C.DG_SETTINGS_PANE_NOTIFICATIONS)
	case platform.PaneLoginItems:
		native = C.uint32_t(C.DG_SETTINGS_PANE_LOGIN_ITEMS)
	default:
		return fmt.Errorf("open system settings: unknown pane %q", pane)
	}
	if code := C.dg_open_system_settings(native); code != 0 {
		return fmt.Errorf("open system settings ABI failed: %d", int32(code))
	}
	return nil
}

func queryLaunchAtLogin() (bool, error) {
	switch code := C.dg_launch_at_login_query(); code {
	case C.DG_LAUNCH_AT_LOGIN_ENABLED:
		return true, nil
	case C.DG_LAUNCH_AT_LOGIN_REQUIRES_APPROVAL, C.DG_LAUNCH_AT_LOGIN_NOT_REGISTERED, C.DG_LAUNCH_AT_LOGIN_NOT_FOUND, C.DG_LAUNCH_AT_LOGIN_UNSUPPORTED:
		return false, nil
	default:
		return false, fmt.Errorf("launch-at-login query failed: %d", int32(code))
	}
}

func setLaunchAtLogin(enabled bool) error {
	var flag C.uint32_t
	if enabled {
		flag = 1
	}
	if code := C.dg_launch_at_login_set(flag); code != 0 {
		return fmt.Errorf("launch-at-login set failed: %d", int32(code))
	}
	return nil
}

func setActivationPolicy(p platform.ActivationPolicy) error {
	var policy C.uint32_t
	switch p {
	case platform.ActivationRegular:
		policy = C.uint32_t(C.DG_ACTIVATION_REGULAR)
	case platform.ActivationAccessory:
		policy = C.uint32_t(C.DG_ACTIVATION_ACCESSORY)
	case platform.ActivationProhibited:
		policy = C.uint32_t(C.DG_ACTIVATION_PROHIBITED)
	default:
		return fmt.Errorf("set activation policy: unknown policy %q", p)
	}
	if code := C.dg_activation_policy_set(policy); code != 0 {
		return fmt.Errorf("activation policy ABI set failed: %d", code)
	}
	return nil
}

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
	title := C.CString(state.Title)
	tooltip := C.CString(state.Tooltip)
	open := C.CString(state.OpenLabel)
	recordings := C.CString(state.RecordingsLabel)
	quit := C.CString(state.QuitLabel)
	pauseMenu := C.CString(state.PauseMenuLabel)
	pause15 := C.CString(state.Pause15Label)
	pause30 := C.CString(state.Pause30Label)
	pause60 := C.CString(state.Pause60Label)
	pauseIndefinite := C.CString(state.PauseIndefiniteLabel)
	primary := C.CString(state.PrimaryActionLabel)
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(tooltip))
	defer C.free(unsafe.Pointer(open))
	defer C.free(unsafe.Pointer(recordings))
	defer C.free(unsafe.Pointer(quit))
	defer C.free(unsafe.Pointer(pauseMenu))
	defer C.free(unsafe.Pointer(pause15))
	defer C.free(unsafe.Pointer(pause30))
	defer C.free(unsafe.Pointer(pause60))
	defer C.free(unsafe.Pointer(pauseIndefinite))
	defer C.free(unsafe.Pointer(primary))
	native := C.dg_status_item_state_v1{
		visible:                 C.uint32_t(boolToUint(state.Visible)),
		pause_durations_enabled: C.uint32_t(boolToUint(state.PauseDurationsEnabled)),
		primary_action_enabled:  C.uint32_t(boolToUint(state.PrimaryActionEnabled)),
		title:                   title,
		tooltip:                 tooltip,
		open_label:              open,
		recordings_label:        recordings,
		quit_label:              quit,
		pause_menu_label:        pauseMenu,
		pause_15_label:          pause15,
		pause_30_label:          pause30,
		pause_60_label:          pause60,
		pause_indefinite_label:  pauseIndefinite,
		primary_action_label:    primary,
	}
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
	// statusActionID is the single source of truth for which codes are valid;
	// an unrecognized code yields a nil ID and is dropped by pushAction.
	if statusActionID(uint32(action)) == nil {
		return
	}
	s.pushAction(platform.EventStatusItemClick, uint32(action))
}
