package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestNextStandupActivitySkipsEmptyDaysAndClipsMidnight(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Kathmandu")
	ctx := t.Context()
	seedBatch(t, store, 1)
	loc := store.location()
	afterGap := time.Date(2026, 9, 9, 0, 0, 0, 0, loc)
	err := store.Write(ctx, "seed anonymous standup activity", func(ctx context.Context, tx *sql.Tx) error {
		spans := []struct {
			start, end time.Time
			category   string
			deleted    bool
		}{
			{afterGap.Add(-72 * time.Hour), afterGap.Add(-71 * time.Hour), "Idle", false},
			{afterGap.Add(-48 * time.Hour), afterGap.Add(-47 * time.Hour), "Work", true},
			{afterGap.Add(-3 * time.Hour), afterGap.Add(-time.Hour), "Work", false},
			{afterGap.Add(-10 * time.Minute), afterGap.Add(20 * time.Minute), "Work", false},
		}
		for _, span := range spans {
			if _, err := tx.ExecContext(ctx, `INSERT INTO timeline_cards
				(batch_id, day, start, end, start_ts, end_ts, category, title, summary, is_deleted, created_at, updated_at)
				VALUES (1, '2026-09-06', '0:00', '0:00', ?, ?, ?, 'fixture', 'fixture', ?, 0, 0)`,
				span.start.Unix(), span.end.Unix(), span.category, span.deleted); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	to := afterGap.AddDate(0, 0, 1)
	for _, tc := range []struct {
		start time.Time
		want  time.Time
	}{
		{afterGap.Add(-96 * time.Hour), afterGap.Add(-3 * time.Hour)},
		{afterGap, afterGap},
		{afterGap.Add(20 * time.Minute), time.Time{}},
	} {
		got, found, err := store.Cards().NextStandupActivity(ctx, tc.start, to)
		if err != nil || found != !tc.want.IsZero() || !got.Equal(tc.want) {
			t.Fatalf("start %s: got %s, %v, %v, want %s", tc.start, got, found, err, tc.want)
		}
	}
}

func TestExistingDaysReturnsStoredStandupDays(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	ctx := context.Background()

	empty, err := store.Standup().ExistingDays(ctx)
	if err != nil {
		t.Fatalf("ExistingDays on empty: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("empty db returned %d days", len(empty))
	}

	for _, day := range []string{"2026-09-08", "2026-09-10"} {
		if err := store.Standup().Upsert(ctx, DailyStandupEntry{
			StandupDay:      day,
			HighlightsTitle: "t",
			Highlights:      []string{"h"},
			TasksTitle:      "n",
			Tasks:           []string{},
			BlockersTitle:   "b",
			BlockersBody:    "",
			GeneratedAt:     time.Unix(1000, 0),
		}); err != nil {
			t.Fatalf("Upsert %s: %v", day, err)
		}
	}

	days, err := store.Standup().ExistingDays(ctx)
	if err != nil {
		t.Fatalf("ExistingDays: %v", err)
	}
	if len(days) != 2 || !days["2026-09-08"] || !days["2026-09-10"] {
		t.Fatalf("days = %v, want {2026-09-08, 2026-09-10}", days)
	}
	if days["2026-09-09"] {
		t.Fatal("unseeded day reported as existing")
	}
}

func TestEarliestCardStartEmptyAndPopulated(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	ctx := context.Background()
	seedBatch(t, store, 1)
	loc := store.location()

	if _, found, err := store.Cards().EarliestCardStart(ctx); err != nil {
		t.Fatalf("EarliestCardStart on empty: %v", err)
	} else if found {
		t.Fatal("empty db reported an earliest card")
	}

	earliest := time.Date(2026, 9, 8, 9, 0, 0, 0, loc)
	insert := func(day string, start time.Time, deleted int) {
		err := store.Write(ctx, "seed card", func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO timeline_cards (batch_id, day, start, end, start_ts, end_ts, category, title, summary, is_deleted, created_at, updated_at)
				 VALUES (1, ?, '9:00 AM', '10:00 AM', ?, ?, 'Coding', 'c', 's', ?, 0, 0)`,
				day, start.Unix(), start.Add(time.Hour).Unix(), deleted)
			return err
		})
		if err != nil {
			t.Fatalf("insert card %s: %v", day, err)
		}
	}
	// A soft-deleted card earlier than every live card must be ignored.
	insert("2026-09-07", time.Date(2026, 9, 7, 8, 0, 0, 0, loc), 1)
	insert("2026-09-09", time.Date(2026, 9, 9, 10, 0, 0, 0, loc), 0)
	insert("2026-09-08", earliest, 0)

	got, found, err := store.Cards().EarliestCardStart(ctx)
	if err != nil {
		t.Fatalf("EarliestCardStart: %v", err)
	}
	if !found {
		t.Fatal("populated db reported no earliest card")
	}
	if !got.Equal(earliest) {
		t.Fatalf("earliest = %v, want %v (live cards only)", got, earliest)
	}
}
