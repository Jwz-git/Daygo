//go:build windows

package windows

import (
	"context"
	"errors"
	"fmt"
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
	policy  platform.ActivationPolicy
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
	case 5:
		return platform.EventScreensaverStart
	case 6:
		return platform.EventScreensaverStop
	case 7:
		return platform.EventDisplaysChanged
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
func (*System) FrontmostApplication(context.Context) (platform.AppInfo, error) {
	return platform.AppInfo{}, errSystemCapabilityUnavailable
}
func (*System) InstalledApplications(ctx context.Context, _ string) ([]platform.AppInfo, error) {
	return installedApplications(ctx)
}
func (s *System) SetActivationPolicy(ctx context.Context, policy platform.ActivationPolicy) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if policy != platform.ActivationRegular && policy != platform.ActivationAccessory && policy != platform.ActivationProhibited {
		return fmt.Errorf("windows activation policy: invalid policy %q", policy)
	}
	// Windows has no process-wide equivalent of NSApplicationActivationPolicy.
	// Daygo's notification-area host already owns the equivalent lifecycle:
	// regular and accessory differ only in whether the Wails window is visible,
	// which remains app-layer state. Remembering the requested policy makes this
	// capability explicit and idempotent instead of reporting it unavailable.
	s.mu.Lock()
	s.policy = policy
	s.mu.Unlock()
	return nil
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
