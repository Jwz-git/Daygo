package analysis

import (
	"context"
	"fmt"

	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// ProbeResult is the in-memory result of running both analysis stages. Probe
// never changes batches, observations, cards, or recordings; it exists for
// diagnostics and local provider verification.
type ProbeResult struct {
	Observations []storage.Observation
	Cards        []domain.CardShell
}

// ProbeError identifies which production analysis stage failed. The wrapped
// error preserves its original classification for callers that need it.
type ProbeError struct {
	Stage string
	Err   error
}

func (e *ProbeError) Error() string {
	if e == nil {
		return "analysis probe failed"
	}
	return fmt.Sprintf("analysis probe %s: %v", e.Stage, e.Err)
}

func (e *ProbeError) Unwrap() error { return e.Err }

// Probe runs the same transcription and card-generation functions used by the
// background pipeline, with the supplied batch frames, but omits all writes
// and status transitions. The card lock is retained so the prompt context and
// generation obey the same serialization boundary as production.
func (s *Service) Probe(ctx context.Context, frames []storage.AnalysisFrame) (ProbeResult, error) {
	if len(frames) == 0 {
		return ProbeResult{}, &ProbeError{Stage: "input", Err: fmt.Errorf("no frames")}
	}
	chain, err := s.cfg.Providers.AnalysisChain(ctx)
	if err != nil {
		return ProbeResult{}, &ProbeError{Stage: "provider chain", Err: err}
	}
	observations, err := s.transcribe(ctx, chain, frames)
	if err != nil {
		return ProbeResult{}, &ProbeError{Stage: "transcription", Err: err}
	}
	if len(observations) == 0 {
		return ProbeResult{}, &ProbeError{Stage: "transcription", Err: fmt.Errorf("no usable observations")}
	}

	s.cardsMu.Lock()
	defer s.cardsMu.Unlock()
	start, end := frames[0].CapturedAt, frames[len(frames)-1].CapturedAt
	existing, err := s.cfg.Cards.CardsInRange(ctx, start.Add(-CardLookback), end)
	if err != nil {
		return ProbeResult{}, &ProbeError{Stage: "load nearby cards", Err: err}
	}
	categories, err := s.cfg.Categories.List(ctx)
	if err != nil {
		return ProbeResult{}, &ProbeError{Stage: "load categories", Err: err}
	}
	cards, _, err := s.generateCards(ctx, chain, storage.Batch{Start: start, End: end}, existing, observations, categories, start, false)
	if err != nil {
		return ProbeResult{Observations: observations}, &ProbeError{Stage: "card generation", Err: err}
	}
	return ProbeResult{Observations: observations, Cards: cards}, nil
}
