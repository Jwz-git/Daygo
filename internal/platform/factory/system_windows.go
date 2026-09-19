//go:build windows

package factory

import (
	"log"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/windows"
)

// NewSystem returns nil when the Windows adapter cannot start, matching the
// non-darwin builds whose composition root already treats a nil System as
// "no OS integration". The error is logged rather than swallowed so a failed
// tray/power-event window does not vanish without a trace.
func NewSystem() platform.System {
	system, err := windows.NewSystem()
	if err != nil {
		log.Printf("platform/factory: windows system adapter unavailable: %v", err)
		return nil
	}
	return system
}
