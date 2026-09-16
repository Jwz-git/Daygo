package app

import (
	"context"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/storage"
)

/*
 * Review verdict bindings. Verdicts are the user's focus statistics from the
 * review flow: they never rewrite the card's category. A judgment snapshots
 * the card's day and minutes at save time; 撤销 deletes the row. Totals are
 * per logical day and read back through GetReviewTotals.
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

// ReviewTotalsDTO carries one day's judged minutes per verdict.
type ReviewTotalsDTO struct {
	DistractionMinutes int `json:"distractionMinutes"`
	NeutralMinutes     int `json:"neutralMinutes"`
	FocusMinutes       int `json:"focusMinutes"`
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
	return ReviewTotalsDTO{
		DistractionMinutes: totals.DistractionMinutes,
		NeutralMinutes:     totals.NeutralMinutes,
		FocusMinutes:       totals.FocusMinutes,
	}, nil
}
