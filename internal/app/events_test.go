package app

import "testing"

func TestEventNameContract(t *testing.T) {
	want := []EventName{
		"timeline:updated",
		"journal:updated",
		"goal:updated",
		"settings:changed",
		"recording:state",
		"capabilities:changed",
		"permission:changed",
		"batch:progress",
		"batch:failed",
		"recording:warning",
		"update:available",
	}

	got := EventNames()
	if len(got) != len(want) {
		t.Fatalf("EventNames length = %d, want %d", len(got), len(want))
	}
	seen := make(map[EventName]bool, len(got))
	for i, event := range got {
		if event != want[i] {
			t.Errorf("EventNames()[%d] = %q, want %q", i, event, want[i])
		}
		if seen[event] {
			t.Errorf("duplicate event name %q", event)
		}
		seen[event] = true
	}

	got[0] = "mutated"
	if EventNames()[0] != EventTimelineUpdated {
		t.Fatal("EventNames exposed mutable package state")
	}
}
