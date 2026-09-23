package app

import (
	"runtime"
	"testing"
)

func TestPlatformWindowChrome(t *testing.T) {
	if got, want := platformFrameless(), runtime.GOOS == "windows"; got != want {
		t.Fatalf("frameless = %v, want %v on %s", got, want, runtime.GOOS)
	}
	if opts := platformWindowsOptions(); runtime.GOOS == "windows" {
		if opts == nil || opts.DisableFramelessWindowDecorations {
			t.Fatal("Windows custom title bar must retain native resize decorations")
		}
	} else if opts != nil {
		t.Fatal("Windows window options must not affect other platforms")
	}
}
