//go:build windows && !cgo

package windows

import (
	"errors"
	"github.com/Jwz-git/Daygo/internal/platform"
	"sync"
)

var activeSystem struct {
	sync.Mutex
	value *System
}

func systemStart() error { return errors.New("windows system event ABI unavailable without cgo") }
func systemStop()        {}
func setStatusItem(platform.StatusItemState) error {
	return errors.New("windows status item ABI unavailable without cgo")
}
func stopStatusItem() {}
