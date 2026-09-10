package app

import (
	"context"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform"
	daytime "github.com/Jwz-git/Daygo/internal/timeutil"
)

const (
	apiRevision = 1
	// TODO(m1): inject from build info once the signing pipeline exists.
	appVersion = "0.0.0"
)

// Clock makes GetDayContext deterministic without giving the binding layer a
// second source of timezone truth.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Backend is the Wails-bound M1 surface. It coordinates pure Go policies and
// platform ports; it contains no native implementation itself.
type Backend struct {
	clock          Clock
	system         platform.System
	canWrite       bool
	isCaptureOwner bool
}

// NewBackend wires the M1 binding surface. system may be nil while the native
// adapter is unavailable; the permission methods then return native_unavailable
// instead of pretending a platform result exists.
func NewBackend(system platform.System) *Backend {
	return newBackend(systemClock{}, system, true, true)
}

func newBackend(clock Clock, system platform.System, canWrite, isCaptureOwner bool) *Backend {
	return &Backend{
		clock:          clock,
		system:         system,
		canWrite:       canWrite,
		isCaptureOwner: isCaptureOwner,
	}
}

// GetCapabilities reports only features that are actually usable. M1 does not
// expose planned M2-M4 feature flags as fake availability.
func (b *Backend) GetCapabilities() (CapabilitiesDTO, error) {
	return CapabilitiesDTO{
		CanWrite:       b.canWrite,
		IsCaptureOwner: b.isCaptureOwner,
		Features:       []string{"settings"},
		AppVersion:     appVersion,
		APIRevision:    apiRevision,
	}, nil
}

// GetDayContext resolves an empty day to the current logical day. A non-empty
// value must be strict yyyy-MM-dd; aliases such as "today" are rejected.
func (b *Backend) GetDayContext(day string) (DayContextDTO, error) {
	now := b.clock.Now()
	loc := now.Location()
	if loc == nil {
		return DayContextDTO{}, apperr.E(apperr.Internal, "local time zone is unavailable", nil)
	}
	if day == "" {
		day = daytime.LogicalDay(now, loc)
	}

	start, end, err := daytime.DayWindow(day, loc)
	if err != nil {
		return DayContextDTO{}, apperr.E(apperr.InvalidArgument, "day must use yyyy-MM-dd", err)
	}
	return DayContextDTO{
		Day:             day,
		StandupDay:      daytime.CalendarDay(now, loc),
		DayStartTs:      start.Unix(),
		DayEndTs:        end.Unix(),
		NowTs:           now.Unix(),
		TimeZone:        loc.String(),
		DayBoundaryHour: daytime.BoundaryHour,
	}, nil
}

// GetRecordingState is truthful even before capture is implemented: M1 starts
// idle and reports the real permission if a platform System is connected.
func (b *Backend) GetRecordingState() (RecordingStateDTO, error) {
	permission, err := b.recordingPermission()
	if err != nil {
		return RecordingStateDTO{}, err
	}
	return RecordingStateDTO{
		State:          RecordingStateIdle,
		Permission:     permission,
		IsCaptureOwner: b.isCaptureOwner,
	}, nil
}

// recordingPermission is GetRecordingState's single query: system unavailability
// is a legitimate answer here (state becomes unknown), not an error.
func (b *Backend) recordingPermission() (string, error) {
	if b.system == nil {
		return "", apperr.E(apperr.NativeUnavailable, "platform services are unavailable", nil)
	}
	return b.permissionState("screen recording", b.system.ScreenRecordingPermission)
}

// GetPermissionState returns the currently observed platform permissions.
func (b *Backend) GetPermissionState() (PermissionDTO, error) {
	if b.system == nil {
		return PermissionDTO{}, apperr.E(apperr.NativeUnavailable, "platform services are unavailable", nil)
	}
	screenRecording, err := b.permissionState("screen recording", b.system.ScreenRecordingPermission)
	if err != nil {
		return PermissionDTO{}, err
	}
	notifications, err := b.permissionState("notifications", b.system.NotificationsPermission)
	if err != nil {
		return PermissionDTO{}, err
	}
	return PermissionDTO{
		ScreenRecording: screenRecording,
		Notifications:   notifications,
		CanRequest:      screenRecording == string(platform.PermissionNotDetermined),
	}, nil
}

// permissionState calls one platform permission query under the binding-layer
// timeout, validating the returned state and mapping failures to apperr.
func (b *Backend) permissionState(op string, query func(ctx context.Context) (platform.PermissionState, error)) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	permission, err := query(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return "", apperr.E(apperr.NativeUnavailable, "platform request timed out", ctx.Err())
		}
		return "", apperr.E(apperr.NativeUnavailable, op+" permission query failed", err)
	}
	if !permission.Valid() {
		return "", apperr.E(apperr.Internal, "platform returned an invalid "+op+" permission state", nil)
	}
	return string(permission), nil
}
