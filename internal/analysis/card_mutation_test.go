package analysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/domain"

	"golang.org/x/sync/semaphore"
)

type mutationContext struct {
	context.Context
	checked chan struct{}
}

func (c mutationContext) Err() error {
	err := c.Context.Err()
	select {
	case <-c.checked:
	default:
		close(c.checked)
	}
	return err
}

func TestCardMutationSharesSerializationAndHonorsCancellation(t *testing.T) {
	s := &Service{cardsGate: semaphore.NewWeighted(1)}
	if err := s.cardsGate.Acquire(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	parent, cancel := context.WithCancel(context.Background())
	ctx := mutationContext{Context: parent, checked: make(chan struct{})}
	done := make(chan error, 1)
	called := false
	go func() { done <- s.WithCardMutation(ctx, func() error { called = true; return nil }) }()
	<-ctx.checked
	cancel()
	select {
	case err := <-done:
		s.cardsGate.Release(1)
		if !errors.Is(err, context.Canceled) || called {
			t.Fatalf("err=%v called=%v", err, called)
		}
	case <-time.After(time.Second):
		s.cardsGate.Release(1)
		<-done
		t.Fatal("canceled mutation waited for the card lock to release")
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
