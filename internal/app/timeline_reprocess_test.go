package app

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// regenerateBackend returns a write-locked backend over a real database holding
// one card in a seeded batch — the state the timeline's per-card regenerate
// binding starts from. No pipeline is attached: the binding tests below cover
// the paths that must be refused before any model call.
func regenerateBackend(t *testing.T) (*Backend, *storage.Store, domain.TimelineCard) {
	t.Helper()
	backend, _ := writerBackendWithStore(t, t.TempDir())
	store := backend.store()

	if err := store.Write(context.Background(), "seed batch", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO analysis_batches (id, start_ts, end_ts, status, created_at, updated_at)
			 VALUES (1, 0, 0, 'succeeded', 0, 0)`)
		return err
	}); err != nil {
		t.Fatalf("seed batch: %v", err)
	}

	start := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	end := time.Date(2026, 9, 12, 10, 15, 0, 0, time.Local)
	loc := store.Location()
	if _, err := store.Cards().ReplaceCardsInRange(context.Background(), start, end,
		[]domain.CardShell{{
			Start:    timeutil.FormatClock(start, loc),
			End:      timeutil.FormatClock(end, loc),
			Category: "Coding",
			Title:    "Editing code",
			Summary:  "S",
		}}, 1); err != nil {
		t.Fatalf("seed card: %v", err)
	}
	cards, err := store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if err != nil || len(cards) != 1 {
		t.Fatalf("cards = %+v, err = %v; want one", cards, err)
	}
	return backend, store, cards[0]
}

// A card with no batch provenance (a System fallback) has nothing a rewrite
// could attribute its rows to: refused as an invalid argument, before the
// pipeline is even consulted.
func TestReprocessCardWithoutBatchIsInvalidArgument(t *testing.T) {
	backend, store, card := regenerateBackend(t)
	if err := store.Write(context.Background(), "clear batch id", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE timeline_cards SET batch_id = NULL WHERE id = ?`, card.ID)
		return err
	}); err != nil {
		t.Fatalf("clear batch id: %v", err)
	}

	assertAppCode(t, backend.ReprocessCard(card.ID), apperr.InvalidArgument)
}

// Without a running pipeline there is no path that could rewrite the card — a
// read-only instance, or a startup where analysis failed. The card must be left
// exactly as it was.
func TestReprocessCardWithoutPipelineFails(t *testing.T) {
	backend, store, card := regenerateBackend(t)

	assertAppCode(t, backend.ReprocessCard(card.ID), apperr.Internal)

	cards, err := store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if err != nil || len(cards) != 1 || cards[0].ID != card.ID || cards[0].Title != "Editing code" {
		t.Fatalf("cards = %+v, err = %v; want the card untouched", cards, err)
	}
}

// A card id that names nothing is a storage not-found, not a silent success.
func TestReprocessCardUnknownIDFails(t *testing.T) {
	backend, _, _ := regenerateBackend(t)
	assertAppCode(t, backend.ReprocessCard(4242), apperr.NotFound)
	assertAppCode(t, backend.ReprocessCard(0), apperr.InvalidArgument)
}
