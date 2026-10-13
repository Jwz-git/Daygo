package analysis

import (
	"context"
	"errors"
	"testing"

	"github.com/Jwz-git/Daygo/internal/domain"
)

func TestCardMutationSharesSerializationAndHonorsCancellation(t *testing.T) {
	s := &Service{}
	s.cardsMu.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	called := false
	go func() { done <- s.WithCardMutation(ctx, func() error { called = true; return nil }) }()
	cancel()
	s.cardsMu.Unlock()
	if err := <-done; !errors.Is(err, context.Canceled) || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
	expected := errors.New("fixture")
	if err := s.WithCardMutation(context.Background(), func() error { return expected }); !errors.Is(err, expected) {
		t.Fatal(err)
	}
}

func TestRegenerateCardRejectsSourceMergedWhileWaiting(t *testing.T) {
	h, _, target, sibling := regenerateHarness(t)
	ctx := context.Background()
	merged, err := h.store.Cards().MergeCards(ctx, []int64{target.ID, sibling.ID}, domain.CardShell{Title: "Merged", Summary: "s", Category: "Coding"})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.service.RegenerateCard(ctx, target); !errors.Is(err, ErrCardChanged) {
		t.Fatalf("stale regenerate=%v", err)
	}
	card, err := h.store.Cards().CardByID(ctx, merged.ID)
	if err != nil || card.Title != "Merged" {
		t.Fatalf("merge lost: %+v err=%v", card, err)
	}
}
