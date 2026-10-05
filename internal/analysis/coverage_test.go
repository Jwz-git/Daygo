package analysis

import (
	"context"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
)

func TestFreshCardRequiresWindowCoverage(t *testing.T) {
	at := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	spans := []cardSpan{{Start: at.Add(7 * time.Minute), End: at.Add(8 * time.Minute)}}
	if issues := validateCards(spans, at, at.Add(15*time.Minute), true); len(issues) != 2 {
		t.Fatalf("issues = %v, want both uncovered boundaries", issues)
	}
	h := newHarness(t, nil)
	floor := h.service.mergeOwnershipStart([]domain.CardShell{{Start: "10:07 AM", End: "10:08 AM"}},
		storage.Batch{Start: at, End: at.Add(15 * time.Minute)}, nil)
	if !floor.Equal(at) {
		t.Fatalf("ownership floor = %v, want batch start %v", floor, at)
	}
}

func TestFreshBatchRejectsIncompleteCoverage(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":14,"observation":"Anonymous work","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:07 AM","end":"10:08 AM","category":"Coding","subcategory":"","title":"Anonymous work","summary":"Anonymous work.","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
	})
	at := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, at, 31, 30*time.Second, func(int) *int { return nil })
	h.service.tick(context.Background())
	batches := mustBatches(t, h.store)
	cards, err := h.store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || batches[0].Status != storage.BatchFailed || len(cards) != 0 {
		t.Fatalf("batches=%+v cards=%+v, want failed batch with no partial cards", batches, cards)
	}
	if calls := h.provider.callCount(string(ai.PurposeCards)); calls != 3 {
		t.Fatalf("card calls = %d, want generation and two corrections", calls)
	}
}
