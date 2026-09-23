package app

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform/secrets"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// seedCodingCard inserts one live Coding card on the given calendar day, so the
// backfill sweep sees that day as having user activity. A shared succeeded
// batch (id 1) and the Coding category are created on first use.
func seedCodingCard(t *testing.T, b *Backend, day string, hour int) {
	t.Helper()
	store := b.store()
	date, err := time.ParseInLocation("2006-01-02", day, time.Local)
	if err != nil {
		t.Fatalf("parse seed day %q: %v", day, err)
	}
	start := time.Date(date.Year(), date.Month(), date.Day(), hour, 0, 0, 0, time.Local)
	err = store.Write(context.Background(), "seed coding card", func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO categories (id, name, color_hex, sort_order, is_system, is_idle, created_at, updated_at)
			 VALUES ('cat-coding', 'Coding', '#123456', 0, 0, 0, 0, 0)`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO analysis_batches (id, start_ts, end_ts, status, created_at, updated_at)
			 VALUES (1, 0, 0, 'succeeded', 0, 0)`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO timeline_cards (batch_id, day, start, end, start_ts, end_ts, category, title, summary, created_at, updated_at)
			 VALUES (1, ?, '10:00 AM', '11:00 AM', ?, ?, 'Coding', 'c', 's', 0, 0)`,
			day, start.Unix(), start.Add(time.Hour).Unix())
		return err
	})
	if err != nil {
		t.Fatalf("seed coding card %q: %v", day, err)
	}
}

// seedIdleCard inserts one live Idle card on the given day. Idle is a built-in
// excluded category, so a day holding only this must be treated as no activity.
func seedIdleCard(t *testing.T, b *Backend, day string, hour int) {
	t.Helper()
	store := b.store()
	date, err := time.ParseInLocation("2006-01-02", day, time.Local)
	if err != nil {
		t.Fatalf("parse seed day %q: %v", day, err)
	}
	start := time.Date(date.Year(), date.Month(), date.Day(), hour, 0, 0, 0, time.Local)
	err = store.Write(context.Background(), "seed idle card", func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO analysis_batches (id, start_ts, end_ts, status, created_at, updated_at)
			 VALUES (1, 0, 0, 'succeeded', 0, 0)`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO timeline_cards (batch_id, day, start, end, start_ts, end_ts, category, title, summary, created_at, updated_at)
			 VALUES (1, ?, '2:00 PM', '2:30 PM', ?, ?, 'Idle', 'i', 's', 0, 0)`,
			day, start.Unix(), start.Add(30*time.Minute).Unix())
		return err
	})
	if err != nil {
		t.Fatalf("seed idle card %q: %v", day, err)
	}
}

// backfillBackend builds a read-write backend whose clock is fixed at
// 2026-09-12 noon, so "today" is 2026-09-12 and any seeded day before it is a
// completed calendar day the backfill should consider.
func backfillBackend(t *testing.T) (*Backend, *recordingEmitter) {
	t.Helper()
	store := openTestStore(t, t.TempDir(), true)
	backend := newBackend(fixedClock{now: time.Date(2026, 9, 12, 12, 0, 0, 0, time.Local)}, nil, store, true, true)
	emitter := &recordingEmitter{}
	backend.setEventEmitter(emitter)
	backend.setSecrets(secrets.NewFake())
	return backend, emitter
}

// TestStandupBackfillGeneratesMissingActivityDays is the golden path: two
// distinct days have Coding activity, one has only Idle, and one already has a
// stored recap. The sweep must generate exactly the two missing activity days,
// skip the idle-only day, skip the pre-existing day, and never touch today.
func TestStandupBackfillGeneratesMissingActivityDays(t *testing.T) {
	backend, emitter := backfillBackend(t)

	seedCodingCard(t, backend, "2026-09-08", 10)
	seedCodingCard(t, backend, "2026-09-09", 10)
	seedIdleCard(t, backend, "2026-09-10", 14)
	seedCodingCard(t, backend, "2026-09-11", 10)

	// 2026-09-11 already has a recap; it must not be regenerated.
	preexisting := storage.DailyStandupEntry{
		StandupDay:      "2026-09-11",
		HighlightsTitle: "keep me",
		Highlights:      []string{"do not overwrite"},
		TasksTitle:      "next",
		Tasks:           []string{},
		BlockersTitle:   "blockers",
		BlockersBody:    "",
		GeneratedAt:     time.Date(2026, 9, 11, 20, 0, 0, 0, time.Local),
	}
	if err := backend.store().Standup().Upsert(context.Background(), preexisting); err != nil {
		t.Fatalf("seed preexisting recap: %v", err)
	}

	server, requests := standupFixtureServer(t, standupFixtureContent())
	configureStandupProvider(t, backend, server.URL)

	if err := backend.standupBackfillSweep(context.Background()); err != nil {
		t.Fatalf("standupBackfillSweep: %v", err)
	}

	// Only 09-08 and 09-09 are missing-with-activity: two generations.
	if len(*requests) != 2 {
		t.Fatalf("provider requests = %d, want 2 (09-08, 09-09)", len(*requests))
	}
	if emitter.count(EventRecapUpdated) != 2 {
		t.Fatalf("recap:updated count = %d, want 2", emitter.count(EventRecapUpdated))
	}

	for _, day := range []string{"2026-09-08", "2026-09-09"} {
		got, ok, err := backend.store().Standup().Get(context.Background(), day)
		if err != nil {
			t.Fatalf("Get %s: %v", day, err)
		}
		if !ok {
			t.Fatalf("day %s was not backfilled", day)
		}
		if got.HighlightsTitle != "完成事项" {
			t.Fatalf("day %s recap = %+v, want fixture content", day, got)
		}
	}

	// The idle-only day wrote no row.
	if _, ok, _ := backend.store().Standup().Get(context.Background(), "2026-09-10"); ok {
		t.Fatal("idle-only day 2026-09-10 was backfilled; empty days must be skipped")
	}
	// The pre-existing recap is untouched.
	got, ok, err := backend.store().Standup().Get(context.Background(), "2026-09-11")
	if err != nil || !ok {
		t.Fatalf("Get 2026-09-11: ok=%v err=%v", ok, err)
	}
	if got.HighlightsTitle != "keep me" {
		t.Fatalf("pre-existing recap was overwritten: %+v", got)
	}
}

// TestStandupBackfillGeneratesToday proves today's recap is generated when it
// has activity and no entry yet.
func TestStandupBackfillGeneratesToday(t *testing.T) {
	backend, emitter := backfillBackend(t)
	seedCodingCard(t, backend, "2026-09-12", 10) // today, per the fixed clock

	server, requests := standupFixtureServer(t, standupFixtureContent())
	configureStandupProvider(t, backend, server.URL)

	if err := backend.standupBackfillSweep(context.Background()); err != nil {
		t.Fatalf("standupBackfillSweep: %v", err)
	}
	if len(*requests) != 1 {
		t.Fatalf("provider requests = %d, want 1 (today generated)", len(*requests))
	}
	if emitter.count(EventRecapUpdated) != 1 {
		t.Fatalf("recap:updated count = %d, want 1", emitter.count(EventRecapUpdated))
	}
	if _, ok, _ := backend.store().Standup().Get(context.Background(), "2026-09-12"); !ok {
		t.Fatal("today was not generated")
	}
}

// TestStandupBackfillSkipsFreshToday proves a today recap generated within the
// refresh window is not regenerated on the next sweep.
func TestStandupBackfillSkipsFreshToday(t *testing.T) {
	backend, emitter := backfillBackend(t)
	seedCodingCard(t, backend, "2026-09-12", 10)

	// A fresh recap generated at "now" (the fixed clock is 2026-09-12 12:00).
	fresh := storage.DailyStandupEntry{
		StandupDay:      "2026-09-12",
		HighlightsTitle: "fresh",
		Highlights:      []string{"already generated"},
		TasksTitle:      "next",
		Tasks:           []string{},
		BlockersTitle:   "blockers",
		BlockersBody:    "",
		GeneratedAt:     time.Date(2026, 9, 12, 12, 0, 0, 0, time.Local),
	}
	if err := backend.store().Standup().Upsert(context.Background(), fresh); err != nil {
		t.Fatalf("seed fresh recap: %v", err)
	}

	server, requests := standupFixtureServer(t, standupFixtureContent())
	configureStandupProvider(t, backend, server.URL)

	if err := backend.standupBackfillSweep(context.Background()); err != nil {
		t.Fatalf("standupBackfillSweep: %v", err)
	}
	if len(*requests) != 0 {
		t.Fatalf("provider requests = %d, want 0 (today still fresh)", len(*requests))
	}
	if emitter.count(EventRecapUpdated) != 0 {
		t.Fatalf("recap:updated count = %d, want 0", emitter.count(EventRecapUpdated))
	}
	got, _, _ := backend.store().Standup().Get(context.Background(), "2026-09-12")
	if got.HighlightsTitle != "fresh" {
		t.Fatalf("fresh recap was overwritten: %+v", got)
	}
}

// TestStandupBackfillRefreshesStaleToday proves a today recap older than the
// refresh window is regenerated.
func TestStandupBackfillRefreshesStaleToday(t *testing.T) {
	backend, _ := backfillBackend(t)
	seedCodingCard(t, backend, "2026-09-12", 10)

	// Generated 5h before the fixed clock (12:00) — past the 4h refresh window.
	stale := storage.DailyStandupEntry{
		StandupDay:      "2026-09-12",
		HighlightsTitle: "stale",
		Highlights:      []string{"generated this morning"},
		TasksTitle:      "next",
		Tasks:           []string{},
		BlockersTitle:   "blockers",
		BlockersBody:    "",
		GeneratedAt:     time.Date(2026, 9, 12, 7, 0, 0, 0, time.Local),
	}
	if err := backend.store().Standup().Upsert(context.Background(), stale); err != nil {
		t.Fatalf("seed stale recap: %v", err)
	}

	server, requests := standupFixtureServer(t, standupFixtureContent())
	configureStandupProvider(t, backend, server.URL)

	if err := backend.standupBackfillSweep(context.Background()); err != nil {
		t.Fatalf("standupBackfillSweep: %v", err)
	}
	if len(*requests) != 1 {
		t.Fatalf("provider requests = %d, want 1 (stale today regenerated)", len(*requests))
	}
	got, _, _ := backend.store().Standup().Get(context.Background(), "2026-09-12")
	if got.HighlightsTitle != "完成事项" {
		t.Fatalf("stale recap was not regenerated: %+v", got)
	}
}

// TestStandupBackfillSkipsTodayWithoutActivity proves today is not generated
// while it has no user activity yet.
func TestStandupBackfillSkipsTodayWithoutActivity(t *testing.T) {
	backend, _ := backfillBackend(t)
	seedIdleCard(t, backend, "2026-09-12", 9) // only idle: no user activity

	server, requests := standupFixtureServer(t, standupFixtureContent())
	configureStandupProvider(t, backend, server.URL)

	if err := backend.standupBackfillSweep(context.Background()); err != nil {
		t.Fatalf("standupBackfillSweep: %v", err)
	}
	if len(*requests) != 0 {
		t.Fatalf("provider requests = %d, want 0 (today has no activity)", len(*requests))
	}
	if _, ok, _ := backend.store().Standup().Get(context.Background(), "2026-09-12"); ok {
		t.Fatal("today was generated despite no activity")
	}
}

// TestStandupBackfillNoProviderAborts checks the sweep stops on the dedicated
// provider_not_configured code and writes nothing, so the next tick can retry
// once a provider exists.
func TestStandupBackfillNoProviderAborts(t *testing.T) {
	backend, emitter := backfillBackend(t)
	seedCodingCard(t, backend, "2026-09-09", 10)

	// No provider configured.
	err := backend.standupBackfillSweep(context.Background())
	assertAppCode(t, err, apperr.ProviderNotConfigured)

	if emitter.count(EventRecapUpdated) != 0 {
		t.Fatalf("recap:updated count = %d, want 0", emitter.count(EventRecapUpdated))
	}
	if _, ok, _ := backend.store().Standup().Get(context.Background(), "2026-09-09"); ok {
		t.Fatal("a recap was written despite no provider")
	}
}

// TestStandupBackfillCancelledStops proves a cancelled context stops the sweep
// before any generation, so shutdown is not blocked.
func TestStandupBackfillCancelledStops(t *testing.T) {
	backend, emitter := backfillBackend(t)
	seedCodingCard(t, backend, "2026-09-09", 10)

	server, requests := standupFixtureServer(t, standupFixtureContent())
	configureStandupProvider(t, backend, server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	if err := backend.standupBackfillSweep(ctx); err != nil {
		t.Fatalf("standupBackfillSweep on cancelled ctx: %v", err)
	}
	if len(*requests) != 0 {
		t.Fatalf("provider requests = %d, want 0 (cancelled)", len(*requests))
	}
	if emitter.count(EventRecapUpdated) != 0 {
		t.Fatalf("recap:updated count = %d, want 0 (cancelled)", emitter.count(EventRecapUpdated))
	}
}

// TestStandupBackfillEmptyDatabase is a no-op: no cards means no candidate days
// and no error.
func TestStandupBackfillEmptyDatabase(t *testing.T) {
	backend, emitter := backfillBackend(t)

	if err := backend.standupBackfillSweep(context.Background()); err != nil {
		t.Fatalf("standupBackfillSweep on empty db: %v", err)
	}
	if emitter.count(EventRecapUpdated) != 0 {
		t.Fatalf("recap:updated count = %d, want 0", emitter.count(EventRecapUpdated))
	}
}

// TestStandupBackfillReadOnlyInstanceIsNoop proves a read-only second instance
// does nothing: it holds neither lock the writes need.
func TestStandupBackfillReadOnlyInstanceIsNoop(t *testing.T) {
	dir := t.TempDir()
	writerBackendWithStore(t, dir) // holds the write + capture lock
	readerStore := openTestStore(t, dir, true)
	reader := newBackend(fixedClock{now: time.Date(2026, 9, 12, 12, 0, 0, 0, time.Local)}, nil, readerStore, false, false)
	emitter := &recordingEmitter{}
	reader.setEventEmitter(emitter)

	if err := reader.standupBackfillSweep(context.Background()); err != nil {
		t.Fatalf("read-only sweep returned error: %v", err)
	}
	if emitter.count(EventRecapUpdated) != 0 {
		t.Fatalf("read-only instance emitted %d events, want 0", emitter.count(EventRecapUpdated))
	}
}
