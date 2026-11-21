package analysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/domain"
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
	s := &Service{}
	start := time.Unix(1760000000, 0)
	end := start.Add(time.Hour)
	release, err := s.lockCardRewrite(t.Context(), start, end)
	if err != nil {
		t.Fatal(err)
	}
	parent, cancel := context.WithCancel(context.Background())
	ctx := mutationContext{Context: parent, checked: make(chan struct{})}
	done := make(chan error, 1)
	called := false
	go func() {
		done <- s.WithCardMutation(ctx, start.Add(-CardLookback-time.Hour), start, func() error { called = true; return nil })
	}()
	<-ctx.checked
	disjoint, cancelDisjoint := context.WithTimeout(t.Context(), time.Second)
	defer cancelDisjoint()
	expected := errors.New("fixture")
	nextDay := start.Add(24 * time.Hour)
	if err := s.WithCardMutation(disjoint, nextDay, nextDay.Add(time.Hour), func() error { return expected }); !errors.Is(err, expected) {
		cancel()
		release()
		<-done
		t.Fatalf("disjoint mutation waited for an active rewrite: %v", err)
	}
	cancel()
	select {
	case err := <-done:
		release()
		if !errors.Is(err, context.Canceled) || called {
			t.Fatalf("err=%v called=%v", err, called)
		}
	case <-time.After(time.Second):
		release()
		<-done
		t.Fatal("canceled mutation waited for the card lock to release")
	}
	if err := s.WithCardMutation(context.Background(), start, end, func() error { return expected }); !errors.Is(err, expected) {
		t.Fatal(err)
	}
}

func TestCardRewriteReservationCoversStorageExpansion(t *testing.T) {
	from := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	to := from.Add(15 * time.Minute)
	window := cardRewriteRange(from, to)
	shells := []domain.CardShell{{Start: "4:00 PM", End: "4:15 PM"}}
	if !window.allowsRewrite(from, to, shells, time.UTC) {
		t.Fatal("shell touching the reserved boundary should fit")
	}
	shells[0].End = "4:16 PM"
	if window.allowsRewrite(from, to, shells, time.UTC) {
		t.Fatal("storage expansion must stay inside the reserved window")
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
