package analysis

import (
	"context"
	"errors"
)

var ErrCardChanged = errors.New("analysis: source card was replaced before regeneration")

// WithCardMutation serializes a human card merge with the pipeline's
// read/generate/rewrite sequences and explicit regeneration. The callback
// must not call another analysis method that acquires the card gate.
func (s *Service) WithCardMutation(ctx context.Context, mutate func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.cardsGate.Acquire(ctx, 1); err != nil {
		return err
	}
	defer s.cardsGate.Release(1)
	if err := ctx.Err(); err != nil {
		return err
	}
	return mutate()
}
