//go:build darwin

package darwin

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
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
	case 4:
		value = "open_recordings"
	case 6:
		value = "pause_indefinite"
	case 7:
		value = "pause_15"
	case 8:
		value = "pause_30"
	case 9:
		value = "pause_60"
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
	return queryScreenRecordingPermission()
}
func (s *System) NotificationsPermission(context.Context) (platform.PermissionState, error) {
	return platform.PermissionNotDetermined, nil
}
func (s *System) RequestScreenRecordingPermission(context.Context) error {
	return requestScreenRecordingPermission()
}
func (s *System) OpenSystemSettings(_ context.Context, pane platform.SettingsPane) error {
	return openSystemSettings(pane)
}
func (s *System) Displays(context.Context) ([]platform.Display, error) { return nil, nil }
func (s *System) FrontmostApplication(context.Context) (platform.AppInfo, error) {
	return platform.AppInfo{}, nil
}
func (s *System) InstalledApplications(ctx context.Context, language string) ([]platform.AppInfo, error) {
	return listApplications(ctx, language)
}
func (s *System) LaunchAtLogin(context.Context) (bool, error)  { return false, nil }
func (s *System) SetLaunchAtLogin(context.Context, bool) error { return nil }
func (s *System) SetActivationPolicy(_ context.Context, p platform.ActivationPolicy) error {
	return setActivationPolicy(p)
}
func (s *System) SetStatusItem(_ context.Context, state platform.StatusItemState) error {
	return setStatusItem(state)
}

// RevealPath opens the path in Finder via /usr/bin/open. open hands the path to
// LaunchServices and exits, so Run waits only for that dispatch, not for the
// window; its exit code still surfaces a missing path as an error.
func (s *System) RevealPath(ctx context.Context, path string) error {
	if err := exec.CommandContext(ctx, "/usr/bin/open", path).Run(); err != nil {
		return fmt.Errorf("reveal path in finder: %w", err)
	}
	return nil
}
func (s *System) ScheduleNotification(context.Context, platform.Notification) error { return nil }
func (s *System) CancelNotifications(context.Context, []string) error               { return nil }

var _ platform.System = (*System)(nil)
