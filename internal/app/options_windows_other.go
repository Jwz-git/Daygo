//go:build !windows

package app

import "github.com/wailsapp/wails/v2/pkg/options/windows"

// platformFrameless preserves native window chrome outside Windows.
func platformFrameless() bool { return false }

// platformWindowsOptions is a no-op outside Windows; it keeps the options.App
// construction identical on every build target.
func platformWindowsOptions() *windows.Options { return nil }
