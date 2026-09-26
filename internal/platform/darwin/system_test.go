//go:build darwin

package darwin

import (
	"testing"

	"github.com/Jwz-git/Daygo/internal/platform"
)

func TestSystemEventKindMapsApplicationActivation(t *testing.T) {
	if got := systemEventKind(8); got != platform.EventApplicationActivated {
		t.Fatalf("systemEventKind(8) = %q, want %q", got, platform.EventApplicationActivated)
	}
}

func TestSystemEventKindMapsApplicationVisibility(t *testing.T) {
	for input, want := range map[uint32]platform.SystemEventKind{9: platform.EventApplicationHidden, 10: platform.EventApplicationUnhidden} {
		if got := systemEventKind(input); got != want || !got.Paired() {
			t.Fatalf("visibility event %d maps to %q", input, got)
		}
	}
}
