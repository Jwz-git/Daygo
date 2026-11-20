//go:build windows

package windows

import (
	"fmt"

	winapi "golang.org/x/sys/windows"
)

// Preserve the process DLL policy previously supplied by Wails v2 options.
// Run this before any adapter loads its DLL, and fail the desktop launch if
// the policy cannot be installed. CLI-only builds do not call this function.
func PrepareDesktopHost() error { return prepareDesktopHost(winapi.SetDefaultDllDirectories) }

func prepareDesktopHost(set func(uint32) error) error {
	if err := set(winapi.LOAD_LIBRARY_SEARCH_APPLICATION_DIR | winapi.LOAD_LIBRARY_SEARCH_SYSTEM32); err != nil {
		return fmt.Errorf("configure desktop DLL search: %w", err)
	}
	return nil
}
