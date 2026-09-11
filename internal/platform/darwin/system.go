//go:build darwin

package darwin

import (
	"context"
	"github.com/Jwz-git/Daygo/internal/platform"
	"sync"
	"time"
)

type System struct {
	mu      sync.Mutex
	events  chan platform.SystemEvent
	started bool
}

func NewSystem() *System {
	s := &System{events: make(chan platform.SystemEvent, 32)}
	_ = s.start()
	return s
}

func (s *System) start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	activeSystem.Lock()
	activeSystem.value = s
	activeSystem.Unlock()
	if err := systemStart(); err != nil {
		activeSystem.Lock()
		activeSystem.value = nil
		activeSystem.Unlock()
		return err
	}
	s.started = true
	return nil
}
func (s *System) Events() <-chan platform.SystemEvent { return s.events }
func (s *System) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		systemStop()
		activeSystem.Lock()
		if activeSystem.value == s {
			activeSystem.value = nil
		}
		activeSystem.Unlock()
		s.started = false
	}
	close(s.events)
}
func (s *System) push(kind uint32, ns int64) {
	select {
	case s.events <- platform.SystemEvent{Kind: systemEventKind(kind), At: time.Unix(0, ns)}:
	default:
	}
}
func (s *System) pushAction(kind platform.SystemEventKind, action uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case s.events <- platform.SystemEvent{Kind: kind, At: time.Now(), Data: platform.SystemEventData{StatusItemID: statusActionID(action)}}:
	default:
	}
}

func statusActionID(action uint32) *string {
	value := ""
	switch action {
	case 1:
		value = "open"
	case 2:
		value = "toggle_pause"
	case 3:
		value = "quit"
	default:
		return nil
	}
	return &value
}
func systemEventKind(k uint32) platform.SystemEventKind {
	switch k {
	case 1:
		return platform.EventSleep
	case 2:
		return platform.EventWake
	case 3:
		return platform.EventScreenLocked
	case 4:
		return platform.EventScreenUnlocked
	case 5:
		return platform.EventScreensaverStart
	case 6:
		return platform.EventScreensaverStop
	case 7:
		return platform.EventDisplaysChanged
	}
	return ""
}
func (s *System) ScreenRecordingPermission(context.Context) (platform.PermissionState, error) {
	return platform.PermissionNotDetermined, nil
}
func (s *System) NotificationsPermission(context.Context) (platform.PermissionState, error) {
	return platform.PermissionNotDetermined, nil
}
func (s *System) RequestScreenRecordingPermission(context.Context) error          { return nil }
func (s *System) OpenSystemSettings(context.Context, platform.SettingsPane) error { return nil }
func (s *System) Displays(context.Context) ([]platform.Display, error)            { return nil, nil }
func (s *System) FrontmostApplication(context.Context) (platform.AppInfo, error) {
	return platform.AppInfo{}, nil
}
func (s *System) InstalledApplications(context.Context) ([]platform.AppInfo, error)    { return nil, nil }
func (s *System) LaunchAtLogin(context.Context) (bool, error)                          { return false, nil }
func (s *System) SetLaunchAtLogin(context.Context, bool) error                         { return nil }
func (s *System) SetActivationPolicy(context.Context, platform.ActivationPolicy) error { return nil }
func (s *System) SetStatusItem(_ context.Context, state platform.StatusItemState) error {
	return setStatusItem(state)
}
func (s *System) ScheduleNotification(context.Context, platform.Notification) error { return nil }
func (s *System) CancelNotifications(context.Context, []string) error               { return nil }

var _ platform.System = (*System)(nil)
