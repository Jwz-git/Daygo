package ai

import (
	"context"
	"errors"
	"testing"
)

func chainEntries(providers ...Provider) []ChainEntry {
	entries := make([]ChainEntry, len(providers))
	for i, p := range providers {
		entries[i] = ChainEntry{ID: string(rune('a' + i)), Provider: p}
	}
	return entries
}

// Within a single turn the chain tries the next entry as soon as one fails,
// regardless of the threshold — the threshold governs where the NEXT turn
// starts.
func TestChainFallsThroughWithinOneTurn(t *testing.T) {
	primary := &sequenceProvider{errors: []error{NewError(ErrorUnavailable, "offline", 503, nil)}}
	secondary := &sequenceProvider{results: []Result{{Text: "ok"}}}
	chain := NewChain(chainEntries(primary, secondary), DefaultChainThreshold)

	result, err := chain.Generate(context.Background(), Request{})
	if err != nil || result.Text != "ok" {
		t.Fatalf("result = %#v, %v", result, err)
	}
}

// Three consecutive primary failures (the threshold) demote it: the next turn
// starts at the secondary without calling the primary first.
func TestChainDemotesAfterThreshold(t *testing.T) {
	primary := &sequenceProvider{errors: []error{
		NewError(ErrorUnavailable, "one", 503, nil),
		NewError(ErrorUnavailable, "two", 503, nil),
		NewError(ErrorUnavailable, "three", 503, nil),
		NewError(ErrorUnavailable, "should-not-be-called", 503, nil),
	}}
	secondary := &sequenceProvider{results: []Result{{Text: "s1"}, {Text: "s2"}}}
	chain := NewChain(chainEntries(primary, secondary), 3)

	// Turn 1: primary fails (failure 1), secondary succeeds; cursor returns to
	// primary because it succeeded? No — success moves the cursor to the
	// SUCCEEDING entry only in the promotion rule; here secondary succeeded, so
	// the cursor points at secondary already.
	if _, err := chain.Generate(context.Background(), Request{}); err != nil {
		t.Fatalf("turn 1: %v", err)
	}
	// Cursor is at the secondary (it succeeded), so turns 2 and 3 hit the
	// secondary without touching the primary at all.

	if got := chain.ActiveID(); got != "b" {
		t.Fatalf("after secondary success, active = %q, want b", got)
	}
	if primary.calls != 1 {
		t.Fatalf("primary calls = %d, want 1 (cursor moved on success)", primary.calls)
	}
}

// Reaching the threshold demotes: the cursor advances past the failing entry
// even though the failing entry itself did not succeed in the meantime.
func TestChainThresholdDemotesCursor(t *testing.T) {
	// Both entries fail on every call; threshold 2.
	primary := &countingFailingProvider{}
	secondary := &countingFailingProvider{}
	chain := NewChain(chainEntries(primary, secondary), 2)

	// Turn 1: primary (failure 1), secondary (failure 1). Both below
	// threshold; cursor stays at primary.
	_, _ = chain.Generate(context.Background(), Request{})
	// Turn 2: primary (failure 2 → demote, cursor→secondary), secondary
	// (failure 2 → demote, cursor→primary, wrapping).
	_, _ = chain.Generate(context.Background(), Request{})
	// Turn 3: cursor at primary again (secondary's own threshold wrapped it).
	_, _ = chain.Generate(context.Background(), Request{})

	if primary.calls != 3 || secondary.calls != 3 {
		t.Fatalf("calls = primary:%d secondary:%d, want 3/3 (cycling)", primary.calls, secondary.calls)
	}
}

// A demoted provider recovers by succeeding: the cursor promotes back to it.
func TestChainSuccessPromotesCursor(t *testing.T) {
	primary := &sequenceProvider{errors: []error{
		NewError(ErrorUnavailable, "one", 503, nil),
		NewError(ErrorUnavailable, "two", 503, nil),
	}}
	secondary := &sequenceProvider{results: []Result{{Text: "s1"}}}
	chain := NewChain(chainEntries(primary, secondary), 2)

	// Turn 1: primary fails (1), secondary succeeds → cursor at secondary.
	if _, err := chain.Generate(context.Background(), Request{}); err != nil {
		t.Fatalf("turn 1: %v", err)
	}
	// Turn 2: cursor at secondary.
	if _, err := chain.Generate(context.Background(), Request{}); err != nil {
		t.Fatalf("turn 2: %v", err)
	}
	if primary.calls != 1 {
		t.Fatalf("primary calls = %d, want 1", primary.calls)
	}

	// Rebuild the primary so it succeeds, then let the chain wrap around to
	// it: every turn walks the full cycle when the cursor entry fails, so a
	// failing secondary eventually re-tries the primary.
	recovered := &sequenceProvider{results: []Result{{Text: "back"}}}
	chain.Rebuild([]ChainEntry{{ID: "a", Provider: recovered}, {ID: "b", Provider: secondary}})
	// Primary's failure counter (1) survives the rebuild.

	// Turn 3: cursor is at secondary ("b"); make it fail so the turn wraps to
	// the recovered primary.
	failingSecondary := &sequenceProvider{errors: []error{NewError(ErrorUnavailable, "s-down", 503, nil)}}
	chain.Rebuild([]ChainEntry{{ID: "a", Provider: recovered}, {ID: "b", Provider: failingSecondary}})

	result, err := chain.Generate(context.Background(), Request{})
	if err != nil || result.Text != "back" {
		t.Fatalf("turn 3 = %#v, %v", result, err)
	}
	if got := chain.ActiveID(); got != "a" {
		t.Fatalf("active after primary recovery = %q, want a", got)
	}
}

// All entries failing within one cycle returns the last error, and the turn
// makes exactly one pass (n attempts for n entries).
func TestChainFullCycleFailureReturnsLastError(t *testing.T) {
	primary := &sequenceProvider{errors: []error{NewError(ErrorAuthentication, "bad key", 401, nil)}}
	secondary := &sequenceProvider{errors: []error{NewError(ErrorUnavailable, "down", 503, nil)}}
	chain := NewChain(chainEntries(primary, secondary), 3)

	_, err := chain.Generate(context.Background(), Request{})
	if err == nil {
		t.Fatal("expected an error when every entry fails")
	}
	var aiErr *Error
	if !errors.As(err, &aiErr) || aiErr.Kind != ErrorUnavailable {
		t.Fatalf("err = %v, want the LAST entry's error (unavailable)", err)
	}
	if primary.calls != 1 || secondary.calls != 1 {
		t.Fatalf("calls = primary:%d secondary:%d, want one pass each", primary.calls, secondary.calls)
	}
}

// Non-retryable errors (auth) still count and still advance within the turn:
// an auth-dead provider is exactly what fallback exists for.
func TestChainCountsNonRetryableErrors(t *testing.T) {
	primary := &sequenceProvider{errors: []error{NewError(ErrorAuthentication, "bad key", 401, nil)}}
	secondary := &sequenceProvider{results: []Result{{Text: "ok"}}}
	chain := NewChain(chainEntries(primary, secondary), 1)

	if _, err := chain.Generate(context.Background(), Request{}); err != nil {
		t.Fatalf("turn 1: %v", err)
	}
	if got := chain.ActiveID(); got != "b" {
		t.Fatalf("active after auth failure at threshold 1 = %q, want b", got)
	}
}

// Cancelling the whole call mid-walk does not count as a provider failure.
func TestChainCancellationDoesNotCount(t *testing.T) {
	primary := &canceledProvider{}
	secondary := &sequenceProvider{results: []Result{{Text: "ok"}}}
	chain := NewChain(chainEntries(primary, secondary), 1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := chain.Generate(ctx, Request{})
	if err == nil {
		t.Fatal("expected an error from a canceled context")
	}
	if primary.calls != 0 {
		t.Fatalf("primary calls = %d; the canceled turn must not call any entry", primary.calls)
	}
	if got := chain.ActiveID(); got != "a" {
		t.Fatalf("active after cancel = %q; cancellation must not demote", got)
	}
}

func TestChainEmptyReturnsErrNoProvider(t *testing.T) {
	chain := NewChain(nil, 3)

	if _, err := chain.Generate(context.Background(), Request{}); !errors.Is(err, ErrNoProvider) {
		t.Fatalf("err = %v, want ErrNoProvider", err)
	}
	if got := chain.ActiveID(); got != "" {
		t.Fatalf("active on empty chain = %q, want empty", got)
	}
}

// Rebuild keeps failure counters by ID; removed entries lose theirs.
func TestChainRebuildPreservesCounters(t *testing.T) {
	primary := &sequenceProvider{errors: []error{
		NewError(ErrorUnavailable, "one", 503, nil),
		NewError(ErrorUnavailable, "two", 503, nil),
		NewError(ErrorUnavailable, "three", 503, nil),
	}}
	secondary := &sequenceProvider{results: []Result{{Text: "s1"}}}
	chain := NewChain(chainEntries(primary, secondary), 3)

	// One turn: primary fails once (counter=1), secondary succeeds.
	if _, err := chain.Generate(context.Background(), Request{}); err != nil {
		t.Fatalf("turn 1: %v", err)
	}

	// Rebuild with the SAME ids but fresh instances: the primary's counter (1)
	// must survive so one more failure (2) is still below the threshold.
	freshPrimary := &sequenceProvider{errors: []error{NewError(ErrorUnavailable, "again", 503, nil)}}
	freshSecondary := &sequenceProvider{results: []Result{{Text: "s2"}}}
	chain.Rebuild([]ChainEntry{{ID: "a", Provider: freshPrimary}, {ID: "b", Provider: freshSecondary}})
	// Cursor was at "b" (secondary succeeded); it survives the rebuild.

	if got := chain.ActiveID(); got != "b" {
		t.Fatalf("active after rebuild = %q, want b (cursor survives)", got)
	}
}

// A single-entry chain degenerates to retry-only behavior.
func TestChainSingleEntry(t *testing.T) {
	provider := &sequenceProvider{results: []Result{{Text: "ok"}}}
	chain := NewChain([]ChainEntry{{ID: "only", Provider: provider}}, 3)

	result, err := chain.Generate(context.Background(), Request{})
	if err != nil || result.Text != "ok" {
		t.Fatalf("result = %#v, %v", result, err)
	}
}

type countingFailingProvider struct {
	calls int
}

func (p *countingFailingProvider) Generate(context.Context, Request) (Result, error) {
	p.calls++
	return Result{}, NewError(ErrorUnavailable, "always down", 503, nil)
}

type canceledProvider struct {
	calls int
}

func (p *canceledProvider) Generate(context.Context, Request) (Result, error) {
	p.calls++
	return Result{}, NewError(ErrorCanceled, "canceled", 0, nil)
}
