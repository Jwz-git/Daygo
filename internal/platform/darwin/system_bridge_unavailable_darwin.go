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
func setStatusItem(platform.StatusItemState) error {
	return errors.New("status item ABI unavailable without cgo")
}
func stopStatusItem() {}
func systemStop()     {}
