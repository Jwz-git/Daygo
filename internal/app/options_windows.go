//go:build windows

package app

import "github.com/wailsapp/wails/v2/pkg/options/windows"

// platformWindowsOptions keeps Windows-specific host policy out of the
// portable application composition. In particular, the capture helper DLL is
// loaded only from the application directory (or System32 for OS libraries),
// rather than from the current working directory.
func platformWindowsOptions() *windows.Options {
	return &windows.Options{
		Theme:            windows.SystemDefault,
		BackdropType:     windows.Mica,
		Messages:         windows.DefaultMessages(),
		ResizeDebounceMS: 16,
		DLLSearchPaths:   windows.DLLSearchApplicationDir | windows.DLLSearchSystem32,
	}
}
