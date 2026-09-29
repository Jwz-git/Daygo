//go:build darwin && !cgo

package darwin

import (
	"errors"
	"github.com/Jwz-git/Daygo/internal/platform"
	"sync"
)

var activeSystem struct {
	sync.Mutex
	value *System
}

func systemStart() error { return errors.New("system ABI unavailable without cgo") }
func relaunch() error    { return errors.New("relaunch ABI unavailable without cgo") }
func setStatusItem(platform.StatusItemState) error {
	return errors.New("status item ABI unavailable without cgo")
}
func stopStatusItem() {}
func showStatusMessage(platform.StatusMessage) error {
	return errors.New("status message ABI unavailable without cgo")
}
func setApplicationMenuLabels(platform.ApplicationMenuLabels) error {
	return errors.New("application menu ABI unavailable without cgo")
}
func statusItemAvailable() (bool, error) {
	return false, errors.New("status item ABI unavailable without cgo")
}
func systemStop() {}
func setActivationPolicy(platform.ActivationPolicy) error {
	return errors.New("activation policy ABI unavailable without cgo")
}
func queryScreenRecordingPermission() (platform.PermissionState, error) {
	return platform.PermissionNotDetermined, errors.New("screen recording permission ABI unavailable without cgo")
}
func requestScreenRecordingPermission() error {
	return errors.New("screen recording permission ABI unavailable without cgo")
}
func openSystemSettings(platform.SettingsPane) error {
	return errors.New("open system settings ABI unavailable without cgo")
}
func queryLaunchAtLogin() (bool, error) {
	return false, errors.New("launch-at-login ABI unavailable without cgo")
}
func setLaunchAtLogin(bool) error {
	return errors.New("launch-at-login ABI unavailable without cgo")
}
