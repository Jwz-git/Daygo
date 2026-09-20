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
