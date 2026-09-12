package app

import (
	"context"
	"log"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
)

// maybeAutoStartRecording starts recording at launch when three conditions
// hold: this instance owns capture, screen-recording permission is granted,
// and a routed primary provider exists (no provider means nothing to analyze,
// so recording would only burn disk — the user who configures a provider is
// the user who wants the loop to run).
//
// This is the initial-version behavior the user asked for. Known gaps,
// deliberately accepted for now: there is no "stopped means stay stopped"
// memory — every launch restarts recording until a settings key lands — and
// G-host (the 10-minute background-survival gate) has not run, so quitting
// the app stops recording with the gap backfilled by the 24-hour unbatched
// lookback on the next launch. Denial or absence of any condition is a silent
// skip, never a permission prompt or an error dialog.
func (b *Backend) maybeAutoStartRecording() {
	_, owner := b.instanceOwnership()
	if !owner {
		return
	}
	if state, err := b.recordingPermission(); err != nil || state != string(platform.PermissionGranted) {
		// Unknown or missing permission: the UI's permission flow is the
		// right place to resolve it, not a startup prompt.
		return
	}
	store := b.store()
	if store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	repo := store.Providers()
	routing, err := b.loadRouting(ctx, repo)
	if err != nil || len(routing.Chain) == 0 {
		return
	}
	if _, err := repo.Get(ctx, routing.Chain[0]); err != nil {
		return
	}
	if err := b.SetRecording(true); err != nil {
		log.Printf("auto-start recording: %v", err)
	}
}
