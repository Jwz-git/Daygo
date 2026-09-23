package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// Summary rating tests reuse the review-card helpers: a rating attaches to a
// real committed card, the same way a verdict does.

func TestReviewRatingUpsertAndReadBack(t *testing.T) {
	store := openWriter(t, newDir(t))
	reviews := store.Reviews()
	ctx := context.Background()
	now := time.Unix(1789600000, 0)

	card := seedReviewCard(t, store, "alpha", 10, 30)

	unrated, err := reviews.Rating(ctx, card)
	if err != nil {
		t.Fatal(err)
	}
	if unrated != "" {
		t.Fatalf("rating = %q before any vote, want empty", unrated)
	}

	if err := reviews.SetRating(ctx, card, RatingUp, now); err != nil {
		t.Fatalf("set rating: %v", err)
	}
	got, err := reviews.Rating(ctx, card)
	if err != nil {
		t.Fatal(err)
	}
	if got != RatingUp {
		t.Fatalf("rating = %q, want %q", got, RatingUp)
	}

	// Re-voting overwrites instead of stacking: flipping to down must not
	// leave both rows behind.
	if err := reviews.SetRating(ctx, card, RatingDown, now); err != nil {
		t.Fatalf("re-rate: %v", err)
	}
	got, err = reviews.Rating(ctx, card)
	if err != nil {
		t.Fatal(err)
	}
	if got != RatingDown {
		t.Fatalf("rating after re-vote = %q, want %q", got, RatingDown)
	}
	var rows int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM card_ratings WHERE card_id = ?`, card).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("card_ratings rows = %d, want 1", rows)
	}

	// Tapping the active thumb again clears the vote.
	if err := reviews.ClearRating(ctx, card); err != nil {
		t.Fatalf("clear rating: %v", err)
	}
	got, err = reviews.Rating(ctx, card)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("rating = %q after clear, want empty", got)
	}

	// Clearing again is a no-op rather than an error: the frontend may race
	// two taps, and the second one has nothing to delete.
	if err := reviews.ClearRating(ctx, card); err != nil {
		t.Fatalf("second clear: %v", err)
	}
}

func TestReviewRatingRejectsUnknownValue(t *testing.T) {
	store := openWriter(t, newDir(t))
	ctx := context.Background()
	card := seedReviewCard(t, store, "alpha", 10, 30)

	if err := store.Reviews().SetRating(ctx, card, "maybe", time.Unix(1789600000, 0)); err == nil {
		t.Fatal("unknown rating accepted; the stored set is closed")
	}
	var rows int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM card_ratings`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("card_ratings rows = %d after a rejected vote, want 0", rows)
	}
}

// The CHECK constraint is the second line of defence: a value that never went
// through SetRating must still not reach the table, so a future writer that
// forgets ValidRating cannot widen the stored set.
func TestReviewRatingTableRejectsUnknownValue(t *testing.T) {
	store := openWriter(t, newDir(t))
	card := seedReviewCard(t, store, "alpha", 10, 30)

	err := store.Write(context.Background(), "insert bogus rating", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO card_ratings (card_id, rating, created_at, updated_at) VALUES (?, 'sideways', 0, 0)`,
			card)
		return err
	})
	if err == nil {
		t.Fatal("CHECK constraint did not reject an unknown rating")
	}
}

func TestReviewRatingRequiresLiveCard(t *testing.T) {
	store := openWriter(t, newDir(t))
	reviews := store.Reviews()
	ctx := context.Background()
	now := time.Unix(1789600000, 0)

	if err := reviews.SetRating(ctx, 999999, RatingUp, now); err == nil {
		t.Fatal("rating accepted for a card that does not exist")
	}

	card := seedReviewCard(t, store, "alpha", 10, 30)
	if err := reviews.SetRating(ctx, card, RatingUp, now); err != nil {
		t.Fatalf("set rating: %v", err)
	}
	if _, err := store.Cards().SoftDeleteCard(ctx, card); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	// A deleted card is not rateable and its stale row must not resurface.
	if err := reviews.SetRating(ctx, card, RatingDown, now); err == nil {
		t.Fatal("rating accepted for a soft-deleted card")
	}
	got, err := reviews.Rating(ctx, card)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("rating = %q for a soft-deleted card, want empty", got)
	}
}
