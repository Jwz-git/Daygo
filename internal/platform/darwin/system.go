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
	closed  bool
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
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	started := s.started
	s.started = false
	close(s.events)
	s.mu.Unlock()
	if started {
		systemStop()
		activeSystem.Lock()
		if activeSystem.value == s {
			activeSystem.value = nil
		}
		activeSystem.Unlock()
	}
	stopStatusItem()
}
func (s *System) push(kind uint32, ns int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	event := platform.SystemEvent{Kind: systemEventKind(kind), At: time.Unix(0, ns)}
	select {
	case s.events <- event:
	default:
		// A terminal shutdown supersedes pending ordinary events. Never block an
		// AppKit callback, and never silently discard the termination intent.
		if event.Kind == platform.EventSystemShutdown {
			select {
			case <-s.events:
			default:
			}
			s.events <- event
		}
	}
}
func (s *System) pushAction(kind platform.SystemEventKind, action uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
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
	case 8:
		return platform.EventApplicationActivated
	case 9:
		return platform.EventApplicationHidden
	case 10:
		return platform.EventApplicationUnhidden
	case 11:
		return platform.EventSystemShutdown
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
func (s *System) LaunchAtLogin(context.Context) (bool, error) { return queryLaunchAtLogin() }
func (s *System) SetLaunchAtLogin(_ context.Context, enabled bool) error {
	return setLaunchAtLogin(enabled)
}
func (s *System) SetActivationPolicy(_ context.Context, p platform.ActivationPolicy) error {
	return setActivationPolicy(p)
}
func (s *System) SetStatusItem(_ context.Context, state platform.StatusItemState) error {
	return setStatusItem(state)
}
func (s *System) StatusItemAvailable(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	started := s.started && !s.closed
	s.mu.Unlock()
	if !started {
		return false, nil
	}
	return statusItemAvailable()
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

// Relaunch schedules a fresh instance to start once this process has exited so
// a newly granted screen-recording (TCC) permission — cached by macOS at launch
// — takes effect and the new instance can reclaim the write/capture locks.
func (s *System) Relaunch(context.Context) error { return relaunch() }

var (
	_ platform.System     = (*System)(nil)
	_ platform.Relauncher = (*System)(nil)
)
