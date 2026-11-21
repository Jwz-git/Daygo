package analysis

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

var ErrCardChanged = errors.New("analysis: source card was replaced before regeneration")

// WithCardMutation serializes a merge of the selected cards with intersecting
// rewrites. The repository rechecks those card IDs inside the write transaction.
func (s *Service) WithCardMutation(ctx context.Context, from, to time.Time, mutate func() error) error {
	release, err := s.cardsGate.acquire(ctx, cardRange{from: from.Unix(), to: to.Unix()})
	if err != nil {
		return err
	}
	defer release()
	if err := ctx.Err(); err != nil {
		return err
	}
	return mutate()
}

type cardRange struct {
	from int64
	to   int64
}

type cardRangeGate struct {
	mu     sync.Mutex
	active map[cardRange]chan struct{}
}

func (g *cardRangeGate) acquire(ctx context.Context, wanted cardRange) (func(), error) {
	if wanted.to <= wanted.from {
		return nil, errors.New("analysis: empty card range")
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		g.mu.Lock()
		var wait chan struct{}
		for held, done := range g.active {
			if held.from < wanted.to && wanted.from < held.to {
				wait = done
				break
			}
		}
		if wait == nil {
			done := make(chan struct{})
			if g.active == nil {
				g.active = make(map[cardRange]chan struct{})
			}
			g.active[wanted] = done
			g.mu.Unlock()
			return func() {
				g.mu.Lock()
				delete(g.active, wanted)
				close(done)
				g.mu.Unlock()
			}, nil
		}
		g.mu.Unlock()
		select {
		case <-wait:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// Allow for cards that cross either edge of the prompt's lookback and for
// adjacent folds. Check resolved clock strings against the same reservation
// before storage can expand its deletion range.
func cardRewriteRange(from, to time.Time) cardRange {
	return cardRange{
		from: from.Add(-CardLookback - 2*storage.MaxCardDuration).Unix(),
		to:   to.Add(storage.MaxCardDuration).Unix(),
	}
}

func (s *Service) lockCardRewrite(ctx context.Context, from, to time.Time) (func(), error) {
	return s.cardsGate.acquire(ctx, cardRewriteRange(from, to))
}

func (r cardRange) allowsRewrite(from, to time.Time, shells []domain.CardShell, loc *time.Location) bool {
	if from.Unix() < r.from || to.Unix() > r.to {
		return false
	}
	anchor := from.Add(to.Sub(from) / 2)
	for _, shell := range shells {
		start, startErr := timeutil.ResolveClock(shell.Start, anchor, loc)
		end, endErr := timeutil.ResolveClock(shell.End, anchor, loc)
		if startErr == nil && endErr == nil && end.After(start) && end.Sub(start) <= storage.MaxCardDuration &&
			(start.Unix() < r.from || end.Unix() > r.to) {
			return false
		}
	}
	return true
}
