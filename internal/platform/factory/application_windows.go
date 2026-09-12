//go:build windows

package factory

import (
	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/windows"
)

func NewApplicationInspector() platform.ApplicationInspector {
	return windows.NewApplicationInspector()
}
