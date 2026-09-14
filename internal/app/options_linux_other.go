//go:build !linux

package app

import "github.com/wailsapp/wails/v2/pkg/options/linux"

// platformLinuxOptions is a no-op on non-linux builds; it exists so the
// Linux: field in options.App keeps a stable type across platforms.
func platformLinuxOptions() *linux.Options { return nil }
