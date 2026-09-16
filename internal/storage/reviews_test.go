package storage

import (
	"context"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/domain"
)

// Review verdict tests use the card-test helpers: verdicts attach to real
// committed cards, whose day and span drive the recorded minutes.

var reviewBatchCounter int64 = 100

func seedReviewCard(t *testing.T, store *Store, title string, hour int, minutes int) int64 {
	t.Helper()
	reviewBatchCounter++
	loc := store.location()
	start := time.Date(2026, 9, 16, hour, 0, 0, 0, loc)
	end := start.Add(time.Duration(minutes) * time.Minute)
	seedBatch(t, store, reviewBatchCounter)
	res, err := store.Cards().ReplaceCardsInRange(
		context.Background(), start, end,
		[]domain.CardShell{domainCardShell(title, start, end)},
		reviewBatchCounter,
	)
	if err != nil {
		t.Fatalf("seed card: %v", err)
	}
	if len(res.InsertedIDs) != 1 {
		t.Fatalf("seeded %d cards, want 1", len(res.InsertedIDs))
	}
	return res.InsertedIDs[0]
}

func domainCardShell(title string, start, end time.Time) domain.CardShell {
	return domain.CardShell{
		Start:    start.Format("3:04 PM"),
		End:      end.Format("3:04 PM"),
		Category: "Focus Work",
		Title:    title,
		Summary:  "s:" + title,
	}
}

func TestReviewVerdictUpsertAndTotals(t *testing.T) {
	store := openWriter(t, newDir(t))
	reviews := store.Reviews()
	ctx := context.Background()
	now := time.Unix(1789600000, 0)

	first := seedReviewCard(t, store, "alpha", 10, 30)
	second := seedReviewCard(t, store, "beta", 14, 20)

	if err := reviews.SetVerdict(ctx, first, VerdictFocus, now); err != nil {
		t.Fatal(err)
	}
	if err := reviews.SetVerdict(ctx, second, VerdictNeutral, now); err != nil {
		t.Fatal(err)
	}
	// Re-judging the same card overwrites instead of stacking.
	if err := reviews.SetVerdict(ctx, first, VerdictDistraction, now); err != nil {
		t.Fatal(err)
	}

	totals, err := reviews.TotalsByDay(ctx, "2026-09-16")
	if err != nil {
		t.Fatal(err)
	}
	if totals.DistractionMinutes != 30 || totals.NeutralMinutes != 20 || totals.FocusMinutes != 0 {
		t.Fatalf("totals=%+v", totals)
	}

	empty, err := reviews.TotalsByDay(ctx, "2026-09-17")
	if err != nil {
		t.Fatal(err)
	}
	if empty != (ReviewDayTotals{}) {
		t.Fatalf("other day polluted: %+v", empty)
	}
}

func TestReviewClearAndSoftDeletedExcluded(t *testing.T) {
	store := openWriter(t, newDir(t))
	reviews := store.Reviews()
	ctx := context.Background()
	now := time.Unix(1789600000, 0)

	kept := seedReviewCard(t, store, "kept", 10, 25)
	removed := seedReviewCard(t, store, "removed", 15, 35)

	if err := reviews.SetVerdict(ctx, kept, VerdictFocus, now); err != nil {
		t.Fatal(err)
	}
	if err := reviews.SetVerdict(ctx, removed, VerdictFocus, now); err != nil {
		t.Fatal(err)
	}

	if _, err := store.Cards().SoftDeleteCard(ctx, removed); err != nil {
		t.Fatal(err)
	}

	if err := reviews.ClearVerdict(ctx, kept); err != nil {
		t.Fatal(err)
	}

	totals, err := reviews.TotalsByDay(ctx, "2026-09-16")
	if err != nil {
		t.Fatal(err)
	}
	// The cleared card contributes nothing; the soft-deleted one drops out.
	if totals.FocusMinutes != 0 {
		t.Fatalf("focus minutes = %d, want 0", totals.FocusMinutes)
	}
}

func TestReviewRejectsUnknownVerdict(t *testing.T) {
	store := openWriter(t, newDir(t))
	id := seedReviewCard(t, store, "gamma", 10, 10)
	if err := store.Reviews().SetVerdict(context.Background(), id, "excellent", time.Unix(1789600000, 0)); err == nil {
		t.Fatal("unknown verdict accepted")
	}
}
