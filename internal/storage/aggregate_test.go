package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/timeutil"
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

func TestCrossFourAMMinutesBelongToEachWindowOnce(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	seedBatch(t, store, 1)
	seedCategory(t, store, "Coding", false)
	ctx := context.Background()
	loc := store.Location()
	from := time.Date(2026, 9, 13, 3, 30, 0, 0, loc)
	to := time.Date(2026, 9, 13, 4, 30, 0, 0, loc)
	if _, err := store.Cards().ReplaceCardsInRange(ctx, from, to, []domain.CardShell{
		shell("3:30 AM", "4:30 AM", "Coding", "boundary"),
	}, 1); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		day        string
		start, end time.Time
	}{
		{"2026-09-12", time.Date(2026, 9, 12, 4, 0, 0, 0, loc), time.Date(2026, 9, 13, 4, 0, 0, 0, loc)},
		{"2026-09-13", time.Date(2026, 9, 13, 4, 0, 0, 0, loc), time.Date(2026, 9, 14, 4, 0, 0, 0, loc)},
	} {
		cards, err := store.Cards().CardsForDay(ctx, tc.day)
		if err != nil || len(cards) != 1 || cards[0].Title != "boundary" {
			t.Fatalf("%s cards = %+v, err = %v", tc.day, cards, err)
		}
		minutes, err := store.Cards().CategoryMinutesInRange(ctx, tc.start, tc.end)
		if err != nil || len(minutes) != 1 || minutes[0].Minutes != 30 {
			t.Fatalf("%s minutes = %+v, err = %v; want 30", tc.day, minutes, err)
		}
		spans, err := store.Cards().CardSpansInRange(ctx, tc.start, tc.end)
		if err != nil || len(spans) != 1 || spans[0].StartTs != max(from.Unix(), tc.start.Unix()) || spans[0].EndTs != min(to.Unix(), tc.end.Unix()) || spans[0].Day != tc.day {
			t.Fatalf("%s spans = %+v, err = %v", tc.day, spans, err)
		}
	}
	spans, err := store.Cards().CardSpansInRange(ctx, time.Date(2026, 9, 12, 4, 0, 0, 0, loc), time.Date(2026, 9, 14, 4, 0, 0, 0, loc))
	if err != nil || len(spans) != 2 || spans[0].Day != "2026-09-12" || spans[1].Day != "2026-09-13" || spans[0].EndTs != spans[1].StartTs {
		t.Fatalf("two-day spans = %+v, err = %v; want two adjoining slices", spans, err)
	}
	total, err := store.Cards().TotalMinutesTracked(ctx, time.Date(2026, 9, 12, 4, 0, 0, 0, loc), time.Date(2026, 9, 13, 4, 0, 0, 0, loc))
	if err != nil || total != 30 {
		t.Fatalf("first day tracked = %v, err = %v; want 30", total, err)
	}
}

func TestCrossFourAMWindowInDSTAndFractionalZones(t *testing.T) {
	for _, tc := range []struct {
		zone             string
		year, month, day int
	}{
		{"America/New_York", 2026, 11, 1}, // fall-back day
		{"Asia/Kolkata", 2026, 9, 13},     // UTC+05:30
		{"Asia/Kathmandu", 2026, 9, 13},   // UTC+05:45
	} {
		t.Run(tc.zone, func(t *testing.T) {
			store := openWriterAt(t, newDir(t), tc.zone)
			seedBatch(t, store, 1)
			ctx := context.Background()
			loc := store.Location()
			from := time.Date(tc.year, time.Month(tc.month), tc.day, 3, 30, 0, 0, loc)
			to := time.Date(tc.year, time.Month(tc.month), tc.day, 4, 30, 0, 0, loc)
			if _, err := store.Cards().ReplaceCardsInRange(ctx, from, to, []domain.CardShell{
				shell("3:30 AM", "4:30 AM", "Coding", "boundary"),
			}, 1); err != nil {
				t.Fatal(err)
			}
			for _, at := range []time.Time{from, to} {
				day := timeutil.LogicalDay(at, loc)
				start, end, err := timeutil.DayWindow(day, loc)
				if err != nil {
					t.Fatal(err)
				}
				rows, err := store.Cards().CategoryMinutesInRange(ctx, start, end)
				if err != nil || len(rows) != 1 || rows[0].Minutes != 30 {
					t.Fatalf("%s: rows = %+v, err = %v; want 30 minutes", day, rows, err)
				}
			}
		})
	}
}

func TestCardSpansInRange(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	seedBatch(t, store, 1)
	seedCategory(t, store, "Coding", false)
	ctx := context.Background()
	loc := store.location()

	from, to := window(loc, 10, 0, 11, 0)
	if _, err := store.Cards().ReplaceCardsInRange(ctx, from, to, []domain.CardShell{
		shell("10:00 AM", "10:30 AM", "Coding", "coding"),
		shell("10:30 AM", "11:00 AM", "Idle", "idle"),
	}, 1); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if err := seedSystemCard(t, store, 1, "10:00 AM", "11:00 AM"); err != nil {
		t.Fatalf("seedSystemCard: %v", err)
	}

	rangeFrom := time.Date(2026, 9, 12, 4, 0, 0, 0, loc)
	rangeTo := time.Date(2026, 9, 13, 4, 0, 0, 0, loc)
	spans, err := store.Cards().CardSpansInRange(ctx, rangeFrom, rangeTo)
	if err != nil {
		t.Fatalf("CardSpansInRange: %v", err)
	}
	if len(spans) != 3 {
		t.Fatalf("spans = %+v, want 3 (Coding, Idle, System)", spans)
	}
	for _, span := range spans {
		if span.Day != "2026-09-12" {
			t.Fatalf("span day = %q, want the 4am logical day", span.Day)
		}
	}
	byCategory := map[string]CardSpan{}
	for _, span := range spans {
		byCategory[span.Category] = span
	}
	if c := byCategory["Coding"]; c.IsIdle || c.ColorHex != "#000000" {
		t.Fatalf("coding span = %+v, want isIdle=false and seeded color", c)
	}
	if i := byCategory["Idle"]; !i.IsIdle || i.ColorHex == "" {
		t.Fatalf("idle span = %+v, want isIdle=true and built-in color", i)
	}
	if s := byCategory["System"]; s.ColorHex != "#8E8E93" {
		t.Fatalf("system span = %+v, want built-in color", s)
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
	seedBatchWithStatus(t, store, 2, "skipped_short", dayEnd-hour, dayEnd)            // normal outcome, not a failure
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
	for _, id := range []int64{1, 4, 5} {
		if !gotIDs[id] {
			t.Fatalf("batch %d missing from %+v (overlap rule: touches window)", id, gotIDs)
		}
	}
	if gotIDs[3] {
		t.Fatal("succeeded batch reported as failed")
	}
	if gotIDs[2] {
		t.Fatal("skipped_short batch reported as a failure (it is a normal outcome)")
	}
	if gotIDs[6] {
		t.Fatal("batch starting at window end reported as overlapping (left-closed, right-open)")
	}
}
