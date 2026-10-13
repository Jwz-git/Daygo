package app

import (
	"context"
	"errors"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

type MergeCardsInputDTO struct {
	CardIDs         []int64 `json:"cardIds"`
	Title           string  `json:"title"`
	Summary         string  `json:"summary"`
	DetailedSummary string  `json:"detailedSummary"`
	Category        string  `json:"category"`
}

// The result exposes identifiers and time only, never paths or raw metadata.
type MergeCardsResultDTO struct {
	CardID        int64   `json:"cardId"`
	MergedCardIDs []int64 `json:"mergedCardIds"`
	Day           string  `json:"day"`
	StartTs       int64   `json:"startTs"`
	EndTs         int64   `json:"endTs"`
}

func (b *Backend) MergeCards(input MergeCardsInputDTO) (MergeCardsResultDTO, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	return b.mergeCards(ctx, input)
}

// All entry points share the capture-owner guard and the atomic repository
// path. Chat/socket additionally enforce their own edit gate before dispatch.
func (b *Backend) mergeCards(ctx context.Context, input MergeCardsInputDTO) (MergeCardsResultDTO, error) {
	if err := b.requireTimelineWrite(); err != nil {
		return MergeCardsResultDTO{}, err
	}
	store := b.store()
	if store == nil {
		return MergeCardsResultDTO{}, apperr.E(apperr.DatabaseError, "merge cards requires a database", nil)
	}
	var card domain.TimelineCard
	mutate := func() error {
		var err error
		card, err = store.Cards().MergeCards(ctx, input.CardIDs, domain.CardShell{
			Title: input.Title, Summary: input.Summary, DetailedSummary: input.DetailedSummary, Category: input.Category,
		})
		return err
	}
	var err error
	if pipeline := b.analysisService(); pipeline != nil {
		err = pipeline.WithCardMutation(ctx, mutate)
	} else {
		err = mutate()
	}
	if errors.Is(err, storage.ErrInvalidCardMerge) {
		return MergeCardsResultDTO{}, apperr.E(apperr.InvalidArgument, err.Error(), err)
	}
	if err != nil {
		return MergeCardsResultDTO{}, mapStorageError("merge cards", err)
	}
	b.emitTimelineInvalidation(card.Day)
	lastDay := timeutil.LogicalDay(time.Unix(card.EndTs-1, 0), store.Location())
	if lastDay != card.Day {
		b.emitTimelineInvalidation(lastDay)
	}
	return MergeCardsResultDTO{
		CardID: card.ID, MergedCardIDs: append([]int64(nil), input.CardIDs...), Day: card.Day, StartTs: card.StartTs, EndTs: card.EndTs,
	}, nil
}
