//go:build daygo_desktop

package app

import "testing"

func TestPlatformWindowChrome(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "windows"} {
		opts := desktopWindowOptions(goos)
		if opts.Frameless != (goos == "windows") {
			t.Errorf("%s: unexpected frameless setting: %v", goos, opts.Frameless)
		}
		if goos == "windows" && opts.Windows.DisableFramelessWindowDecorations {
			t.Error("Windows custom title bar must retain native resize decorations")
		}
	}
}
