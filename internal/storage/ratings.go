package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

/*
 * Summary ratings: the user's thumbs up/down on one card's AI summary.
 * Ratings are feedback on the summary text only — they never rewrite the
 * summary or the card's category. One row per card (upsert), keyed by card;
 * clearing deletes the row, mirroring 撤销 in the review flow.
 */

const (
	RatingUp   = "up"
	RatingDown = "down"
)

// ValidRating reports whether the string is one of the two stored ratings.
func ValidRating(rating string) bool {
	switch rating {
	case RatingUp, RatingDown:
		return true
	}
	return false
}

// SetRating upserts the summary rating for one card. The card must exist and
// not be soft-deleted, so a rating cannot outlive the card it describes.
func (r *ReviewRepo) SetRating(ctx context.Context, cardID int64, rating string, now time.Time) error {
	if r == nil || r.store == nil {
		return fmt.Errorf("reviews: store unavailable")
	}
	if !ValidRating(rating) {
		return fmt.Errorf("reviews: unknown rating %q", rating)
	}
	return r.store.Write(ctx, "review set rating", func(ctx context.Context, tx *sql.Tx) error {
		var present int
		err := tx.QueryRowContext(ctx,
			`SELECT 1 FROM timeline_cards WHERE id = ? AND is_deleted = 0`, cardID).Scan(&present)
		if err != nil {
			return fmt.Errorf("reviews: card %d unavailable: %w", cardID, err)
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO card_ratings (card_id, rating, created_at, updated_at)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT(card_id) DO UPDATE SET
			   rating = excluded.rating,
			   updated_at = excluded.updated_at`,
			cardID, rating, now.Unix(), now.Unix())
		return err
	})
}

// ClearRating removes the summary rating for one card. Clearing an unrated
// card is a no-op, so the caller does not need to read first.
func (r *ReviewRepo) ClearRating(ctx context.Context, cardID int64) error {
	if r == nil || r.store == nil {
		return fmt.Errorf("reviews: store unavailable")
	}
	return r.store.Write(ctx, "review clear rating", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM card_ratings WHERE card_id = ?`, cardID)
		return err
	})
}

// Rating returns the stored summary rating for one card, or "" when the card
// is unrated (or was soft-deleted after being rated).
func (r *ReviewRepo) Rating(ctx context.Context, cardID int64) (string, error) {
	if r == nil || r.store == nil {
		return "", fmt.Errorf("reviews: store unavailable")
	}
	var rating string
	err := r.store.Read(ctx, "review card rating", func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx,
			`SELECT cr.rating FROM card_ratings cr
			 JOIN timeline_cards c ON c.id = cr.card_id AND c.is_deleted = 0
			 WHERE cr.card_id = ?`, cardID)
		switch err := row.Scan(&rating); {
		case errors.Is(err, sql.ErrNoRows):
			rating = ""
			return nil
		case err != nil:
			return err
		}
		return nil
	})
	return rating, err
}
