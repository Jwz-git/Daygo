package app

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	daytime "github.com/Jwz-git/Daygo/internal/timeutil"
)

const (
	// standupBackfillInterval is how often the resident agent re-scans for
	// calendar days that completed since the last sweep. Standup days roll over
	// once at midnight, so an hourly pass catches a newly completed day well
	// within the day it becomes eligible; the startup sweep clears any backlog
	// accumulated while the agent was not running.
	standupBackfillInterval = time.Hour

	// standupBackfillPace is the pause between consecutive day generations so a
	// large first-run backlog does not burst the provider. It mirrors the
	// analysis scheduler's BatchPacing intent at a gentler rate.
	standupBackfillPace = 2 * time.Second

	// maxConsecutiveStandupFailures aborts a sweep once this many days fail in a
	// row: a healthy provider does not fail every day, so a run of failures
	// means the provider is down and the rest of the backlog would only burn
	// attempts. The next tick retries from where this sweep stopped.
	maxConsecutiveStandupFailures = 3

	// standupTodayRefreshInterval is how stale today's recap may get before the
	// sweep regenerates it. Today is still accumulating activity, so unlike a
	// completed day its recap is refreshed on this cadence (checked each hourly
	// tick) rather than generated once. A manual "regenerate" resets the clock,
	// since it updates the entry's generated_at.
	standupTodayRefreshInterval = 4 * time.Hour
)

// runStandupBackfill generates missing standups for past calendar days, then
// re-scans on a ticker for the life of ctx. It runs only on the read-write
// instance (its writes need the same locks the analysis pipeline needs), so it
// is launched from the RW-only startup block alongside the analysis service.
func (b *Backend) runStandupBackfill(ctx context.Context) {
	ticker := time.NewTicker(standupBackfillInterval)
	defer ticker.Stop()
	for {
		if err := b.standupBackfillSweep(ctx); err != nil && ctx.Err() == nil {
			// A sweep-level error (no provider, storage failure, too many
			// consecutive failures) is not fatal: the next tick retries. Log
			// the code only — apperr messages carry no screen content.
			log.Printf("standup backfill: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// standupBackfillSweep generates a standup for every calendar day that has
// activity but no entry yet, from the earliest recorded card up to (but not
// including) today. Completed past days generate once and are never
// overwritten; today, still accumulating activity, is (re)generated whenever
// its recap is missing or older than standupTodayRefreshInterval. Days with no
// user activity are skipped without writing a row. It is the unit under test.
func (b *Backend) standupBackfillSweep(ctx context.Context) error {
	if err := b.requireTimelineWrite(); err != nil {
		// A read-only instance has nothing to do; not an error worth logging.
		return nil
	}
	store := b.store()
	if store == nil {
		return nil
	}
	loc := b.clock.Now().Location()

	if ctx.Err() != nil {
		return nil
	}
	earliest, found, err := store.Cards().EarliestCardStart(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return mapStorageError("standup backfill", err)
	}
	if !found {
		return nil
	}
	existing, err := store.Standup().ExistingDays(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return mapStorageError("standup backfill", err)
	}

	now := b.clock.Now()
	today := daytime.CalendarDay(now, loc)
	cursor, err := daytime.ParseDay(daytime.CalendarDay(earliest, loc), loc)
	if err != nil {
		return mapStorageError("standup backfill", err)
	}

	consecutiveFailures := 0
	for {
		if ctx.Err() != nil {
			return nil
		}
		day := daytime.CalendarDay(cursor, loc)
		// yyyy-MM-dd sorts lexicographically in chronological order, so this
		// stops after today (today is the last day considered).
		if day > today {
			return nil
		}
		cursor = cursor.AddDate(0, 0, 1)
		isToday := day == today

		// Fast path: a completed day that already has a recap is done.
		if !isToday && existing[day] {
			continue
		}
		activity, err := b.dayActivityCards(ctx, day, loc)
		if err != nil {
			return err
		}
		if len(activity) == 0 {
			// No user activity: writing an empty recap would be noise, and
			// re-checking it next sweep is one cheap indexed range query.
			continue
		}
		// Re-read the current entry: for a completed day it guards against a
		// manual save landing mid-sweep (never overwrite); for today it drives
		// the refresh cadence.
		entry, ok, err := store.Standup().Get(ctx, day)
		if err != nil {
			return mapStorageError("standup backfill", err)
		}
		if ok {
			if !isToday {
				continue
			}
			if entry.GeneratedAt.Add(standupTodayRefreshInterval).After(now) {
				// Today's recap is still fresh; refresh it on a later tick.
				continue
			}
			// Today, stale: fall through to regenerate.
		}

		if err := b.paceStandupBackfill(ctx); err != nil {
			return nil
		}
		dayCtx, cancel := context.WithTimeout(ctx, standupTimeout)
		_, err = b.generateRecapFromCards(dayCtx, day, activity)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			var appErr *apperr.Error
			if errors.As(err, &appErr) && appErr.Code == apperr.ProviderNotConfigured {
				// Nothing to generate against yet; wait for the next tick in
				// case the user configures a provider.
				return err
			}
			consecutiveFailures++
			if consecutiveFailures >= maxConsecutiveStandupFailures {
				return err
			}
			log.Printf("standup backfill: skip %s: %v", day, err)
			continue
		}
		consecutiveFailures = 0
	}
}

// paceStandupBackfill sleeps for the inter-day pace, returning early if ctx is
// cancelled so shutdown is not delayed by the pause.
func (b *Backend) paceStandupBackfill(ctx context.Context) error {
	timer := time.NewTimer(standupBackfillPace)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
