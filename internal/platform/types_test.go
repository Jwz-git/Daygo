package platform

import "testing"

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

func TestClosedEnums(t *testing.T) {
	if !PermissionGranted.Valid() || !PermissionDenied.Valid() || !PermissionNotDetermined.Valid() || PermissionState("invented").Valid() {
		t.Fatal("permission-state closed set mismatch")
	}
	if !PaneScreenRecording.Valid() || !PaneNotifications.Valid() || !PaneLoginItems.Valid() || SettingsPane("arbitrary_url").Valid() {
		t.Fatal("settings-pane closed set mismatch")
	}
	if !CaptureImageJPEG.Valid() || CaptureImageFormat("png").Valid() {
		t.Fatal("capture-image-format closed set mismatch")
	}
	if !CaptureWritten.Valid() || !CaptureBlocked.Valid() || CaptureOutcome("other").Valid() {
		t.Fatal("capture-outcome closed set mismatch")
	}
	validCaptureErrors := []CaptureErrorCode{
		CaptureInvalidArgument,
		CaptureABIMismatch,
		CaptureUnsupported,
		CapturePermissionDenied,
		CaptureNoDisplay,
		CaptureTimeout,
		CaptureIO,
		CapturePrivacyUnsupported,
		CaptureNative,
	}
	for _, code := range validCaptureErrors {
		if !code.Valid() {
			t.Errorf("capture error code %q is not valid", code)
		}
	}
	if CaptureErrorCode("invented").Valid() {
		t.Fatal("unknown capture error code is valid")
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
