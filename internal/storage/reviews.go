package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

/*
 * Review verdicts: the user's focus judgment per card from the review flow.
 * Verdicts are statistics only — they never rewrite the card's category.
 * One row per card (upsert), keyed by card; the day and minutes are snapshotted
 * at judgment time from the card itself, so day totals survive card edits and
 * soft deletes drop out of totals through the join.
 */

const (
	VerdictDistraction = "distraction"
	VerdictNeutral     = "neutral"
	VerdictFocus       = "focus"
)

// ValidVerdict reports whether the string is one of the three stored verdicts.
func ValidVerdict(verdict string) bool {
	switch verdict {
	case VerdictDistraction, VerdictNeutral, VerdictFocus:
		return true
	}
	return false
}

type ReviewRepo struct{ store *Store }

func (s *Store) Reviews() *ReviewRepo { return &ReviewRepo{store: s} }

// ReviewDayTotals sums judged minutes per verdict for one logical day.
type ReviewDayTotals struct {
	DistractionMinutes int
	NeutralMinutes     int
	FocusMinutes       int
}

// SetVerdict upserts the verdict for one card. The day and minutes come from
// the card row itself, so a caller cannot record totals detached from data.
func (r *ReviewRepo) SetVerdict(ctx context.Context, cardID int64, verdict string, now time.Time) error {
	if r == nil || r.store == nil {
		return fmt.Errorf("reviews: store unavailable")
	}
	if !ValidVerdict(verdict) {
		return fmt.Errorf("reviews: unknown verdict %q", verdict)
	}
	return r.store.Write(ctx, "review set verdict", func(ctx context.Context, tx *sql.Tx) error {
		var day string
		var minutes int
		err := tx.QueryRowContext(ctx,
			// minutes mirror the binding's cardDurationMinutes: the span in
			// whole minutes (truncated), matching how the UI displays it.
			`SELECT day, CAST((end_ts - start_ts) / 60 AS INTEGER) FROM timeline_cards
			 WHERE id = ? AND is_deleted = 0`, cardID,
		).Scan(&day, &minutes)
		if err != nil {
			return fmt.Errorf("reviews: card %d unavailable: %w", cardID, err)
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO card_reviews (card_id, day, verdict, minutes, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?)
			 ON CONFLICT(card_id) DO UPDATE SET
			   verdict = excluded.verdict,
			   minutes = excluded.minutes,
			   day = excluded.day,
			   updated_at = excluded.updated_at`,
			cardID, day, verdict, minutes, now.Unix(), now.Unix())
		return err
	})
}

// ClearVerdict removes the judgment for one card (撤销).
func (r *ReviewRepo) ClearVerdict(ctx context.Context, cardID int64) error {
	if r == nil || r.store == nil {
		return fmt.Errorf("reviews: store unavailable")
	}
	return r.store.Write(ctx, "review clear verdict", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM card_reviews WHERE card_id = ?`, cardID)
		return err
	})
}

// ReviewedCardIDs lists the cards with a stored verdict for one logical
// day, so the review queue can exclude already-judged cards after a restart.
func (r *ReviewRepo) ReviewedCardIDs(ctx context.Context, day string) ([]int64, error) {
	if r == nil || r.store == nil {
		return nil, fmt.Errorf("reviews: store unavailable")
	}
	var ids []int64
	err := r.store.Read(ctx, "review reviewed ids", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			`SELECT cr.card_id FROM card_reviews cr
			 JOIN timeline_cards c ON c.id = cr.card_id AND c.is_deleted = 0
			 WHERE cr.day = ?`, day)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	return ids, err
}

// Verdict returns the stored verdict for one card, or "" when the card has no
// judgment (or was soft-deleted after judging). Statistics only — the card's
// category is untouched.
func (r *ReviewRepo) Verdict(ctx context.Context, cardID int64) (string, error) {
	if r == nil || r.store == nil {
		return "", fmt.Errorf("reviews: store unavailable")
	}
	var verdict string
	err := r.store.Read(ctx, "review card verdict", func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx,
			`SELECT cr.verdict FROM card_reviews cr
			 JOIN timeline_cards c ON c.id = cr.card_id AND c.is_deleted = 0
			 WHERE cr.card_id = ?`, cardID)
		switch err := row.Scan(&verdict); {
		case errors.Is(err, sql.ErrNoRows):
			verdict = ""
			return nil
		case err != nil:
			return err
		}
		return nil
	})
	return verdict, err
}

// TotalsByDay sums judged minutes per verdict for one logical day. Cards that
// were soft-deleted after judging drop out of the totals.
func (r *ReviewRepo) TotalsByDay(ctx context.Context, day string) (ReviewDayTotals, error) {
	if r == nil || r.store == nil {
		return ReviewDayTotals{}, fmt.Errorf("reviews: store unavailable")
	}
	var totals ReviewDayTotals
	err := r.store.Read(ctx, "review day totals", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			`SELECT cr.verdict, COALESCE(SUM(cr.minutes), 0)
			 FROM card_reviews cr
			 JOIN timeline_cards c ON c.id = cr.card_id AND c.is_deleted = 0
			 WHERE cr.day = ?
			 GROUP BY cr.verdict`, day)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var verdict string
			var minutes int
			if err := rows.Scan(&verdict, &minutes); err != nil {
				return err
			}
			switch verdict {
			case VerdictDistraction:
				totals.DistractionMinutes = minutes
			case VerdictNeutral:
				totals.NeutralMinutes = minutes
			case VerdictFocus:
				totals.FocusMinutes = minutes
			}
		}
		return rows.Err()
	})
	return totals, err
}
