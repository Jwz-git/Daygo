package ai

import (
	"context"
	"slices"
	"sync"
)

// DefaultChainThreshold is the number of consecutive failed Generate calls
// (after each entry's own retry policy is exhausted) that demotes a provider
// in the chain: once reached, the cursor advances to the next entry until the
// demoted one succeeds again. It is a constant, not a setting — the decision
// record (decisions/providers-fallback-chain) keeps it out of the UI on
// purpose.
const DefaultChainThreshold = 3

// ChainEntry is one provider in the fallback chain. ID keys the failure
// counters, so entries can be swapped without losing demotion history;
// Provider is expected to be pre-wrapped with WithRetry.
type ChainEntry struct {
	ID           string
	Provider     Provider
	PrefersVideo bool
}

// Chain routes Generate calls through an ordered provider list with cycling
// fallback (decisions/providers-fallback-chain):
//
//   - state is in-memory and keyed by provider ID; it resets on restart;
//   - each Generate walks entries starting at the cursor, wrapping around,
//     at most one full cycle;
//   - a failed entry increments its consecutive-failure count and the turn
//     continues with the next entry; reaching the threshold demotes it by
//     advancing the cursor, which stays there until that entry succeeds;
//   - a success resets that entry's counter and moves the cursor to it;
//   - cancelling the whole call does not count as a provider failure;
//   - if every entry fails within one cycle, the last error is returned.
type Chain struct {
	mu        sync.Mutex
	entries   []ChainEntry
	threshold int
	history   *ChainState
	state     *chainProgress
}

// ChainState keeps routing progress across freshly constructed clients. It
// holds provider IDs and counters only. Each distinct ordered route gets a
// progress snapshot so requests in flight can finish against the old route.
type ChainState struct {
	mu      sync.Mutex
	ids     []string
	current *chainProgress
}

type chainProgress struct {
	mu       sync.Mutex
	cursor   int
	failures map[string]int
}

// NewChain builds a chain. threshold <= 0 falls back to DefaultChainThreshold.
// An empty chain is valid to construct but every Generate returns
// ErrNoProvider.
func NewChain(entries []ChainEntry, threshold int) *Chain {
	return (&ChainState{}).NewChain(entries, threshold)
}

// NewChain uses the current routing progress with this batch's own clients.
func (s *ChainState) NewChain(entries []ChainEntry, threshold int) *Chain {
	if threshold <= 0 {
		threshold = DefaultChainThreshold
	}
	return &Chain{
		entries:   append([]ChainEntry(nil), entries...),
		threshold: threshold,
		history:   s,
		state:     s.forEntries(entries),
	}
}

func (s *ChainState) forEntries(entries []ChainEntry) *chainProgress {
	ids := make([]string, len(entries))
	for i, entry := range entries {
		ids[i] = entry.ID
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != nil && slices.Equal(s.ids, ids) {
		return s.current
	}

	next := &chainProgress{failures: make(map[string]int, len(ids))}
	if current := s.current; current != nil {
		current.mu.Lock()
		if current.cursor < len(s.ids) {
			next.cursor = max(slices.Index(ids, s.ids[current.cursor]), 0)
		}
		for _, id := range ids {
			next.failures[id] = current.failures[id]
		}
		current.mu.Unlock()
	}
	s.ids, s.current = ids, next
	return next
}

// ErrNoProvider is returned by Generate when the chain holds no entries. It is
// a sentinel, not a classified ai.Error: the caller maps it to the binding
// layer's provider_not_configured.
var ErrNoProvider = &chainError{}

type chainError struct{}

func (*chainError) Error() string { return "ai: no provider configured" }

// Rebuild swaps the entry list, keeping failure counters by ID so an edit
// mid-session does not erase demotion history. Counters of entries whose IDs
// vanished are dropped. If the current cursor's entry was removed, the cursor
// resets to the head of the new list.
func (c *Chain) Rebuild(entries []ChainEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = append([]ChainEntry(nil), entries...)
	c.state = c.history.forEntries(entries)
}

// ActiveID returns the ID of the entry the cursor currently points at, or ""
// when the chain is empty.
func (c *Chain) ActiveID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	if c.state.cursor >= len(c.entries) {
		return ""
	}
	return c.entries[c.state.cursor].ID
}

// PrefersVideo reports whether any entry can use the alternate video request.
func (c *Chain) PrefersVideo() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, entry := range c.entries {
		if entry.PrefersVideo {
			return true
		}
	}
	return false
}

// Generate implements Provider using a fixed request for every entry.
func (c *Chain) Generate(ctx context.Context, request Request) (Result, error) {
	return c.GeneratePrepared(ctx, func(bool) (Request, error) { return request, nil })
}

// GeneratePrepared selects the request for each target before its retry
// wrapper runs. A local preparation error is not counted against the target.
func (c *Chain) GeneratePrepared(ctx context.Context, prepare func(prefersVideo bool) (Request, error)) (Result, error) {
	c.mu.Lock()
	entries, state := c.entries, c.state
	c.mu.Unlock()

	if len(entries) == 0 {
		return Result{}, ErrNoProvider
	}
	state.mu.Lock()
	start := state.cursor
	state.mu.Unlock()

	var lastErr error
	for offset := range entries {
		index := (start + offset) % len(entries)
		entry := entries[index]

		// Cancellation of the whole call is the user's doing, not a provider
		// failure: return without touching counters.
		if ctx.Err() != nil {
			if lastErr == nil {
				return Result{}, canceledError(ctx.Err())
			}
			return Result{}, lastErr
		}

		request, err := prepare(entry.PrefersVideo)
		if err != nil {
			return Result{}, err
		}
		if err := ctx.Err(); err != nil {
			return Result{}, canceledError(err)
		}
		result, err := entry.Provider.Generate(ctx, request)
		if err == nil {
			state.recordSuccess(entry.ID, index)
			return result, nil
		}
		// Cancellation may also land mid-flight, inside the provider call
		// itself: the same not-a-provider-failure rule applies, and the walk
		// ends here without touching counters. The kind check catches a
		// provider-reported cancellation even when the shared context object
		// has not been canceled itself.
		if ctx.Err() != nil || ErrorKindOf(err) == ErrorCanceled {
			if lastErr == nil {
				return Result{}, canceledError(err)
			}
			return Result{}, lastErr
		}
		lastErr = err
		state.recordFailure(entry.ID, index, c.threshold, len(entries))
	}
	return Result{}, lastErr
}

// recordSuccess resets the entry's counter and moves the cursor to it. This is
// the recovery path: a demoted provider that succeeds once is promoted back.
func (s *chainProgress) recordSuccess(id string, index int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures[id] = 0
	s.cursor = index
}

// recordFailure increments the entry's consecutive-failure count. Reaching the
// threshold demotes it: the cursor advances to the next surviving entry and
// stays there until this one succeeds again.
func (s *chainProgress) recordFailure(id string, index, threshold, size int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures[id]++
	if s.failures[id] >= threshold && s.cursor == index && size > 1 {
		s.cursor = (index + 1) % size
	}
}
