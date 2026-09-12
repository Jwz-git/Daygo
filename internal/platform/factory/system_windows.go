//go:build windows

package factory

import (
	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/windows"
)

func NewSystem() platform.System {
	system, err := windows.NewSystem()
	if err != nil {
		return nil
	}
	return system
}
