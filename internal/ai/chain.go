package ai

import (
	"context"
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
	ID       string
	Provider Provider
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
	cursor    int
	threshold int
	failures  map[string]int
}

// NewChain builds a chain. threshold <= 0 falls back to DefaultChainThreshold.
// An empty chain is valid to construct but every Generate returns
// ErrNoProvider.
func NewChain(entries []ChainEntry, threshold int) *Chain {
	if threshold <= 0 {
		threshold = DefaultChainThreshold
	}
	return &Chain{
		entries:   append([]ChainEntry(nil), entries...),
		threshold: threshold,
		failures:  make(map[string]int),
	}
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

	var cursorID string
	if c.cursor >= 0 && c.cursor < len(c.entries) {
		cursorID = c.entries[c.cursor].ID
	}
	c.entries = append([]ChainEntry(nil), entries...)
	c.cursor = 0
	for i, entry := range c.entries {
		if entry.ID == cursorID {
			c.cursor = i
			break
		}
	}
	// Drop counters for providers no longer in the chain.
	alive := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		alive[entry.ID] = struct{}{}
	}
	for id := range c.failures {
		if _, ok := alive[id]; !ok {
			delete(c.failures, id)
		}
	}
}

// ActiveID returns the ID of the entry the cursor currently points at, or ""
// when the chain is empty.
func (c *Chain) ActiveID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cursor < 0 || c.cursor >= len(c.entries) {
		return ""
	}
	return c.entries[c.cursor].ID
}

// Generate implements Provider by walking the chain (see the type comment for
// the full semantics).
func (c *Chain) Generate(ctx context.Context, request Request) (Result, error) {
	c.mu.Lock()
	entries := append([]ChainEntry(nil), c.entries...)
	start := c.cursor
	c.mu.Unlock()

	if len(entries) == 0 {
		return Result{}, ErrNoProvider
	}

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

		result, err := entry.Provider.Generate(ctx, request)
		if err == nil {
			c.recordSuccess(entry.ID, index)
			return result, nil
		}
		lastErr = err
		c.recordFailure(entry.ID, index)
	}
	return Result{}, lastErr
}

// recordSuccess resets the entry's counter and moves the cursor to it. This is
// the recovery path: a demoted provider that succeeds once is promoted back.
func (c *Chain) recordSuccess(id string, index int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures[id] = 0
	c.cursor = index
}

// recordFailure increments the entry's consecutive-failure count. Reaching the
// threshold demotes it: the cursor advances to the next surviving entry and
// stays there until this one succeeds again.
func (c *Chain) recordFailure(id string, index int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures[id]++
	if c.failures[id] >= c.threshold && c.cursor == index && len(c.entries) > 1 {
		c.cursor = (index + 1) % len(c.entries)
	}
}
