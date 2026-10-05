package analysis

import (
	"context"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/storage"
)

func TestNextAutomaticBatchPreservesHumanCard(t *testing.T) {
	for _, kind := range []string{"title and verdict", "category", "summary", "detailed summary", "verdict only", "rating only", "deleted"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			h := newHarness(t, map[string]string{
				string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":14,"observation":"Anonymous work","apps":[]}]}`,
				string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"","title":"Generated title","summary":"Anonymous work.","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
			})
			at := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
			h.commitFrames(t, at, 31, 30*time.Second, func(int) *int { return nil })
			h.service.tick(ctx)
			cards, err := h.store.Cards().CardsForDay(ctx, "2026-09-12")
			if err != nil || len(cards) != 1 {
				t.Fatalf("setup cards=%+v err=%v", cards, err)
			}
			original := cards[0]
			switch kind {
			case "title and verdict":
				err = h.store.Cards().UpdateCardTitle(ctx, original.ID, "Human corrected title")
				if err == nil {
					err = h.store.Reviews().SetVerdict(ctx, original.ID, storage.VerdictFocus, at)
				}
			case "category":
				err = h.store.Cards().UpdateCardCategory(ctx, original.ID, "Idle")
			case "summary":
				err = h.store.Cards().UpdateCardSummary(ctx, original.ID, "Human summary")
			case "detailed summary":
				err = h.store.Cards().UpdateCardDetailedSummary(ctx, original.ID, "Human details")
			case "verdict only":
				err = h.store.Reviews().SetVerdict(ctx, original.ID, storage.VerdictFocus, at)
			case "rating only":
				err = h.store.Reviews().SetRating(ctx, original.ID, storage.RatingUp, at)
			case "deleted":
				_, err = h.store.Cards().SoftDeleteCard(ctx, original.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			if kind != "deleted" {
				original, err = h.store.Cards().CardByID(ctx, original.ID)
				if err != nil {
					t.Fatal(err)
				}
			}
			h.provider.responses[string(ai.PurposeCards)] = `{"cards":[{"start":"10:00 AM","end":"10:30 AM","category":"Coding","subcategory":"","title":"Generated title","summary":"Anonymous work.","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`
			h.commitFrames(t, at.Add(15*time.Minute+30*time.Second), 31, 30*time.Second, func(int) *int { return nil })
			h.service.tick(ctx)
			if batches := mustBatches(t, h.store); len(batches) != 2 || batches[1].Status != storage.BatchSucceeded {
				t.Fatalf("batches=%+v", batches)
			}
			cards, err = h.store.Cards().CardsForDay(ctx, "2026-09-12")
			if err != nil {
				t.Fatal(err)
			}
			if kind == "deleted" {
				if len(cards) != 1 || cards[0].StartTs < original.EndTs {
					t.Fatalf("deleted interval resurrected: %+v", cards)
				}
			} else {
				if len(cards) != 2 || cards[0].ID != original.ID || cards[0].Title != original.Title ||
					cards[0].Category != original.Category || cards[0].Summary != original.Summary || cards[0].DetailedSummary != original.DetailedSummary ||
					cards[0].StartTs != original.StartTs || cards[0].EndTs != original.EndTs || cards[1].StartTs < original.EndTs {
					t.Fatalf("human card changed or overlapped: original=%+v cards=%+v", original, cards)
				}
			}
			if kind == "title and verdict" || kind == "verdict only" {
				totals, err := h.store.Reviews().TotalsByDay(ctx, "2026-09-12")
				if err != nil || totals.FocusMinutes != 15 {
					t.Fatalf("review totals=%+v err=%v", totals, err)
				}
			}
			if kind == "rating only" {
				rating, err := h.store.Reviews().Rating(ctx, original.ID)
				if err != nil || rating != storage.RatingUp {
					t.Fatalf("rating=%q err=%v", rating, err)
				}
			}
		})
	}
}

func TestHumanEditWhileModelGeneratesIsPreserved(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":14,"observation":"Anonymous work","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"","title":"Generated title","summary":"Anonymous work.","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
	})
	at := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, at, 31, 30*time.Second, func(int) *int { return nil })
	h.service.tick(ctx)
	cards, err := h.store.Cards().CardsForDay(ctx, "2026-09-12")
	if err != nil || len(cards) != 1 {
		t.Fatalf("setup=%+v err=%v", cards, err)
	}
	id := cards[0].ID
	h.provider.responses[string(ai.PurposeCards)] = `{"cards":[{"start":"10:00 AM","end":"10:30 AM","category":"Coding","subcategory":"","title":"Generated title","summary":"Anonymous work.","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`
	h.service.cfg.Providers = schedulerChainSource{provider: schedulerProvider{generate: func(callCtx context.Context, req ai.Request) (ai.Result, error) {
		if req.Purpose == ai.PurposeCards {
			// History has already been read; commit a human write while the model
			// request is running, before generated output reaches the transaction.
			if err := h.store.Cards().UpdateCardTitle(callCtx, id, "In-flight human title"); err != nil {
				return ai.Result{}, err
			}
		}
		return h.provider.Generate(callCtx, req)
	}}}
	h.commitFrames(t, at.Add(15*time.Minute+30*time.Second), 31, 30*time.Second, func(int) *int { return nil })
	h.service.tick(ctx)
	kept, err := h.store.Cards().CardByID(ctx, id)
	if err != nil || kept.Title != "In-flight human title" {
		t.Fatalf("kept=%+v err=%v", kept, err)
	}
}

func TestIdleBatchKeepsReviewedCard(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	at := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, at, 31, 30*time.Second, func(int) *int { return intPtr(3600) })
	h.service.tick(ctx)
	cards, err := h.store.Cards().CardsForDay(ctx, "2026-09-12")
	if err != nil || len(cards) != 1 {
		t.Fatalf("setup=%+v err=%v", cards, err)
	}
	id := cards[0].ID
	if err := h.store.Reviews().SetVerdict(ctx, id, storage.VerdictNeutral, at); err != nil {
		t.Fatal(err)
	}
	h.commitFrames(t, at.Add(15*time.Minute+30*time.Second), 31, 30*time.Second, func(int) *int { return intPtr(3600) })
	h.service.tick(ctx)
	kept, err := h.store.Cards().CardByID(ctx, id)
	if err != nil || kept.EndTs != at.Add(15*time.Minute).Unix() {
		t.Fatalf("kept=%+v err=%v", kept, err)
	}
	totals, err := h.store.Reviews().TotalsByDay(ctx, "2026-09-12")
	if err != nil || totals.NeutralMinutes != 15 {
		t.Fatalf("totals=%+v err=%v", totals, err)
	}
}
