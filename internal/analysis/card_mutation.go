package analysis

import (
	"context"
	"errors"
)

var ErrCardChanged = errors.New("analysis: source card was replaced before regeneration")

// WithCardMutation serializes a human card merge with the pipeline's
// read/generate/rewrite sequences and explicit regeneration. The callback
// must not call another analysis method that acquires cardsMu.
func (s *Service) WithCardMutation(ctx context.Context, mutate func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.cardsMu.Lock()
	defer s.cardsMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return mutate()
}
