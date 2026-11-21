package app

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/Jwz-git/Daygo/internal/recordinglocation"
)

func (backend *Backend) startCaptureOwnerServices(ctx context.Context, dir, recordingsRoot string, rootErr error, desktopOS string) {
	if writable, owner := backend.instanceOwnership(); !writable || !owner {
		return
	}

	store := backend.store()
	// Complete a previously committed file cleanup. A failed cleanup leaves
	// the new root authoritative and is retried on the next launch.
	if desktopOS == "windows" {
		if err := recordinglocation.Finish(ctx, store.Settings()); err != nil {
			log.Printf("recording location cleanup pending")
		}
	}

	// A missing external volume must not make Reconcile soft-delete its
	// committed frames. The pipeline can resume on the next app start.
	rootAvailable := rootErr == nil
	if rootAvailable {
		info, statErr := os.Stat(recordingsRoot)
		if statErr == nil {
			rootAvailable = info.IsDir()
		} else if !os.IsNotExist(statErr) || recordingsRoot != filepath.Join(dir, "recordings") {
			rootAvailable = false
		}
	}

	// Crash recovery first (docs/modules/recording): settle pending
	// capture intents against the filesystem before the analysis
	// scheduler looks at frames. A commit the process died before
	// making is completed here; an intent whose file never landed is
	// dropped. Failures are logged, not fatal — the UI still works on
	// committed data.
	reconcileCtx, reconcileCancel := context.WithTimeout(ctx, 30*time.Second)
	if rootAvailable {
		if err := store.Captures().Reconcile(reconcileCtx, recordingsRoot); err != nil {
			log.Printf("capture reconcile: %v", err)
		}
	}
	reconcileCancel()

	if rootAvailable {
		if _, err := startAnalysis(ctx, backend, store, recordingsRoot); err != nil {
			// Analysis failing to start must not take the shell down: the UI
			// still renders stored cards, and diagnostics reports the gap.
			log.Printf("analysis pipeline unavailable: %v", err)
		}
	}

	// Backfill standups for calendar days that completed while the agent
	// was not running, then re-scan on a ticker for newly completed days.
	// Owned by ctx like the analysis pipeline, so shutdown stops it.
	go backend.runStandupBackfill(ctx)

	// Keep notifications in sync with journal and plan settings.
	go backend.runJournalReminder(ctx)
	go backend.runPlanReminder(ctx)
}
