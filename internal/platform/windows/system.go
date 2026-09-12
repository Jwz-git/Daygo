//go:build windows

package windows

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
)

var errSystemCapabilityUnavailable = errors.New("windows system capability is unavailable")

// System adapts the Windows event ABI to the cross-platform System port. This
// slice implements power and session events only; unrelated methods fail
// explicitly until their native capability is added.
type System struct {
	mu      sync.Mutex
	events  chan platform.SystemEvent
	started bool
}

func NewSystem() (*System, error) {
	system := &System{events: make(chan platform.SystemEvent, 32)}
	if err := system.start(); err != nil {
		close(system.events)
		return nil, err
	}
	return system, nil
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
		if activeSystem.value == s {
			activeSystem.value = nil
		}
		activeSystem.Unlock()
		return err
	}
	s.started = true
	return nil
}

func (s *System) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return
	}
	stopStatusItem()
	systemStop()
	activeSystem.Lock()
	if activeSystem.value == s {
		activeSystem.value = nil
	}
	activeSystem.Unlock()
	s.started = false
	close(s.events)
}

func (s *System) Events() <-chan platform.SystemEvent { return s.events }

func (s *System) push(kind uint32, unixNS int64) {
	eventKind := systemEventKind(kind)
	if eventKind == "" {
		return
	}
	select {
	case s.events <- platform.SystemEvent{Kind: eventKind, At: time.Unix(0, unixNS)}:
	default:
	}
}

func (s *System) pushAction(action uint32) {
	id := statusActionID(action)
	if id == nil {
		return
	}
	select {
	case s.events <- platform.SystemEvent{
		Kind: platform.EventStatusItemClick,
		At:   time.Now(),
		Data: platform.SystemEventData{StatusItemID: id},
	}:
	default:
	}
}

func statusActionID(action uint32) *string {
	var value string
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

func systemEventKind(kind uint32) platform.SystemEventKind {
	switch kind {
	case 1:
		return platform.EventSleep
	case 2:
		return platform.EventWake
	case 3:
		return platform.EventScreenLocked
	case 4:
		return platform.EventScreenUnlocked
	default:
		return ""
	}
}

func (*System) ScreenRecordingPermission(context.Context) (platform.PermissionState, error) {
	// Windows desktop capture has no macOS-style TCC screen-recording prompt.
	return platform.PermissionGranted, nil
}
func (*System) RequestScreenRecordingPermission(context.Context) error { return nil }

func (*System) NotificationsPermission(context.Context) (platform.PermissionState, error) {
	return "", errSystemCapabilityUnavailable
}
func (*System) OpenSystemSettings(context.Context, platform.SettingsPane) error {
	return errSystemCapabilityUnavailable
}
func (*System) Displays(context.Context) ([]platform.Display, error) {
	return nil, errSystemCapabilityUnavailable
}
func (*System) FrontmostApplication(context.Context) (platform.AppInfo, error) {
	return platform.AppInfo{}, errSystemCapabilityUnavailable
}
func (*System) InstalledApplications(context.Context) ([]platform.AppInfo, error) {
	return nil, errSystemCapabilityUnavailable
}
func (*System) LaunchAtLogin(context.Context) (bool, error) {
	return false, errSystemCapabilityUnavailable
}
func (*System) SetLaunchAtLogin(context.Context, bool) error {
	return errSystemCapabilityUnavailable
}
func (*System) SetActivationPolicy(context.Context, platform.ActivationPolicy) error {
	return errSystemCapabilityUnavailable
}
func (*System) SetStatusItem(_ context.Context, state platform.StatusItemState) error {
	return setStatusItem(state)
}
func (*System) ScheduleNotification(context.Context, platform.Notification) error {
	return errSystemCapabilityUnavailable
}
func (*System) CancelNotifications(context.Context, []string) error {
	return errSystemCapabilityUnavailable
}

var _ platform.System = (*System)(nil)
