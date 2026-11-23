package app

import "testing"

func TestResidentReleaseRequiresCurrentRequestAndRestorableState(t *testing.T) {
	var release windowRelease
	snapshot := WindowResumeDTO{Route: "timeline", Day: "2026-11-23", CardID: 7,
		Player: &WindowPlayerDTO{Progress: 32.5, Rate: 60, Started: true, Playing: true, Expanded: true}}
	first := release.begin()
	if release.accept(first, snapshot) {
		t.Fatal("accepted before requesting snapshot")
	}
	release.cancel()
	if release.request(first) {
		t.Fatal("timer survived reopen")
	}
	second := release.begin()
	if second == first || !release.request(second) {
		t.Fatal("missing fresh request")
	}
	if release.accept(first, snapshot) {
		t.Fatal("accepted stale reply")
	}
	bad := snapshot
	bad.Day = "2026-02-31"
	if release.accept(second, bad) {
		t.Fatal("accepted malformed reply")
	}
	if !release.accept(second, snapshot) || release.accept(second, snapshot) {
		t.Fatal("reply must be accepted once")
	}
	third := release.begin()
	release.request(third)
	release.cancel()
	if release.accept(third, snapshot) {
		t.Fatal("late reply closed a reopened window")
	}
}
