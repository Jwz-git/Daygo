package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// Consumer-side interfaces. *storage.AnalysisRepo, *storage.CardRepo, and
// *storage.CategoryRepo satisfy these structurally; tests use the same real
// repositories over a temp-dir database (docs/09 §9.2: fakes prove shape,
// the real stores prove SQL).

type Store interface {
	UnbatchedFrames(ctx context.Context, since, until time.Time) ([]storage.AnalysisFrame, error)
	CreateBatch(ctx context.Context, frames []storage.AnalysisFrame, status storage.BatchStatus, now time.Time) (storage.Batch, error)
	FramesForBatch(ctx context.Context, batchID int64) ([]storage.AnalysisFrame, error)
	PendingBatches(ctx context.Context) ([]storage.Batch, error)
	ProcessingBatchesInRange(ctx context.Context, from, to time.Time) ([]storage.Batch, error)
	SetBatchStatus(ctx context.Context, batchID int64, to storage.BatchStatus, failureKind, failureNote string, now time.Time) error
	AdoptStaleProcessing(ctx context.Context, now time.Time) (int, error)
	RequeueFailed(ctx context.Context, olderThan, now time.Time) (int, error)
	InsertObservations(ctx context.Context, batchID int64, obs []storage.Observation, now time.Time) error
	ObservationsForBatch(ctx context.Context, batchID int64) ([]storage.Observation, error)
	ObservationsInRange(ctx context.Context, from, to time.Time) ([]storage.Observation, error)
}

type CardStore interface {
	CardsInRange(ctx context.Context, from, to time.Time) ([]domain.TimelineCard, error)
	ReplaceCardsInRange(ctx context.Context, from, to time.Time, cards []domain.CardShell, batchID int64) (storage.ReplaceResult, error)
}

type CategorySource interface {
	List(ctx context.Context) ([]domain.Category, error)
}

// ChainSource rebuilds the provider chain per call; the app layer owns the
// routing and secrets wiring (mirrors chat's rebuildChain).
type ChainSource interface {
	AnalysisChain(ctx context.Context) (*ai.Chain, error)
}

// FrameSource reads one frame's pixels. Provisional until the Media port's
// format decisions (#7/#8) land: the staging adapter reads single-frame
// JPEGs by segment path, which is all the current recorder produces.
type FrameSource interface {
	FrameBytes(ctx context.Context, segmentPath string, frameIndex int) ([]byte, error)
}

// Config assembles a Service. Now/TickInterval are injectable for tests.
type Config struct {
	Store            Store
	Cards            CardStore
	Categories       CategorySource
	Providers        ChainSource
	Media            FrameSource
	Language         func(ctx context.Context) string
	Now              func() time.Time
	TickEvery        time.Duration
	Workers          int
	IdleRules        IdleRules
	OnCardsCommitted func(days []string)
	OnBatchFailed    func(batch storage.Batch, kind, note string)
}

type Service struct {
	cfg     Config
	cardsMu sync.Mutex // serializes the read→generate→rewrite card sequence (docs/04 §4.3.2)
}

func New(cfg Config) (*Service, error) {
	if cfg.Store == nil || cfg.Cards == nil || cfg.Categories == nil ||
		cfg.Providers == nil || cfg.Media == nil {
		return nil, fmt.Errorf("analysis: config requires store, cards, categories, providers and media")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.TickEvery <= 0 {
		cfg.TickEvery = time.Minute
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 2
	}
	if cfg.Language == nil {
		cfg.Language = func(context.Context) string { return "" }
	}
	if cfg.IdleRules == (IdleRules{}) {
		cfg.IdleRules = DefaultIdleRules()
	}
	return &Service{cfg: cfg}, nil
}

// Run is the scheduler loop (docs/04 §4.3). It blocks until ctx is done and
// must be owned by the app lifetime, like storage.Maintainer. On cancellation
// an in-flight batch stays processing: the next startup adopts it.
func (s *Service) Run(ctx context.Context) {
	now := s.cfg.Now()
	if _, err := s.cfg.Store.AdoptStaleProcessing(ctx, now); err != nil {
		// Adoption is best-effort at startup: a stale batch left in processing
		// is adopted on a later run, never lost.
		_ = err
	}

	ticker := time.NewTicker(s.cfg.TickEvery)
	defer ticker.Stop()
	for {
		s.tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) tick(ctx context.Context) {
	now := s.cfg.Now()

	// Requeue failed batches past their cooldown.
	if _, err := s.cfg.Store.RequeueFailed(ctx, now.Add(-FailureRetryCooldown), now); err != nil {
		return
	}

	// Drain pending batches oldest-first before creating new ones. One pass
	// per tick is enough: batches this pass marked failed stay failed until
	// the cooldown, and new pending batches created below run immediately.
	batches, err := s.cfg.Store.PendingBatches(ctx)
	if err == nil {
		for _, batch := range batches {
			if ctx.Err() != nil {
				return
			}
			if err := s.processBatch(ctx, batch); err != nil {
				if ctx.Err() != nil {
					// Canceled mid-flight: the batch stays processing for the
					// next startup to adopt. Not a failure.
					return
				}
				s.failBatch(ctx, batch, err)
			}
		}
	}

	// Discover unbatched frames and create batches.
	frames, err := s.cfg.Store.UnbatchedFrames(ctx, now.Add(-UnbatchedLookback), now)
	if err != nil || len(frames) == 0 {
		return
	}
	split := SplitFrames(frames)
	for _, plan := range split.Closed {
		if plan.Span() < MinAnalysisDuration {
			if _, err := s.cfg.Store.CreateBatch(ctx, plan.Frames, storage.BatchSkippedShort, now); err != nil {
				return
			}
			continue
		}
		batch, err := s.cfg.Store.CreateBatch(ctx, plan.Frames, storage.BatchPending, now)
		if err != nil {
			return
		}
		if err := s.processBatch(ctx, batch); err != nil {
			if ctx.Err() != nil {
				return
			}
			s.failBatch(ctx, batch, err)
		}
	}
	// The latest run stays unbatched until its span reaches the target
	// (off-by-one-interval rule); a run that already reached it qualifies.
	if split.Latest.Span() >= TargetBatchDuration {
		batch, err := s.cfg.Store.CreateBatch(ctx, split.Latest.Frames, storage.BatchPending, now)
		if err != nil {
			return
		}
		if err := s.processBatch(ctx, batch); err != nil {
			if ctx.Err() != nil {
				return
			}
			s.failBatch(ctx, batch, err)
		}
	}
}

// processBatch runs one batch through the pipeline. A returned error fails
// the batch; context cancellation returns ctx.Err() with the batch left in
// processing for the next startup.
func (s *Service) processBatch(ctx context.Context, batch storage.Batch) error {
	if err := s.cfg.Store.SetBatchStatus(ctx, batch.ID, storage.BatchProcessing, "", "", s.cfg.Now()); err != nil {
		return err
	}
	frames, err := s.cfg.Store.FramesForBatch(ctx, batch.ID)
	if err != nil {
		return err
	}
	if len(frames) == 0 {
		return fmt.Errorf("batch %d has no frames", batch.ID)
	}

	// Idle fast path: no LLM call at all (docs/04 §4.4).
	if DetectIdle(frames, s.cfg.IdleRules) {
		return s.commitIdleCard(ctx, batch)
	}

	chain, err := s.cfg.Providers.AnalysisChain(ctx)
	if err != nil {
		return err
	}

	// Transcription: consecutive groups of at most ai.MaxImages frames.
	observations, err := s.transcribe(ctx, chain, batch, frames)
	if err != nil {
		return err
	}
	if len(observations) == 0 {
		return s.cfg.Store.SetBatchStatus(ctx, batch.ID, storage.BatchFailedEmpty,
			"empty", "transcription produced no observations", s.cfg.Now())
	}
	if err := s.cfg.Store.InsertObservations(ctx, batch.ID, observations, s.cfg.Now()); err != nil {
		return err
	}

	// Card generation holds cardsMu across read→generate→rewrite so two
	// batches never interleave a rewrite of the same range (docs/04 §4.3.2).
	s.cardsMu.Lock()
	defer s.cardsMu.Unlock()

	lookbackStart := batch.Start.Add(-CardLookback)
	existing, err := s.cfg.Cards.CardsInRange(ctx, lookbackStart, batch.End)
	if err != nil {
		return err
	}
	categories, err := s.cfg.Categories.List(ctx)
	if err != nil {
		return err
	}

	shells, err := s.generateCards(ctx, chain, batch, existing, observations, categories)
	if err != nil {
		return err
	}
	result, err := s.cfg.Cards.ReplaceCardsInRange(ctx, batch.Start, batch.End, shells, batch.ID)
	if err != nil {
		return err
	}
	// SkippedCards is a defect signal, never a silent drop (docs/03 §3.5);
	// the counter lives in storage.NoteSkippedCards. A shell the model
	// resolved entirely outside the window is filtered before this point.
	if len(result.SkippedCards) > 0 {
		return fmt.Errorf("batch %d skipped %d unresolvable cards", batch.ID, len(result.SkippedCards))
	}

	if err := s.cfg.Store.SetBatchStatus(ctx, batch.ID, storage.BatchSucceeded, "", "", s.cfg.Now()); err != nil {
		return err
	}
	s.notifyDays(batch.Start, batch.End)
	return nil
}

// commitIdleCard writes the Idle card directly, merging with a directly
// preceding Idle card when close enough (docs/04 §4.4).
func (s *Service) commitIdleCard(ctx context.Context, batch storage.Batch) error {
	replaceFrom := batch.Start
	shell := domain.CardShell{
		Start:    timeutil.FormatClock(batch.Start, time.Local),
		End:      timeutil.FormatClock(batch.End, time.Local),
		Category: "Idle",
		Title:    "Idle",
		Summary:  "No user activity detected during this period.",
	}
	// Merge with a preceding Idle card within AdjacentIdleMergeGap.
	preceding, err := s.cfg.Cards.CardsInRange(ctx,
		batch.Start.Add(-s.cfg.IdleRules.AdjacentIdleMergeGap-time.Minute), batch.Start)
	if err == nil {
		for _, card := range slices.Backward(preceding) {
			if card.Category == "Idle" && card.EndTs <= batch.Start.Unix() &&
				batch.Start.Unix()-card.EndTs <= int64(s.cfg.IdleRules.AdjacentIdleMergeGap.Seconds()) {
				replaceFrom = time.Unix(card.StartTs, 0)
				shell.Start = card.Start
				break
			}
		}
	}
	if _, err := s.cfg.Cards.ReplaceCardsInRange(ctx, replaceFrom, batch.End,
		[]domain.CardShell{shell}, batch.ID); err != nil {
		return err
	}
	if err := s.cfg.Store.SetBatchStatus(ctx, batch.ID, storage.BatchSucceeded, "", "", s.cfg.Now()); err != nil {
		return err
	}
	s.notifyDays(replaceFrom, batch.End)
	return nil
}

// transcribe groups frames and runs the transcription stage with bounded
// parallelism. Attempt metadata ties llm_calls rows to the batch.
func (s *Service) transcribe(ctx context.Context, chain *ai.Chain, batch storage.Batch,
	frames []storage.AnalysisFrame) ([]storage.Observation, error) {

	groups := groupFrames(frames)
	type outcome struct {
		obs []storage.Observation
		err error
	}
	results := make([]outcome, len(groups))
	sem := make(chan struct{}, s.cfg.Workers)
	var wg sync.WaitGroup
	batchID := batch.ID
	ctx = ai.WithAttemptMetadata(ctx, ai.AttemptMetadata{BatchID: &batchID})

	for i, group := range groups {
		wg.Add(1)
		go func(i int, group []storage.AnalysisFrame) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			obs, err := s.transcribeGroup(ctx, chain, group)
			results[i] = outcome{obs: obs, err: err}
		}(i, group)
	}
	wg.Wait()

	var all []storage.Observation
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
		all = append(all, r.obs...)
	}
	return all, nil
}

func (s *Service) transcribeGroup(ctx context.Context, chain *ai.Chain,
	group []storage.AnalysisFrame) ([]storage.Observation, error) {

	prompt := transcribePrompt(group, s.cfg.Language(ctx))
	parts := []ai.Part{ai.TextPart(prompt)}
	for _, f := range group {
		data, err := s.cfg.Media.FrameBytes(ctx, f.SegmentPath, f.FrameIndex)
		if err != nil {
			return nil, fmt.Errorf("read frame %s: %w", f.SegmentPath, err)
		}
		part, err := ai.ImagePart(ai.MediaJPEG, data)
		if err != nil {
			return nil, fmt.Errorf("frame %s: %w", f.SegmentPath, err)
		}
		parts = append(parts, part)
	}

	request := ai.Request{
		Purpose:         ai.PurposeTranscribe,
		Parts:           parts,
		Output:          &transcribeOutput,
		MaxOutputTokens: 3072,
	}
	result, err := chain.Generate(ctx, request)
	if err != nil {
		return nil, err
	}
	raw, err := ai.ParseStructuredOutput(result.Text, transcribeOutput)
	if err != nil {
		return nil, err
	}
	var envelope transcribeEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode transcriptions: %w", err)
	}

	var out []storage.Observation
	for _, o := range envelope.Observations {
		if o.Observation == "" {
			continue
		}
		from, to := clampFrameRange(o.FromFrame, o.ToFrame, len(group))
		if from > to {
			continue
		}
		metadata, _ := json.Marshal(map[string]any{"apps": o.Apps})
		out = append(out, storage.Observation{
			Start:       group[from].CapturedAt,
			End:         group[to].CapturedAt,
			Observation: o.Observation,
			Metadata:    string(metadata),
		})
	}
	return out, nil
}

// generateCards runs the card stage: prompt with sliding-window context,
// parse, then validate every category against the known list — an unknown
// category maps to System and is counted, never auto-created (docs/04 §4.3.4).
func (s *Service) generateCards(ctx context.Context, chain *ai.Chain, batch storage.Batch,
	existing []domain.TimelineCard, obs []storage.Observation,
	categories []domain.Category) ([]domain.CardShell, error) {

	prompt := cardsPrompt(batch.Start, batch.End, existing, obs, categories, s.cfg.Language(ctx))
	request := ai.Request{
		Purpose:         ai.PurposeCards,
		Parts:           []ai.Part{ai.TextPart(prompt)},
		Output:          &cardsOutput,
		MaxOutputTokens: 4096,
	}
	result, err := chain.Generate(ctx, request)
	if err != nil {
		return nil, err
	}
	raw, err := ai.ParseStructuredOutput(result.Text, cardsOutput)
	if err != nil {
		return nil, err
	}
	var envelope cardsEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode cards: %w", err)
	}

	known := make(map[string]bool, len(categories))
	for _, c := range categories {
		known[c.Name] = true
	}

	var shells []domain.CardShell
	for _, c := range envelope.Cards {
		category := c.Category
		if !known[category] {
			category = "System"
		}
		metadata, _ := json.Marshal(map[string]any{
			"appSites":     c.AppSites,
			"distractions": c.Distractions,
		})
		shell := domain.CardShell{
			Start:           c.Start,
			End:             c.End,
			Category:        category,
			Subcategory:     c.Subcategory,
			Title:           c.Title,
			Summary:         c.Summary,
			DetailedSummary: c.DetailedSummary,
			Metadata:        string(metadata),
		}
		// A card the model resolved entirely outside the batch window is a
		// context-merge hallucination; drop it rather than let the rewrite
		// duplicate it outside the range we own.
		if s.shellOverlapsWindow(shell, batch) {
			shells = append(shells, shell)
		}
	}
	return shells, nil
}

// shellOverlapsWindow pre-resolves the shell's clocks and keeps only cards
// overlapping the batch window. Shells whose clocks do not resolve at all are
// kept — ReplaceCardsInRange reports them as SkippedCards, which fails the
// batch loudly instead of silently here.
func (s *Service) shellOverlapsWindow(shell domain.CardShell, batch storage.Batch) bool {
	loc := time.Local
	anchor := batch.Start.Add(batch.End.Sub(batch.Start) / 2)
	start, err := timeutil.ResolveClock(shell.Start, anchor, loc)
	if err != nil {
		return true
	}
	end, err := timeutil.ResolveClock(shell.End, anchor, loc)
	if err != nil {
		return true
	}
	if end.Before(start) {
		end = end.AddDate(0, 0, 1)
	}
	return start.Unix() < batch.End.Unix() && end.Unix() > batch.Start.Unix()
}

// groupFrames slices time-ordered frames into consecutive groups of at most
// ai.MaxImages with a total-size safety budget below ai.MaxTotalBytes.
func groupFrames(frames []storage.AnalysisFrame) [][]storage.AnalysisFrame {
	if len(frames) == 0 {
		return nil
	}
	const byteBudget = 18 << 20 // headroom below ai.MaxTotalBytes for prompt text
	var groups [][]storage.AnalysisFrame
	current := []storage.AnalysisFrame{frames[0]}
	total := frames[0].FileSize
	for _, f := range frames[1:] {
		if len(current) >= ai.MaxImages || (f.FileSize > 0 && total+f.FileSize > byteBudget) {
			groups = append(groups, current)
			current = []storage.AnalysisFrame{f}
			total = f.FileSize
			continue
		}
		current = append(current, f)
		total += f.FileSize
	}
	return append(groups, current)
}

// clampFrameRange maps model frame indices onto the group, tolerating
// out-of-range values by clamping; a reversed range yields from > to.
func clampFrameRange(from, to, n int) (int, int) {
	clamp := func(i int) int {
		if i < 0 {
			return 0
		}
		if i > n-1 {
			return n - 1
		}
		return i
	}
	return clamp(from), clamp(to)
}

// failBatch records the failure with a user-facing kind and notifies.
func (s *Service) failBatch(ctx context.Context, batch storage.Batch, err error) {
	kind := failureKind(err)
	note := "analysis failed"
	if msg := err.Error(); msg != "" {
		note = truncate(msg, 200)
	}
	if err := s.cfg.Store.SetBatchStatus(ctx, batch.ID, storage.BatchFailed, kind, note, s.cfg.Now()); err != nil {
		return
	}
	if s.cfg.OnBatchFailed != nil {
		s.cfg.OnBatchFailed(batch, kind, note)
	}
}

// failureKind maps an error to the user-facing failure classification of
// docs/04 §4.3.3. Notes carry only already-sanitized fixed strings.
func failureKind(err error) string {
	switch ai.ErrorKindOf(err) {
	case ai.ErrorAuthentication:
		return "auth"
	case ai.ErrorRateLimited:
		return "rate_limited"
	case ai.ErrorTimeout, ai.ErrorUnavailable:
		return "network"
	case ai.ErrorInvalidOutput:
		return "invalid_output"
	case ai.ErrorInvalidRequest, ai.ErrorUnsupportedFeature:
		return "invalid_request"
	}
	var storageErr error = err
	_ = storageErr
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	return "internal"
}

// notifyDays emits the logical days a commit touched — one, or two when the
// window crosses the 4 AM boundary.
func (s *Service) notifyDays(from, to time.Time) {
	if s.cfg.OnCardsCommitted == nil {
		return
	}
	loc := time.Local
	days := []string{timeutil.LogicalDay(from, loc)}
	if end := timeutil.LogicalDay(to, loc); end != days[0] {
		days = append(days, end)
	}
	s.cfg.OnCardsCommitted(days)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
