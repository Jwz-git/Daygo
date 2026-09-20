//go:build windows

package windows

import (
	"context"
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

func TestSetActivationPolicyUsesWindowsEquivalentSemantics(t *testing.T) {
	system := &System{}
	for _, policy := range []platform.ActivationPolicy{
		platform.ActivationRegular,
		platform.ActivationAccessory,
		platform.ActivationProhibited,
	} {
		if err := system.SetActivationPolicy(context.Background(), policy); err != nil {
			t.Fatalf("SetActivationPolicy(%q): %v", policy, err)
		}
		if system.policy != policy {
			t.Fatalf("policy = %q, want %q", system.policy, policy)
		}
	}
	if err := system.SetActivationPolicy(context.Background(), "invalid"); err == nil {
		t.Fatal("invalid policy succeeded")
	}
}

func TestSystemEventKindIncludesScreensaverAndDisplays(t *testing.T) {
	for input, want := range map[uint32]platform.SystemEventKind{
		5: platform.EventScreensaverStart,
		6: platform.EventScreensaverStop,
		7: platform.EventDisplaysChanged,
	} {
		if got := systemEventKind(input); got != want {
			t.Fatalf("systemEventKind(%d) = %q, want %q", input, got, want)
		}
	}
}

func TestStatusActionID(t *testing.T) {
	tests := map[uint32]string{
		1: "open",
		2: "toggle_pause",
		3: "quit",
		4: "open_recordings",
		6: "pause_indefinite",
		7: "pause_15",
		8: "pause_30",
		9: "pause_60",
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
