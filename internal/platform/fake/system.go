package fake

import (
	"context"
	"github.com/Jwz-git/Daygo/internal/platform"
	"sync"
	"time"
)

type System struct {
	mu                sync.Mutex
	permission        platform.PermissionState
	notifications     platform.PermissionState
	events            chan platform.SystemEvent
	statusItem        platform.StatusItemState
	activationPolicy  platform.ActivationPolicy
	activationSet     bool
	activationCalls   int
	revealedPaths     []string
	launchAtLogin     bool
	launchAtLoginCall int
	relaunches        int
	// scheduled is the live notification set keyed by ID: a later
	// ScheduleNotification with the same ID replaces the earlier entry, matching
	// the one-shot-with-stable-ID contract the app layer relies on.
	scheduled        map[string]platform.Notification
	scheduleCalls    int
	cancelCalls      int
	lastCancelledIDs []string
	scheduleErr      error
	cancelErr        error
}

// SetNotificationsPermission makes NotificationsPermission report state. The
// default is granted; a test that wants a denied platform sets it explicitly.
func (s *System) SetNotificationsPermission(state platform.PermissionState) {
	s.mu.Lock()
	s.notifications = state
	s.mu.Unlock()
}

// SetScheduleError makes ScheduleNotification fail, standing in for a platform
// without notification authorization.
func (s *System) SetScheduleError(err error) {
	s.mu.Lock()
	s.scheduleErr = err
	s.mu.Unlock()
}

// SetCancelError makes CancelNotifications fail.
func (s *System) SetCancelError(err error) {
	s.mu.Lock()
	s.cancelErr = err
	s.mu.Unlock()
}

// ScheduledNotification returns the live notification with id and whether one
// is currently scheduled. It is how a test asserts the app re-scheduled, left
// the entry alone, or cancelled.
func (s *System) ScheduledNotification(id string) (platform.Notification, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.scheduled[id]
	return n, ok
}

// ScheduledIDs returns the ids of every live notification.
func (s *System) ScheduledIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.scheduled))
	for id := range s.scheduled {
		ids = append(ids, id)
	}
	return ids
}

// ScheduleCalls counts ScheduleNotification calls. A test uses it to prove the
// app skipped a redundant re-schedule rather than merely ending in the same
// notification, which ScheduledNotification alone cannot distinguish.
func (s *System) ScheduleCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scheduleCalls
}

// CancelCalls counts CancelNotifications calls and returns the ids of the last
// one, so a test can assert a disable cancelled the reminder.
func (s *System) CancelCalls() (int, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cancelCalls, append([]string(nil), s.lastCancelledIDs...)
}

func NewSystem() *System {
	return &System{
		permission:    platform.PermissionGranted,
		notifications: platform.PermissionGranted,
		events:        make(chan platform.SystemEvent, 32),
		scheduled:     make(map[string]platform.Notification),
	}
}
func (s *System) Emit(kind platform.SystemEventKind) {
	select {
	case s.events <- platform.SystemEvent{Kind: kind, At: time.Now()}:
	default:
	}
}
func (s *System) Events() <-chan platform.SystemEvent { return s.events }
func (s *System) ScreenRecordingPermission(context.Context) (platform.PermissionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.permission, nil
}
func (s *System) NotificationsPermission(context.Context) (platform.PermissionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.notifications, nil
}
func (s *System) RequestScreenRecordingPermission(context.Context) error          { return nil }
func (s *System) OpenSystemSettings(context.Context, platform.SettingsPane) error { return nil }
func (s *System) Displays(context.Context) ([]platform.Display, error) {
	return []platform.Display{{ID: "primary", Name: "Primary", Width: 1920, Height: 1080, Primary: true}}, nil
}
func (s *System) FrontmostApplication(context.Context) (platform.AppInfo, error) {
	return platform.AppInfo{}, nil
}
func (s *System) InstalledApplications(context.Context, string) ([]platform.AppInfo, error) {
	return []platform.AppInfo{}, nil
}
func (s *System) LaunchAtLogin(context.Context) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.launchAtLogin, nil
}
func (s *System) SetLaunchAtLogin(_ context.Context, enabled bool) error {
	s.mu.Lock()
	s.launchAtLogin = enabled
	s.launchAtLoginCall++
	s.mu.Unlock()
	return nil
}

// LaunchAtLoginState reports the last value passed to SetLaunchAtLogin and how
// many times it was called, so tests can prove the app layer forwarded a
// launch-at-login change to the platform rather than only persisting it.
func (s *System) LaunchAtLoginState() (enabled bool, calls int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.launchAtLogin, s.launchAtLoginCall
}
func (s *System) SetActivationPolicy(_ context.Context, p platform.ActivationPolicy) error {
	s.mu.Lock()
	s.activationPolicy = p
	s.activationSet = true
	s.activationCalls++
	s.mu.Unlock()
	return nil
}

// ActivationPolicy reports the last policy passed to SetActivationPolicy and
// whether it was ever set. It exists for tests exercising lifecycle behavior.
func (s *System) ActivationPolicy() (platform.ActivationPolicy, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activationPolicy, s.activationSet
}

// ActivationPolicyCalls counts SetActivationPolicy calls. Tests use it to prove
// a transition was skipped rather than merely ending in the same policy, which
// ActivationPolicy alone cannot distinguish.
func (s *System) ActivationPolicyCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activationCalls
}
func (s *System) SetStatusItem(_ context.Context, state platform.StatusItemState) error {
	s.mu.Lock()
	s.statusItem = state
	s.mu.Unlock()
	return nil
}
func (s *System) RevealPath(_ context.Context, path string) error {
	s.mu.Lock()
	s.revealedPaths = append(s.revealedPaths, path)
	s.mu.Unlock()
	return nil
}

// RevealedPaths reports the paths passed to RevealPath in call order. It exists
// for tests exercising the menu-bar "open recordings folder" action.
func (s *System) RevealedPaths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.revealedPaths...)
}
func (s *System) ScheduleNotification(_ context.Context, n platform.Notification) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scheduleCalls++
	if s.scheduleErr != nil {
		return s.scheduleErr
	}
	s.scheduled[n.ID] = n
	return nil
}

func (s *System) CancelNotifications(_ context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancelCalls++
	s.lastCancelledIDs = append([]string(nil), ids...)
	if s.cancelErr != nil {
		return s.cancelErr
	}
	for _, id := range ids {
		delete(s.scheduled, id)
	}
	return nil
}

func (s *System) Relaunch(context.Context) error {
	s.mu.Lock()
	s.relaunches++
	s.mu.Unlock()
	return nil
}

// Relaunches counts Relaunch calls so tests can prove the permission-change
// restart scheduled a relaunch rather than only quitting.
func (s *System) Relaunches() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.relaunches
}

var _ platform.System = (*System)(nil)
var _ platform.Relauncher = (*System)(nil)
