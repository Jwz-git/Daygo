//go:build windows

package windows

import (
	"errors"
	"testing"
)

func TestDesktopDLLSearchPolicy(t *testing.T) {
	want := uint32(0x200 | 0x800) // application directory + System32
	if err := prepareDesktopHost(func(flags uint32) error {
		if flags != want {
			t.Fatalf("DLL search flags = %#x, want %#x", flags, want)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("anonymous failure")
	if err := prepareDesktopHost(func(uint32) error { return cause }); !errors.Is(err, cause) {
		t.Fatal("host initialization must preserve the native failure")
	}
}
