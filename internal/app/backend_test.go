package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type systemStub struct {
	permission    platform.PermissionState
	notifications platform.PermissionState
	permissionErr error
	requestErr    error
	openErr       error
	requested     bool
	opened        platform.SettingsPane
}

func (s *systemStub) ScreenRecordingPermission(context.Context) (platform.PermissionState, error) {
	return s.permission, s.permissionErr
}
func (s *systemStub) NotificationsPermission(context.Context) (platform.PermissionState, error) {
	return s.notifications, nil
}
func (s *systemStub) RequestScreenRecordingPermission(context.Context) error {
	s.requested = true
	return s.requestErr
}
func (s *systemStub) OpenSystemSettings(_ context.Context, pane platform.SettingsPane) error {
	s.opened = pane
	return s.openErr
}
func (*systemStub) Displays(context.Context) ([]platform.Display, error) { return nil, nil }
func (*systemStub) FrontmostApplication(context.Context) (platform.AppInfo, error) {
	return platform.AppInfo{}, nil
}
func (*systemStub) InstalledApplications(context.Context) ([]platform.AppInfo, error) {
	return nil, nil
}
func (*systemStub) LaunchAtLogin(context.Context) (bool, error)                          { return false, nil }
func (*systemStub) SetLaunchAtLogin(context.Context, bool) error                         { return nil }
func (*systemStub) SetActivationPolicy(context.Context, platform.ActivationPolicy) error { return nil }
func (*systemStub) SetStatusItem(context.Context, platform.StatusItemState) error        { return nil }
func (*systemStub) ScheduleNotification(context.Context, platform.Notification) error    { return nil }
func (*systemStub) CancelNotifications(context.Context, []string) error                  { return nil }
func (*systemStub) Events() <-chan platform.SystemEvent                                  { return nil }

func TestGetDayContextCurrentLogicalDay(t *testing.T) {
	loc := mustLocation(t, "America/Los_Angeles")
	now := time.Date(2026, time.March, 8, 2, 30, 0, 0, loc)
	backend := newBackend(fixedClock{now: now}, &systemStub{}, nil, true, true)

	got, err := backend.GetDayContext("")
	if err != nil {
		t.Fatalf("GetDayContext: %v", err)
	}
	if got.Day != "2026-03-07" || got.StandupDay != "2026-03-08" {
		t.Fatalf("day context days = day %q standup %q", got.Day, got.StandupDay)
	}
	if got.DayEndTs-got.DayStartTs != int64((23*time.Hour)/time.Second) {
		t.Fatalf("day window duration = %d seconds, want 23 hours", got.DayEndTs-got.DayStartTs)
	}
	if got.NowTs != now.Unix() || got.TimeZone != "America/Los_Angeles" || got.DayBoundaryHour != 4 {
		t.Fatalf("unexpected day context metadata: %+v", got)
	}
}

func TestGetDayContextExplicitAndInvalid(t *testing.T) {
	loc := mustLocation(t, "Asia/Kolkata")
	backend := newBackend(fixedClock{now: time.Date(2026, 1, 15, 2, 0, 0, 0, loc)}, &systemStub{}, nil, true, true)
	got, err := backend.GetDayContext("2024-02-29")
	if err != nil || got.Day != "2024-02-29" || got.StandupDay != "2024-02-29" {
		t.Fatalf("explicit day = %+v, %v", got, err)
	}
	for _, day := range []string{"today", "2026-2-03", "2026-02-30"} {
		_, err := backend.GetDayContext(day)
		assertAppCode(t, err, apperr.InvalidArgument)
	}
}

func TestM1PermissionBindings(t *testing.T) {
	system := &systemStub{permission: platform.PermissionNotDetermined, notifications: platform.PermissionDenied}
	backend := newBackend(fixedClock{now: time.Now()}, system, nil, true, false)

	permission, err := backend.GetPermissionState()
	if err != nil {
		t.Fatalf("GetPermissionState: %v", err)
	}
	if permission.ScreenRecording != "not_determined" || permission.Notifications != "denied" || !permission.CanRequest {
		t.Fatalf("unexpected permission: %+v", permission)
	}
	recording, err := backend.GetRecordingState()
	if err != nil {
		t.Fatalf("GetRecordingState: %v", err)
	}
	if recording.State != RecordingStateIdle || recording.Permission != "not_determined" || recording.IsCaptureOwner {
		t.Fatalf("unexpected recording state: %+v", recording)
	}

	if err := backend.RequestScreenRecordingPermission(); err != nil || !system.requested {
		t.Fatalf("request permission = %v, requested %t", err, system.requested)
	}
	if err := backend.OpenSystemSettings("screen_recording"); err != nil || system.opened != platform.PaneScreenRecording {
		t.Fatalf("open settings = %v, pane %q", err, system.opened)
	}
}

func TestM1BindingErrorsAreAppErrors(t *testing.T) {
	backend := newBackend(fixedClock{now: time.Now()}, nil, nil, true, true)
	_, err := backend.GetPermissionState()
	assertAppCode(t, err, apperr.NativeUnavailable)
	_, err = backend.GetRecordingState()
	assertAppCode(t, err, apperr.NativeUnavailable)
	assertAppCode(t, backend.RequestScreenRecordingPermission(), apperr.NativeUnavailable)
	assertAppCode(t, backend.OpenSystemSettings("arbitrary-url"), apperr.InvalidArgument)

	system := &systemStub{permission: "invented"}
	backend = newBackend(fixedClock{now: time.Now()}, system, nil, true, true)
	_, err = backend.GetPermissionState()
	assertAppCode(t, err, apperr.Internal)
}

func TestM1PlatformErrorsAreSanitized(t *testing.T) {
	cause := errors.New("private native path and diagnostic detail")
	backend := newBackend(fixedClock{now: time.Now()}, &systemStub{permissionErr: cause}, nil, true, true)
	_, err := backend.GetPermissionState()
	assertAppCode(t, err, apperr.NativeUnavailable)
	if err.Error() == cause.Error() {
		t.Fatal("binding exposed the platform cause")
	}
}

func TestGetCapabilitiesDoesNotAdvertisePlannedFeatures(t *testing.T) {
	backend := newBackend(fixedClock{now: time.Now()}, nil, nil, true, false)
	got, err := backend.GetCapabilities()
	if err != nil {
		t.Fatalf("GetCapabilities: %v", err)
	}
	if len(got.Features) != 1 || got.Features[0] != "settings" {
		t.Fatalf("Features = %v, want only delivered settings shell", got.Features)
	}
	if got.APIRevision != 1 || !got.CanWrite || got.IsCaptureOwner {
		t.Fatalf("unexpected capabilities: %+v", got)
	}
}

func assertAppCode(t *testing.T, err error, want apperr.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %q", want)
	}
	var appError *apperr.Error
	if !errors.As(err, &appError) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	if appError.Code != want {
		t.Fatalf("error code = %q, want %q", appError.Code, want)
	}
}

func mustLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load location %q: %v", name, err)
	}
	return loc
}
