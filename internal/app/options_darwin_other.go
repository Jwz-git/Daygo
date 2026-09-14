//go:build !darwin

package app

import "github.com/wailsapp/wails/v2/pkg/options/mac"

// platformMacOptions is a no-op on non-darwin builds; it exists so the
// Mac: field in options.App keeps a stable type across platforms.
func platformMacOptions() *mac.Options { return nil }
