package fake

import (
	"context"
	"github.com/Jwz-git/Daygo/internal/platform"
	"sync"
	"time"
)

type System struct {
	mu         sync.Mutex
	permission platform.PermissionState
	events     chan platform.SystemEvent
	statusItem platform.StatusItemState
}

func NewSystem() *System {
	return &System{permission: platform.PermissionGranted, events: make(chan platform.SystemEvent, 32)}
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
	return platform.PermissionGranted, nil
}
func (s *System) RequestScreenRecordingPermission(context.Context) error          { return nil }
func (s *System) OpenSystemSettings(context.Context, platform.SettingsPane) error { return nil }
func (s *System) Displays(context.Context) ([]platform.Display, error) {
	return []platform.Display{{ID: "primary", Name: "Primary", Width: 1920, Height: 1080, Primary: true}}, nil
}
func (s *System) FrontmostApplication(context.Context) (platform.AppInfo, error) {
	return platform.AppInfo{}, nil
}
func (s *System) InstalledApplications(context.Context) ([]platform.AppInfo, error) {
	return []platform.AppInfo{}, nil
}
func (s *System) LaunchAtLogin(context.Context) (bool, error)                          { return false, nil }
func (s *System) SetLaunchAtLogin(context.Context, bool) error                         { return nil }
func (s *System) SetActivationPolicy(context.Context, platform.ActivationPolicy) error { return nil }
func (s *System) SetStatusItem(_ context.Context, state platform.StatusItemState) error {
	s.mu.Lock()
	s.statusItem = state
	s.mu.Unlock()
	return nil
}
func (s *System) ScheduleNotification(context.Context, platform.Notification) error { return nil }
func (s *System) CancelNotifications(context.Context, []string) error               { return nil }

var _ platform.System = (*System)(nil)
