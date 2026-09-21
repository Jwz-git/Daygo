package app

import (
	"context"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
)

// GetUpdaterState reports the current update status. A build with no update
// adapter wired (docs/06 §6.7) returns native_unavailable rather than a
// fabricated state, so the frontend can hide the update surface honestly.
func (b *Backend) GetUpdaterState() (UpdaterStateDTO, error) {
	if b.updater == nil {
		return UpdaterStateDTO{}, apperr.E(apperr.NativeUnavailable, "update services are unavailable", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	state, err := b.updater.State(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return UpdaterStateDTO{}, apperr.E(apperr.NativeUnavailable, "update state query timed out", ctx.Err())
		}
		return UpdaterStateDTO{}, apperr.E(apperr.NativeUnavailable, "update state query failed", err)
	}
	dto := UpdaterStateDTO{Automatic: state.Automatic, Checking: state.Checking}
	if state.AvailableVersion != nil {
		version := *state.AvailableVersion
		dto.AvailableVersion = &version
	}
	if state.LastCheckedAt != nil {
		ts := state.LastCheckedAt.Unix()
		dto.LastCheckedAtTs = &ts
	}
	return dto, nil
}

// CheckForUpdates starts an update check. interactive=true is a user-initiated
// check that reports "up to date" too; interactive=false is the scheduled
// background check that only surfaces when an update exists
// (docs/decisions/delivery-auto-update.md §2). Discovery is delivered as the
// update:available event, not this return value.
func (b *Backend) CheckForUpdates(interactive bool) error {
	if b.updater == nil {
		return apperr.E(apperr.NativeUnavailable, "update services are unavailable", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := b.updater.CheckForUpdates(ctx, interactive); err != nil {
		if ctx.Err() != nil {
			return apperr.E(apperr.NativeUnavailable, "update check timed out", ctx.Err())
		}
		return apperr.E(apperr.NativeUnavailable, "update check failed", err)
	}
	return nil
}

// SetAutomaticUpdateChecks changes the platform updater's own scheduler. The
// updater remains the single owner of check timing; Daygo does not run a
// second timer alongside Sparkle or WinSparkle.
func (b *Backend) SetAutomaticUpdateChecks(enabled bool) error {
	if b.updater == nil {
		return apperr.E(apperr.NativeUnavailable, "update services are unavailable", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := b.updater.SetAutomaticChecks(ctx, enabled); err != nil {
		if ctx.Err() != nil {
			return apperr.E(apperr.NativeUnavailable, "update settings timed out", ctx.Err())
		}
		return apperr.E(apperr.NativeUnavailable, "update settings failed", err)
	}
	return nil
}
