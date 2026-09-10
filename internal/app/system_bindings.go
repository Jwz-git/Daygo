package app

import (
	"context"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform"
)

// RequestScreenRecordingPermission asks macOS to show the screen-recording
// permission flow. The resulting state is read separately; this method does not
// assume that displaying the prompt means permission was granted.
func (b *Backend) RequestScreenRecordingPermission() error {
	if b.system == nil {
		return apperr.E(apperr.NativeUnavailable, "platform services are unavailable", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := b.system.RequestScreenRecordingPermission(ctx); err != nil {
		if ctx.Err() != nil {
			return apperr.E(apperr.NativeUnavailable, "platform request timed out", ctx.Err())
		}
		return apperr.E(apperr.NativeUnavailable, "platform request failed", err)
	}
	return nil
}

// OpenSystemSettings opens one of the explicitly allowed system panes. It never
// accepts an arbitrary URL.
func (b *Backend) OpenSystemSettings(pane string) error {
	value := platform.SettingsPane(pane)
	if !value.Valid() {
		return apperr.E(apperr.InvalidArgument, "unknown system settings pane", nil)
	}
	if b.system == nil {
		return apperr.E(apperr.NativeUnavailable, "platform services are unavailable", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := b.system.OpenSystemSettings(ctx, value); err != nil {
		if ctx.Err() != nil {
			return apperr.E(apperr.NativeUnavailable, "platform request timed out", ctx.Err())
		}
		return apperr.E(apperr.NativeUnavailable, "platform request failed", err)
	}
	return nil
}
