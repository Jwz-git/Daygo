//go:build darwin

package app

import "github.com/wailsapp/wails/v2/pkg/options/mac"

// platformMacOptions returns the macOS-specific window options.
//
// TitleBarHidden (not TitleBarHiddenInset): both keep the native traffic lights,
// but HiddenInset also attaches an empty NSToolbar, and on macOS 26 a window
// with a toolbar is drawn with a noticeably larger corner radius than a
// titlebar-only window. Dropping the toolbar is what keeps the window corners
// tight.
func platformMacOptions() *mac.Options {
	return &mac.Options{
		TitleBar:             mac.TitleBarHidden(),
		WebviewIsTransparent: false,
	}
}
