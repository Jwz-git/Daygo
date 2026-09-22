package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

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
