package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/domain"
)

// seedCategory inserts a categories row; cards store names, but the
// aggregation resolves is_idle through this table.
func seedCategory(t *testing.T, store *Store, name string, isIdle bool) {
	t.Helper()
	err := store.Write(context.Background(), "seed category", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO categories (id, name, color_hex, sort_order, is_idle, created_at, updated_at)
			 VALUES (?, ?, '#000000', 0, ?, 0, 0)`,
			"cat-"+name, name, isIdle)
		return err
	})
	if err != nil {
		t.Fatalf("seed category %s: %v", name, err)
	}
}

// seedBatchWithStatus inserts an analysis_batches row in a chosen status.
func seedBatchWithStatus(t *testing.T, store *Store, id int64, status string, startTs, endTs int64) {
	t.Helper()
	err := store.Write(context.Background(), "seed batch with status", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO analysis_batches (id, start_ts, end_ts, status, failure_kind, failure_note, created_at, updated_at)
			 VALUES (?, ?, ?, ?, 'llm_error', 'sanitized note', 0, 0)`,
			id, startTs, endTs, status)
		return err
	})
	if err != nil {
		t.Fatalf("seed batch %d (%s): %v", id, status, err)
	}
}

func TestCategoryMinutesInRange(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	seedBatch(t, store, 1)
	seedBatch(t, store, 2)
	// Coding needs a row; System and Idle are the v2 built-in seeds and Idle
	// already carries is_idle=1, which is exactly what the aggregation must
	// resolve for a card whose category name matches it.
	seedCategory(t, store, "Coding", false)
	ctx := context.Background()
	loc := store.location()

	from, to := window(loc, 10, 0, 11, 0)
	if _, err := store.Cards().ReplaceCardsInRange(ctx, from, to, []domain.CardShell{
		shell("10:00 AM", "10:30 AM", "Coding", "thirty"),
		shell("10:30 AM", "11:00 AM", "Idle", "thirty-more"),
	}, 1); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if err := seedSystemCard(t, store, 2, "10:00 AM", "11:00 AM"); err != nil {
		t.Fatalf("seedSystemCard: %v", err)
	}

	rangeFrom := time.Date(2026, 9, 12, 4, 0, 0, 0, loc)
	rangeTo := time.Date(2026, 9, 13, 4, 0, 0, 0, loc)
	rows, err := store.Cards().CategoryMinutesInRange(ctx, rangeFrom, rangeTo)
	if err != nil {
		t.Fatalf("CategoryMinutesInRange: %v", err)
	}
	want := map[string]CategoryMinutes{
		"Coding": {Name: "Coding", IsIdle: false, Minutes: 30},
		"Idle":   {Name: "Idle", IsIdle: true, Minutes: 30},
		"System": {Name: "System", IsIdle: false, Minutes: 60},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %+v, want %d groups", rows, len(want))
	}
	for _, row := range rows {
		expected, ok := want[row.Name]
		if !ok {
			t.Fatalf("unexpected category %q in %+v", row.Name, rows)
		}
		if row.IsIdle != expected.IsIdle || row.Minutes != expected.Minutes {
			t.Fatalf("row %+v, want %+v", row, expected)
		}
	}

	// A disjoint window aggregates nothing.
	otherFrom := time.Date(2026, 9, 13, 4, 0, 0, 0, loc)
	otherTo := time.Date(2026, 9, 14, 4, 0, 0, 0, loc)
	rows, err = store.Cards().CategoryMinutesInRange(ctx, otherFrom, otherTo)
	if err != nil {
		t.Fatalf("CategoryMinutesInRange(disjoint): %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("disjoint window rows = %+v, want none", rows)
	}
}

func TestFailedBatchesInRange(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	ctx := context.Background()
	loc := store.location()

	dayStart := time.Date(2026, 9, 12, 4, 0, 0, 0, loc).Unix()
	dayEnd := time.Date(2026, 9, 13, 4, 0, 0, 0, loc).Unix()
	hour := int64(3600)
	seedBatchWithStatus(t, store, 1, "failed", dayStart, dayStart+hour)               // in window
	seedBatchWithStatus(t, store, 2, "skipped_short", dayEnd-hour, dayEnd)            // touches end from inside
	seedBatchWithStatus(t, store, 3, "succeeded", dayStart, dayStart+hour)            // not a failure state
	seedBatchWithStatus(t, store, 4, "failed", dayStart-hour, dayStart+hour)          // overlaps start
	seedBatchWithStatus(t, store, 5, "failed_empty", dayStart+10*hour, dayEnd+2*hour) // overlaps end
	seedBatchWithStatus(t, store, 6, "failed", dayEnd, dayEnd+hour)                   // starts exactly at end: outside

	rows, err := store.Cards().FailedBatchesInRange(ctx,
		time.Unix(dayStart, 0), time.Unix(dayEnd, 0))
	if err != nil {
		t.Fatalf("FailedBatchesInRange: %v", err)
	}
	gotIDs := make(map[int64]bool)
	for _, row := range rows {
		gotIDs[row.ID] = true
		if row.FailureKind != "llm_error" || row.FailureNote != "sanitized note" {
			t.Fatalf("row %+v, want kind and note carried through", row)
		}
	}
	for _, id := range []int64{1, 2, 4, 5} {
		if !gotIDs[id] {
			t.Fatalf("batch %d missing from %+v (overlap rule: touches window)", id, gotIDs)
		}
	}
	if gotIDs[3] {
		t.Fatal("succeeded batch reported as failed")
	}
	if gotIDs[6] {
		t.Fatal("batch starting at window end reported as overlapping (left-closed, right-open)")
	}
}
