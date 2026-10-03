//go:build darwin && !cgo

package darwin

import (
	"context"

	"github.com/Jwz-git/Daygo/internal/platform"
)

// Without cgo there is no UserNotifications bridge; the schedulers treat the
// capability as absent and stay silent.

func scheduleNotification(context.Context, platform.Notification) error {
	return platform.ErrCapabilityUnavailable
}

func cancelNotifications(context.Context, []string) error {
	return platform.ErrCapabilityUnavailable
}

func notificationPermission(context.Context) (platform.PermissionState, error) {
	return platform.PermissionNotDetermined, nil
}
