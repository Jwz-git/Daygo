package app

import (
	"context"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/storage"
)

/*
 * Review bindings. Verdicts are the user's focus statistics from the review
 * flow: they never rewrite the card's category. A judgment snapshots the card's
 * day and minutes at save time; 撤销 deletes the row. Totals are per logical day
 * and read back through GetReviewTotals. Summary ratings are the separate thumbs
 * up/down on a card's summary text.
 */
func (b *Backend) SaveCardReview(cardID int64, verdict string) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	if cardID <= 0 {
		return apperr.E(apperr.InvalidArgument, "invalid card id", nil)
	}
	if !storage.ValidVerdict(verdict) {
		return apperr.E(apperr.InvalidArgument, "unknown review verdict", nil)
	}
	store := b.store()
	if store == nil {
		return apperr.E(apperr.DatabaseError, "review requires a database", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	if err := store.Reviews().SetVerdict(ctx, cardID, verdict, time.Now()); err != nil {
		return mapStorageError("save card review", err)
	}
	return nil
}

// GetCardVerdict returns the stored focus verdict for one card, or "" when it
// has not been judged, so the inspector can show and change the current choice
// outside the sequential review flow. Read-only: no write lock required.
func (b *Backend) GetCardVerdict(cardID int64) (string, error) {
	if cardID <= 0 {
		return "", apperr.E(apperr.InvalidArgument, "invalid card id", nil)
	}
	store := b.store()
	if store == nil {
		return "", apperr.E(apperr.DatabaseError, "review requires a database", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	verdict, err := store.Reviews().Verdict(ctx, cardID)
	if err != nil {
		return "", mapStorageError("card verdict", err)
	}
	return verdict, nil
}

func (b *Backend) ClearCardReview(cardID int64) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	if cardID <= 0 {
		return apperr.E(apperr.InvalidArgument, "invalid card id", nil)
	}
	store := b.store()
	if store == nil {
		return apperr.E(apperr.DatabaseError, "review requires a database", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	if err := store.Reviews().ClearVerdict(ctx, cardID); err != nil {
		return mapStorageError("clear card review", err)
	}
	return nil
}

/*
 * Summary rating bindings. The thumbs answer "was this card's summary any
 * good?" — feedback on the AI-written summary text. A rating never rewrites the
 * summary or the card's category, and 撤销 (tapping the active thumb again) is
 * ClearCardRating. Reading needs no write lock, so the inspector can show the
 * current choice on a read-only instance.
 */
func (b *Backend) SaveCardRating(cardID int64, rating string) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	if cardID <= 0 {
		return apperr.E(apperr.InvalidArgument, "invalid card id", nil)
	}
	if !storage.ValidRating(rating) {
		return apperr.E(apperr.InvalidArgument, "unknown summary rating", nil)
	}
	store := b.store()
	if store == nil {
		return apperr.E(apperr.DatabaseError, "review requires a database", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	if err := store.Reviews().SetRating(ctx, cardID, rating, time.Now()); err != nil {
		return mapStorageError("save card rating", err)
	}
	return nil
}

// GetCardRating returns the stored summary rating for one card, or "" when it
// has not been rated, so the inspector can show the active thumb.
func (b *Backend) GetCardRating(cardID int64) (string, error) {
	if cardID <= 0 {
		return "", apperr.E(apperr.InvalidArgument, "invalid card id", nil)
	}
	store := b.store()
	if store == nil {
		return "", apperr.E(apperr.DatabaseError, "review requires a database", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	rating, err := store.Reviews().Rating(ctx, cardID)
	if err != nil {
		return "", mapStorageError("card rating", err)
	}
	return rating, nil
}

func (b *Backend) ClearCardRating(cardID int64) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	if cardID <= 0 {
		return apperr.E(apperr.InvalidArgument, "invalid card id", nil)
	}
	store := b.store()
	if store == nil {
		return apperr.E(apperr.DatabaseError, "review requires a database", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	if err := store.Reviews().ClearRating(ctx, cardID); err != nil {
		return mapStorageError("clear card rating", err)
	}
	return nil
}

// ReviewTotalsDTO carries one day's judged minutes per verdict plus the
// card ids already judged, so the review queue excludes them after a restart.
type ReviewTotalsDTO struct {
	DistractionMinutes int     `json:"distractionMinutes"`
	NeutralMinutes     int     `json:"neutralMinutes"`
	FocusMinutes       int     `json:"focusMinutes"`
	ReviewedCardIDs    []int64 `json:"reviewedCardIds"`
}

func (b *Backend) GetReviewTotals(day string) (ReviewTotalsDTO, error) {
	if len(day) != 10 {
		return ReviewTotalsDTO{}, apperr.E(apperr.InvalidArgument, "invalid day", nil)
	}
	store := b.store()
	if store == nil {
		return ReviewTotalsDTO{}, apperr.E(apperr.DatabaseError, "review requires a database", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	totals, err := store.Reviews().TotalsByDay(ctx, day)
	if err != nil {
		return ReviewTotalsDTO{}, mapStorageError("review totals", err)
	}
	reviewed, err := store.Reviews().ReviewedCardIDs(ctx, day)
	if err != nil {
		return ReviewTotalsDTO{}, mapStorageError("review totals", err)
	}
	if reviewed == nil {
		reviewed = []int64{}
	}
	return ReviewTotalsDTO{
		DistractionMinutes: totals.DistractionMinutes,
		NeutralMinutes:     totals.NeutralMinutes,
		FocusMinutes:       totals.FocusMinutes,
		ReviewedCardIDs:    reviewed,
	}, nil
}
