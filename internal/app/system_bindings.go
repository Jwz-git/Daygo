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

// SetPermissionRestartArmed arms or disarms the permission-change restart. The
// frontend arms it while the screen-recording permission guidance is visible, so
// that macOS's "Quit & Reopen" (and the guidance's own restart button) fully
// terminate and relaunch the resident agent instead of soft-quitting to the
// background — the only way a freshly granted TCC permission takes effect. It is
// disarmed when the guidance is dismissed so an ordinary Cmd+Q still soft-quits.
func (b *Backend) SetPermissionRestartArmed(armed bool) error {
	if armed {
		b.armPermissionRestart()
	} else {
		b.disarmPermissionRestart()
	}
	return nil
}

// RelaunchForPermission finalizes the active segment and restarts the app so a
// freshly granted screen-recording permission takes effect. It is the guidance
// layer's explicit "restart to apply" action; a resident agent otherwise only
// hides its window on quit and never re-reads the grant.
func (b *Backend) RelaunchForPermission() error {
	requestShutdown := b.shutdownRequest()
	if requestShutdown == nil {
		return apperr.E(apperr.NativeUnavailable, "desktop shell is unavailable", nil)
	}
	b.beginPermissionRestart()
	requestShutdown()
	return nil
}
