//go:build windows

package windows

import (
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
)

func TestSystemEventKind(t *testing.T) {
	tests := map[uint32]platform.SystemEventKind{
		1: platform.EventSleep,
		2: platform.EventWake,
		3: platform.EventScreenLocked,
		4: platform.EventScreenUnlocked,
		5: "",
	}
	for input, want := range tests {
		if got := systemEventKind(input); got != want {
			t.Errorf("systemEventKind(%d) = %q, want %q", input, got, want)
		}
	}
}

func TestStatusActionID(t *testing.T) {
	tests := map[uint32]string{
		1: "open",
		2: "toggle_pause",
		3: "quit",
	}
	for input, want := range tests {
		got := statusActionID(input)
		if got == nil || *got != want {
			t.Errorf("statusActionID(%d) = %v, want %q", input, got, want)
		}
	}
	if got := statusActionID(99); got != nil {
		t.Errorf("statusActionID(99) = %q, want nil", *got)
	}
}

func TestSystemPushActionPublishesStatusItemEvent(t *testing.T) {
	system := &System{events: make(chan platform.SystemEvent, 1)}
	system.pushAction(2)

	select {
	case event := <-system.events:
		if event.Kind != platform.EventStatusItemClick {
			t.Fatalf("event kind = %q, want %q", event.Kind, platform.EventStatusItemClick)
		}
		if event.Data.StatusItemID == nil || *event.Data.StatusItemID != "toggle_pause" {
			t.Fatalf("status item id = %v, want toggle_pause", event.Data.StatusItemID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for status-item event")
	}
}
