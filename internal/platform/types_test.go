package platform

import (
	"testing"
	"time"
)

func TestValidSegmentPath(t *testing.T) {
	valid := []string{"2026/09/10/segment-0001", "segment-0001", "dir.with-dots/file"}
	for _, value := range valid {
		if !ValidSegmentPath(value) {
			t.Errorf("ValidSegmentPath(%q) = false", value)
		}
	}
	invalid := []string{"", ".", "..", "../escape", "dir/../escape", "/absolute", "dir//file", "dir/./file", `C:\absolute`, `dir\file`}
	for _, value := range invalid {
		if ValidSegmentPath(value) {
			t.Errorf("ValidSegmentPath(%q) = true", value)
		}
	}
}

func TestCaptureConfigCanonicalization(t *testing.T) {
	display := "display-1"
	original := CaptureConfig{
		Interval:              10 * time.Second,
		CaptureHeight:         1080,
		BlockedApplicationIDs: []string{"com.example.b", "com.example.a"},
		SegmentDirectory:      "/owned/by-adapter",
		PreferredDisplayID:    &display,
		ShowsCursor:           true,
		SegmentMaxFrames:      600,
		SegmentMaxDuration:    10 * time.Minute,
	}
	canonical := CanonicalizeCaptureConfig(original)
	original.BlockedApplicationIDs[0] = "mutated"
	display = "mutated"
	if canonical.BlockedApplicationIDs[0] != "com.example.a" || canonical.BlockedApplicationIDs[1] != "com.example.b" {
		t.Fatalf("canonical IDs = %v", canonical.BlockedApplicationIDs)
	}
	if canonical.PreferredDisplayID == nil || *canonical.PreferredDisplayID != "display-1" {
		t.Fatalf("canonical display = %v", canonical.PreferredDisplayID)
	}
	if !CaptureConfigEqual(canonical, CaptureConfig{
		Interval:              10 * time.Second,
		CaptureHeight:         1080,
		BlockedApplicationIDs: []string{"com.example.b", "com.example.a"},
		SegmentDirectory:      "/owned/by-adapter",
		PreferredDisplayID:    stringPointer("display-1"),
		ShowsCursor:           true,
		SegmentMaxFrames:      600,
		SegmentMaxDuration:    10 * time.Minute,
	}) {
		t.Fatal("reordered blocked-ID set was not equivalent")
	}
}

func TestClosedEnums(t *testing.T) {
	if !PermissionGranted.Valid() || !PermissionDenied.Valid() || !PermissionNotDetermined.Valid() || PermissionState("invented").Valid() {
		t.Fatal("permission-state closed set mismatch")
	}
	if !PaneScreenRecording.Valid() || !PaneNotifications.Valid() || !PaneLoginItems.Valid() || SettingsPane("arbitrary_url").Valid() {
		t.Fatal("settings-pane closed set mismatch")
	}
	if !CaptureEventFrame.Valid() || !CaptureEventSegmentClosed.Valid() || CaptureEventKind("other").Valid() {
		t.Fatal("capture-event closed set mismatch")
	}
	if !CaptureIdle.Valid() || !CaptureStarting.Valid() || !CaptureCapturing.Valid() || !CapturePaused.Valid() || CapturePhase("other").Valid() {
		t.Fatal("capture-phase closed set mismatch")
	}
}

func TestPairedSystemEvents(t *testing.T) {
	paired := []SystemEventKind{EventSleep, EventWake, EventScreenLocked, EventScreenUnlocked, EventScreensaverStart, EventScreensaverStop}
	for _, kind := range paired {
		if !kind.Paired() {
			t.Errorf("%q must be paired", kind)
		}
	}
	if EventDisplaysChanged.Paired() || EventDeepLink.Paired() {
		t.Fatal("unpaired event reported paired")
	}
}

func stringPointer(value string) *string { return &value }
