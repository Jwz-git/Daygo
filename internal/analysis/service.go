package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

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
// routing and secrets wiring (mirrors chat's rebuildChain). ImageCap is the
// per-request image limit for transcription grouping: the minimum across the
// chain, so a request sized for one entry never exceeds another that may take
// it over mid-flight. 0 means the ai.MaxImages default.
type ChainSource interface {
	AnalysisChain(ctx context.Context) (*ai.Chain, error)
	ImageCap(ctx context.Context) int
}

// FrameSource reads one frame's pixels. Provisional until the Media port's
// format decisions (#7/#8) land: the staging adapter reads single-frame
// JPEGs by segment path, which is all the current recorder produces.
type FrameSource interface {
	FrameBytes(ctx context.Context, segmentPath string, frameIndex int) ([]byte, error)
}

// Config assembles a Service. Now/TickInterval are injectable for tests.
type Config struct {
	Store      Store
	Cards      CardStore
	Categories CategorySource
	Providers  ChainSource
	Media      FrameSource
	Language   func(ctx context.Context) string
	// ActiveSegment reports the recording segment currently being written, or
	// "" when none is active. A batch whose frames still live in that segment is
	// deferred rather than processed: its container is unfinalized and its frames
	// cannot be decoded yet, so processing would fail and burn a retry cooldown
	// until the segment happens to roll over. Nil means no active-segment
	// awareness (tests, headless read-only) — every batch runs immediately.
	ActiveSegment func() string
	Now           func() time.Time
	TickEvery     time.Duration
	Workers       int
	// BatchPacing is the pause between processing batches when draining
	// multiple pending or closed batches. 0 means no delay (used in tests).
	BatchPacing time.Duration
	IdleRules   IdleRules
	// Location is the zone for every local-time decision the service makes
	// (idle card clocks, out-of-window prefiltering, day notification). It
	// must match the storage layer's zone, because ReplaceCardsInRange
	// derives start_ts/end_ts/day there; a mismatch would prefilter with one
	// zone and insert with another. Nil means time.Local.
	Location         *time.Location
	OnCardsCommitted func(days []string)
	OnBatchFailed    func(batch storage.Batch, kind, note string)
}

type Service struct {
	cfg                   Config
	cardsMu               sync.Mutex // serializes the read→generate→rewrite card sequence (docs/04 §4.3.2)
	queueMu               sync.Mutex // guards queue pacing timestamps and rate limit tracking
	lastBatchProcessedAt  time.Time
	rateLimitBackoffUntil time.Time
	rateLimitCount        map[int64]int
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
		cfg.Workers = 1
	}
	if cfg.Language == nil {
		cfg.Language = func(context.Context) string { return "" }
	}
	if cfg.IdleRules == (IdleRules{}) {
		cfg.IdleRules = DefaultIdleRules()
	}
	if cfg.Location == nil {
		cfg.Location = time.Local
	}
	return &Service{
		cfg:            cfg,
		rateLimitCount: make(map[int64]int),
	}, nil
}

// loc is the single zone accessor for the service.
func (s *Service) loc() *time.Location { return s.cfg.Location }

// paceBatch ensures the configured BatchPacing delay has elapsed since the
// previous batch completed before starting the next batch in the queue.
func (s *Service) paceBatch(ctx context.Context) error {
	if s.cfg.BatchPacing <= 0 {
		return nil
	}
	s.queueMu.Lock()
	last := s.lastBatchProcessedAt
	s.queueMu.Unlock()
	if last.IsZero() {
		return nil
	}
	elapsed := s.cfg.Now().Sub(last)
	if elapsed >= s.cfg.BatchPacing {
		return nil
	}
	pause := s.cfg.BatchPacing - elapsed
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(pause):
		return nil
	}
}

func (s *Service) handleBatchRateLimit(ctx context.Context, batch storage.Batch, err error) {
	s.queueMu.Lock()
	s.rateLimitCount[batch.ID]++
	count := s.rateLimitCount[batch.ID]
	s.rateLimitBackoffUntil = s.cfg.Now().Add(DefaultRateLimitCooldown)
	s.lastBatchProcessedAt = s.cfg.Now()
	s.queueMu.Unlock()

	if count >= 5 {
		// Truly out of quota after multiple queue cooldown retries.
		s.failBatch(ctx, batch, err)
		return
	}

	// Transient TPM / RPM rate limit: reset status to pending without burning attempts
	_ = s.cfg.Store.SetBatchStatus(ctx, batch.ID, storage.BatchPending, "", "", s.cfg.Now())
}

func (s *Service) recordBatchSuccess(batchID int64) {
	s.queueMu.Lock()
	delete(s.rateLimitCount, batchID)
	s.lastBatchProcessedAt = s.cfg.Now()
	s.queueMu.Unlock()
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

	s.queueMu.Lock()
	backoff := s.rateLimitBackoffUntil
	s.queueMu.Unlock()
	if !backoff.IsZero() && backoff.After(now) {
		// Rate limit cooldown active: wait for token bucket to replenish before resuming queue
		return
	}

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
			if err := s.paceBatch(ctx); err != nil {
				return
			}
			if err := s.processBatch(ctx, batch); err != nil {
				if ctx.Err() != nil {
					// Canceled mid-flight: the batch stays processing for the
					// next startup to adopt. Not a failure.
					return
				}
				if errors.Is(err, errBatchDeferred) {
					// Still in the active segment: stays pending for a later tick.
					continue
				}
				if isRateLimitError(err) {
					s.handleBatchRateLimit(ctx, batch, err)
					return
				}
				s.queueMu.Lock()
				s.lastBatchProcessedAt = s.cfg.Now()
				s.queueMu.Unlock()
				s.failBatch(ctx, batch, err)
			} else {
				s.recordBatchSuccess(batch.ID)
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
		if err := s.paceBatch(ctx); err != nil {
			return
		}
		if err := s.processBatch(ctx, batch); err != nil {
			if ctx.Err() != nil {
				return
			}
			if errors.Is(err, errBatchDeferred) {
				// Still in the active segment: stays pending, drained next tick.
				continue
			}
			if isRateLimitError(err) {
				s.handleBatchRateLimit(ctx, batch, err)
				return
			}
			s.queueMu.Lock()
			s.lastBatchProcessedAt = s.cfg.Now()
			s.queueMu.Unlock()
			s.failBatch(ctx, batch, err)
		} else {
			s.recordBatchSuccess(batch.ID)
		}
	}
	// The latest run stays unbatched until its span reaches the target
	// (off-by-one-interval rule); a run that already reached it qualifies.
	if split.Latest.Span() >= TargetBatchDuration {
		batch, err := s.cfg.Store.CreateBatch(ctx, split.Latest.Frames, storage.BatchPending, now)
		if err != nil {
			return
		}
		if err := s.paceBatch(ctx); err != nil {
			return
		}
		if err := s.processBatch(ctx, batch); err != nil {
			if ctx.Err() != nil {
				return
			}
			if errors.Is(err, errBatchDeferred) {
				// Still in the active segment: stays pending, drained next tick.
				return
			}
			if isRateLimitError(err) {
				s.handleBatchRateLimit(ctx, batch, err)
				return
			}
			s.queueMu.Lock()
			s.lastBatchProcessedAt = s.cfg.Now()
			s.queueMu.Unlock()
			s.failBatch(ctx, batch, err)
		} else {
			s.recordBatchSuccess(batch.ID)
		}
	}
}

// errBatchDeferred signals processBatch declined to run a batch that still
// references the active recording segment. It is not a failure: the batch stays
// pending, untouched, for a later tick to retry once the segment finalizes. The
// scheduler must not fail the batch or count an attempt against it.
var errBatchDeferred = errors.New("analysis: batch references the active recording segment")

// batchInActiveSegment reports whether any frame in the batch still belongs to
// the segment the recorder is actively writing. Segments finalize in order and
// only one is ever active, so this is the tail of a freshly sealed batch — an
// unfinalized container (no moov atom on macOS) that cannot be decoded yet.
// A finalized segment never matches, so an old missing or corrupt segment still
// fails visibly through the normal path rather than deferring forever.
func (s *Service) batchInActiveSegment(frames []storage.AnalysisFrame) bool {
	if s.cfg.ActiveSegment == nil || len(frames) == 0 {
		return false
	}
	active := s.cfg.ActiveSegment()
	if active == "" {
		return false
	}
	for _, f := range frames {
		if f.SegmentPath == active {
			return true
		}
	}
	return false
}

// processBatch runs one batch through the pipeline. A returned error fails
// the batch; context cancellation returns ctx.Err() with the batch left in
// processing for the next startup; errBatchDeferred leaves the batch pending.
func (s *Service) processBatch(ctx context.Context, batch storage.Batch) error {
	frames, err := s.cfg.Store.FramesForBatch(ctx, batch.ID)
	if err != nil {
		return err
	}
	// Frames still in the active recording segment cannot be decoded yet: the
	// container has no moov atom until the segment rolls over or is finalized on
	// pause/stop. Leave the batch pending (no processing status, no attempt
	// counted) so a later tick retries once it finalizes, instead of failing the
	// batch on a frameDecode error and waiting out the retry cooldown.
	if s.batchInActiveSegment(frames) {
		return errBatchDeferred
	}
	if err := s.cfg.Store.SetBatchStatus(ctx, batch.ID, storage.BatchProcessing, "", "", s.cfg.Now()); err != nil {
		return err
	}
	if len(frames) == 0 {
		return fmt.Errorf("batch %d has no frames", batch.ID)
	}

	// Idle fast path: no LLM call at all (docs/04 §4.4). It rewrites cards in
	// a range like the LLM path, so it takes the same serialization lock —
	// an idle commit interleaving a neighboring batch's read→generate→rewrite
	// would clobber one of the two.
	if DetectIdle(frames, s.cfg.IdleRules) {
		s.cardsMu.Lock()
		defer s.cardsMu.Unlock()
		return s.commitIdleCard(ctx, batch)
	}

	chain, err := s.cfg.Providers.AnalysisChain(ctx)
	if err != nil {
		return err
	}
	// All provider calls in this batch, including the text-only card stage,
	// carry the batch id so attempt diagnostics can identify the failing stage.
	batchID := batch.ID
	ctx = ai.WithAttemptMetadata(ctx, ai.AttemptMetadata{BatchID: &batchID})

	// Transcription: consecutive groups of at most ai.MaxImages frames.
	observations, err := s.transcribe(ctx, chain, frames)
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

	// Generation, validation, and storage must own the exact same left edge.
	// A card crossing the batch start is especially important on reprocess: if
	// storage deletes it as an overlap while generation starts at the batch
	// boundary, the card's earlier prefix silently disappears.
	rewriteStart, ongoing := cardRewriteStart(existing, batch.Start)

	shells, ownedFrom, err := s.generateCards(ctx, chain, batch, existing, observations, categories, rewriteStart, ongoing)
	if err != nil {
		return err
	}
	// The rewrite replaces everything from the span's start (the merged
	// predecessor's start in ongoing mode) through the window end; cards
	// before the span survive beside the rewrite. ownedFrom is that start after
	// the merge gate: a merge the gate refused starts the span at the window
	// instead, and the cross-category predecessor keeps its own card.
	result, err := s.cfg.Cards.ReplaceCardsInRange(ctx, ownedFrom, batch.End, shells, batch.ID)
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

// cardRewriteStart returns the connected span owned by the next card pass. A
// straddling card wins because replacing only its overlap would delete its
// prefix. Otherwise only the nearest predecessor within five minutes is
// eligible for an ongoing rewrite.
func cardRewriteStart(existing []domain.TimelineCard, batchStart time.Time) (time.Time, bool) {
	startUnix := batchStart.Unix()
	var straddling *domain.TimelineCard
	for i := range existing {
		card := &existing[i]
		if card.StartTs <= startUnix && card.EndTs > startUnix &&
			(straddling == nil || card.StartTs < straddling.StartTs) {
			straddling = card
		}
	}
	if straddling != nil {
		return time.Unix(straddling.StartTs, 0), true
	}
	for i := len(existing) - 1; i >= 0; i-- {
		card := existing[i]
		if card.EndTs <= startUnix {
			if startUnix-card.EndTs <= 300 {
				return time.Unix(card.StartTs, 0), true
			}
			break
		}
	}
	return batchStart, false
}

// commitIdleCard writes the Idle card directly, merging with a directly
// preceding Idle card when close enough (docs/04 §4.4).
func (s *Service) commitIdleCard(ctx context.Context, batch storage.Batch) error {
	replaceFrom := batch.Start
	shell := domain.CardShell{
		Start:    timeutil.FormatClock(batch.Start, s.loc()),
		End:      timeutil.FormatClock(batch.End, s.loc()),
		Category: "Idle",
		Title:    "Idle",
		Summary:  "No user activity detected during this period.",
	}
	// Merge with a preceding Idle card within AdjacentIdleMergeGap. A read
	// failure propagates: silently opening a new card next to a mergeable
	// one would split the idle span on a transient storage error.
	preceding, err := s.cfg.Cards.CardsInRange(ctx,
		batch.Start.Add(-s.cfg.IdleRules.AdjacentIdleMergeGap-time.Minute), batch.Start)
	if err != nil {
		return err
	}
	for _, card := range slices.Backward(preceding) {
		if card.Category == "Idle" && card.EndTs <= batch.Start.Unix() &&
			batch.Start.Unix()-card.EndTs <= int64(s.cfg.IdleRules.AdjacentIdleMergeGap.Seconds()) {
			replaceFrom = time.Unix(card.StartTs, 0)
			shell.Start = card.Start
			break
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
// parallelism. The context already carries the batch id from processBatch,
// which also covers the later card-generation call.
func (s *Service) transcribe(ctx context.Context, chain *ai.Chain,
	frames []storage.AnalysisFrame) ([]storage.Observation, error) {

	// Downsample frames to at most DefaultSampledFrames (15 frames evenly spaced
	// across the batch, matching Dayflow) or the chain's image cap if lower.
	// This bounds token consumption per batch to ~20k tokens instead of >130k.
	targetSamples := DefaultSampledFrames
	if cap := s.cfg.Providers.ImageCap(ctx); cap > 0 && cap < targetSamples {
		targetSamples = cap
	}
	sampled := sampleFrames(frames, targetSamples)

	// The image cap comes from the chain source, not the request: grouping
	// must size every group so ANY chain entry can serve it, since fallback
	// may hand a group to a provider with a lower gateway limit mid-flight.
	groups := groupFrames(sampled, targetSamples)
	type outcome struct {
		obs []storage.Observation
		err error
	}
	results := make([]outcome, len(groups))
	sem := make(chan struct{}, s.cfg.Workers)
	var wg sync.WaitGroup

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
			// No segment paths in the message: it lands verbatim in
			// failure_note and the batch:failed event.
			return nil, fmt.Errorf("read frame %d of batch: %w", f.FrameIndex, err)
		}
		part, err := ai.ImagePart(ai.MediaJPEG, data)
		if err != nil {
			return nil, fmt.Errorf("prepare frame %d for transcription: %w", f.FrameIndex, err)
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

// appSitesMetadata is the appSites wire shape the binding layer reads out of
// card metadata (docs/05 §5.5.2). The model returns a flat list, so the
// pipeline — not the consumer — owns the mapping onto primary/secondary.
type appSitesMetadata struct {
	Primary   *string `json:"primary"`
	Secondary *string `json:"secondary"`
}

// distractionMetadata is the distraction wire shape the binding layer reads out
// of card metadata (docs/05 §5.5.2). The model names the range start/end, like
// a card window; metadata names it startTime/endTime, like the DTO. The
// pipeline owns that rename for the same reason it owns appSites: a consumer
// that knows the model's field names is a consumer that breaks when they move.
type distractionMetadata struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
}

// distractionsFromModel maps the model's distractions onto the metadata
// contract. Untitled entries are dropped: an interruption the model could not
// name would render as a bare clock range. The result is never nil, so an empty
// list stores as [] and consumers doing .length stay safe.
func distractionsFromModel(values []cardsDistraction) []distractionMetadata {
	out := make([]distractionMetadata, 0, len(values))
	for _, value := range values {
		title := strings.TrimSpace(value.Title)
		if title == "" {
			continue
		}
		out = append(out, distractionMetadata{
			StartTime: strings.TrimSpace(value.Start),
			EndTime:   strings.TrimSpace(value.End),
			Title:     title,
			Summary:   strings.TrimSpace(value.Summary),
		})
	}
	return out
}

func isBrowserName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "edge", "microsoft edge", "chrome", "google chrome", "safari", "firefox", "arc", "brave", "opera", "vivaldi":
		return true
	}
	return false
}

// appSitesFromList maps the model's flat app/site list onto the contract
// shape. Blank entries and anything past the second are dropped; an empty list
// stores null rather than an empty object.
func appSitesFromList(values []string) *appSitesMetadata {
	var kept []string
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			kept = append(kept, value)
			if len(kept) == 2 {
				break
			}
		}
	}
	if len(kept) == 0 {
		return nil
	}
	// If the model provided [Browser, Website], swap them so the website is Primary
	// and the enclosing browser is Secondary.
	if len(kept) == 2 && isBrowserName(kept[0]) && !isBrowserName(kept[1]) {
		kept[0], kept[1] = kept[1], kept[0]
	}
	sites := &appSitesMetadata{Primary: &kept[0]}
	if len(kept) == 2 {
		sites.Secondary = &kept[1]
	}
	return sites
}

// shellsFromModel maps the model's cards onto storable shells: a category
// outside the known list falls back to System (counted, never auto-created),
// activity points without a description are dropped, and the metadata is
// rebuilt in the stored shape. Both the batch pipeline and the single-card
// rewrite map through here, so a card means the same thing whichever path
// produced it.
func shellsFromModel(cards []modelCard, known map[string]bool) []domain.CardShell {
	shells := make([]domain.CardShell, 0, len(cards))
	for _, c := range cards {
		category := c.Category
		if !known[category] {
			category = "System"
		}
		points := make([]cardActivityPoint, 0, len(c.ActivityPoints))
		for _, p := range c.ActivityPoints {
			if p.Description == "" {
				continue
			}
			points = append(points, cardActivityPoint{Time: p.Time, Description: p.Description})
		}
		metadata, _ := json.Marshal(map[string]any{
			"appSites":       appSitesFromList(c.AppSites),
			"distractions":   distractionsFromModel(c.Distractions),
			"activityPoints": points,
		})
		shells = append(shells, domain.CardShell{
			Start:           c.Start,
			End:             c.End,
			Category:        category,
			Subcategory:     c.Subcategory,
			Title:           c.Title,
			Summary:         boundSummary(c.Summary),
			DetailedSummary: boundDetailedSummary(c.DetailedSummary),
			Metadata:        string(metadata),
		})
	}
	return shells
}

// generateCards runs the card stage: prompt with sliding-window context,
// parse, then validate every category against the known list — an unknown
// category maps to System and is counted, never auto-created (docs/04 §4.3.4).
// generateCards runs the card stage in Dayflow's shape: a mode-dependent
// prompt, then up to three attempts where a failed validation sends the
// previous JSON back with structured issues (the correction pass). Returns
// the accepted shells and the rewrite span start the caller must replace from.
func (s *Service) generateCards(ctx context.Context, chain *ai.Chain, batch storage.Batch,
	existing []domain.TimelineCard, obs []storage.Observation,
	categories []domain.Category, rewriteStart time.Time, ongoing bool) ([]domain.CardShell, time.Time, error) {

	// Built-in names are not valid model output: a model echoing "Idle" from
	// nearby-card context must fall into the System fallback, never land as
	// Idle — that category belongs to the hardware idle fast path alone.
	known := make(map[string]bool, len(categories))
	for _, c := range categories {
		if c.IsSystem {
			continue
		}
		known[c.Name] = true
	}

	requiresSingleCard := !ongoing
	mode := cardModeFresh
	if ongoing {
		mode = cardModeOngoing
	}
	ownedFrom := rewriteStart
	var lastRaw json.RawMessage
	var issues []string
	for attempt := 1; attempt <= 3; attempt++ {
		var request ai.Request
		if attempt == 1 {
			request = ai.Request{
				Purpose:         ai.PurposeCards,
				Parts:           []ai.Part{ai.TextPart(cardsPrompt(batch.Start, batch.End, existing, obs, categories, s.cfg.Language(ctx), mode))},
				Output:          &cardsOutput,
				MaxOutputTokens: 4096,
			}
		} else {
			request = ai.Request{
				Purpose:         ai.PurposeCards,
				Parts:           []ai.Part{ai.TextPart(cardsCorrectionPrompt(string(lastRaw), issues, mode, ownedFrom, batch.End))},
				Output:          &cardsOutput,
				MaxOutputTokens: 4096,
			}
		}
		result, err := chain.Generate(ctx, request)
		if err != nil {
			return nil, rewriteStart, err
		}
		raw, err := ai.ParseStructuredOutput(result.Text, cardsOutput)
		if err != nil {
			return nil, rewriteStart, err
		}
		lastRaw = raw
		var envelope cardsEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, rewriteStart, fmt.Errorf("decode cards: %w", err)
		}

		var shells []domain.CardShell
		var rejectedIssues []string
		for cardIndex, shell := range shellsFromModel(envelope.Cards, known) {
			// A shell the model resolved entirely outside the rewrite span is
			// a context-merge hallucination; drop it rather than let the
			// rewrite duplicate it outside the range we own.
			if issue := s.shellSpanIssue(shell, rewriteStart, batch.End); issue == "" {
				shells = append(shells, shell)
			} else {
				rejectedIssues = append(rejectedIssues, fmt.Sprintf("card %d (%s) %s", cardIndex+1, shell.Title, issue))
			}
		}

		// The gate owns the left edge from here on: a shell that claimed time
		// before the batch window keeps only the part the gate allowed, and
		// validation checks the same start the rewrite will replace from.
		ownedFrom = s.mergeOwnershipStart(shells, batch, existing)
		for i := range shells {
			shells[i] = s.clampToMergeFloor(shells[i], ownedFrom, batch)
			s.inheritMergedAppSites(&shells[i], ownedFrom, batch, existing)
		}

		spans := resolveCardSpans(shells, ownedFrom, batch.End, s.loc())
		issues = validateCards(spans, ownedFrom, batch.End, requiresSingleCard)
		// If the provider returned cards but every one was rejected before
		// validation, "no cards returned" is false and unactionable. Preserve
		// the rejected clocks and required span for the correction pass.
		if len(shells) == 0 && len(rejectedIssues) > 0 {
			issues = rejectedIssues
		}
		if len(issues) == 0 {
			return shells, ownedFrom, nil
		}
		// Activity points from before the owned span are dropped so a refused
		// merge's points never duplicate inside the rewrite.
		for i := range shells {
			shells[i].Metadata = dropPreWindowPoints(shells[i].Metadata, ownedFrom, batch.Start.Add(batch.End.Sub(batch.Start)/2), s.loc())
		}
	}
	return nil, ownedFrom, fmt.Errorf("cards failed validation after 3 attempts: %s", strings.Join(issues, "; "))
}

// Failures of the single-card rewrite, kept distinguishable so the binding
// layer can map them to user-facing error codes without matching on message
// text (docs/05 §5.6.2).
var (
	// ErrNoSourceBatch: the card carries no batch provenance, so a rewrite has
	// nothing to attribute its rows to.
	ErrNoSourceBatch = errors.New("analysis: card has no source batch")
	// ErrWindowInAnalysis: a batch over the same window is pending or
	// processing and would overwrite this regeneration when it lands.
	ErrWindowInAnalysis = errors.New("analysis: card window is still being analyzed")
	// ErrNoCardEvidence: the window has no stored observations to rewrite from.
	ErrNoCardEvidence = errors.New("analysis: card window has no stored observations")
	// ErrCardsInvalid: the model's cards never covered the window within the
	// correction attempts. Nothing was written.
	ErrCardsInvalid = errors.New("analysis: card output failed validation")
)

/*
 * RegenerateCard rewrites exactly one card's own span from the evidence already
 * stored for those minutes. It is deliberately not the batch pipeline: every
 * card a batch writes carries that batch's id, so requeueing the batch
 * regenerates the card's siblings too — the two 60-minute cards a long activity
 * was split into share one batch — and the sliding window's merge rule can
 * extend the rewrite back over the preceding card. Here the window is the
 * card's own stored span, both outer boundaries are fixed, and the neighbours
 * on either side are context rather than output.
 *
 * The evidence is the observations already stored for the window, not a fresh
 * transcription: the point is a second take on the same screenshots, and the
 * batch's own transcription is what the model already saw. A rewrite that names
 * no app inherits the replaced card's appSites, so regenerating never costs the
 * card its icon.
 */
func (s *Service) RegenerateCard(ctx context.Context, card domain.TimelineCard) error {
	if card.BatchID == nil {
		return ErrNoSourceBatch
	}
	windowStart, windowEnd, err := s.cardWindow(card)
	if err != nil {
		return err
	}
	// A live batch over the same window will rewrite these cards when it lands,
	// which would silently discard this regeneration's result.
	live, err := s.cfg.Store.ProcessingBatchesInRange(ctx, windowStart, windowEnd)
	if err != nil {
		return err
	}
	if len(live) > 0 {
		return fmt.Errorf("%w: card window %s-%s is being analyzed by batch %d",
			ErrWindowInAnalysis, formatFrameClock(windowStart), formatFrameClock(windowEnd), live[0].ID)
	}

	// The same lock the batch pipeline holds across read→generate→rewrite: a
	// regeneration interleaving a neighbouring batch's rewrite of the same
	// minutes would clobber one of the two (docs/04 §4.3.2).
	s.cardsMu.Lock()
	defer s.cardsMu.Unlock()

	observations, err := s.cfg.Store.ObservationsInRange(ctx, windowStart, windowEnd)
	if err != nil {
		return err
	}
	if len(observations) == 0 {
		return fmt.Errorf("%w: card window %s-%s has no stored observations",
			ErrNoCardEvidence, formatFrameClock(windowStart), formatFrameClock(windowEnd))
	}
	categories, err := s.cfg.Categories.List(ctx)
	if err != nil {
		return err
	}
	// Context: the cards ending at or before this window, plus the card being
	// replaced — its apps and time points are evidence the model may keep when
	// the new take does not replace them.
	existing, err := s.cfg.Cards.CardsInRange(ctx, windowStart.Add(-CardLookback), windowStart)
	if err != nil {
		return err
	}
	existing = append(existing, card)

	chain, err := s.cfg.Providers.AnalysisChain(ctx)
	if err != nil {
		return err
	}
	shells, err := s.generateScopedCards(ctx, chain, card, windowStart, windowEnd, existing, observations, categories)
	if err != nil {
		return err
	}
	if _, err := s.cfg.Cards.ReplaceCardsInRange(ctx, windowStart, windowEnd, shells, *card.BatchID); err != nil {
		return err
	}
	s.notifyDays(windowStart, windowEnd)
	return nil
}

// cardWindow resolves a card's stored clock strings back to the instants the
// rewrite owns. The stored strings — not a reformat of start_ts/end_ts — are
// the authority: ReplaceCardsInRange resolves the same strings through the same
// function, so "the window" means one range in both directions.
func (s *Service) cardWindow(card domain.TimelineCard) (time.Time, time.Time, error) {
	loc := s.loc()
	anchor := time.Unix((card.StartTs+card.EndTs)/2, 0)
	start, err := timeutil.ResolveClock(card.Start, anchor, loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("card %d start %q: %w", card.ID, card.Start, err)
	}
	end, err := timeutil.ResolveClock(card.End, anchor, loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("card %d end %q: %w", card.ID, card.End, err)
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("card %d has an empty window %s-%s", card.ID, card.Start, card.End)
	}
	return start, end, nil
}

// generateScopedCards runs the card stage for one fixed window: the mode that
// neither extends over the preceding card nor demands a single card, with the
// same three-attempt correction loop as the batch pipeline.
func (s *Service) generateScopedCards(ctx context.Context, chain *ai.Chain, card domain.TimelineCard,
	windowStart, windowEnd time.Time, existing []domain.TimelineCard,
	obs []storage.Observation, categories []domain.Category) ([]domain.CardShell, error) {

	known := make(map[string]bool, len(categories))
	for _, c := range categories {
		if c.IsSystem {
			continue
		}
		known[c.Name] = true
	}

	var lastRaw json.RawMessage
	var issues []string
	for attempt := 1; attempt <= 3; attempt++ {
		var request ai.Request
		if attempt == 1 {
			request = ai.Request{
				Purpose:         ai.PurposeCards,
				Parts:           []ai.Part{ai.TextPart(cardsPrompt(windowStart, windowEnd, existing, obs, categories, s.cfg.Language(ctx), cardModeScoped))},
				Output:          &cardsOutput,
				MaxOutputTokens: 4096,
			}
		} else {
			request = ai.Request{
				Purpose:         ai.PurposeCards,
				Parts:           []ai.Part{ai.TextPart(cardsCorrectionPrompt(string(lastRaw), issues, cardModeScoped, windowStart, windowEnd))},
				Output:          &cardsOutput,
				MaxOutputTokens: 4096,
			}
		}
		result, err := chain.Generate(ctx, request)
		if err != nil {
			return nil, err
		}
		raw, err := ai.ParseStructuredOutput(result.Text, cardsOutput)
		if err != nil {
			return nil, err
		}
		lastRaw = raw
		var envelope cardsEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, fmt.Errorf("decode cards: %w", err)
		}

		var shells []domain.CardShell
		var rejectedIssues []string
		for cardIndex, shell := range shellsFromModel(envelope.Cards, known) {
			if issue := s.shellSpanIssue(shell, windowStart, windowEnd); issue == "" {
				shells = append(shells, shell)
			} else {
				rejectedIssues = append(rejectedIssues, fmt.Sprintf("card %d (%s) %s", cardIndex+1, shell.Title, issue))
			}
		}

		spans := resolveCardSpans(shells, windowStart, windowEnd, s.loc())
		issues = validateScopedCards(spans, windowStart, windowEnd)
		if len(shells) == 0 && len(rejectedIssues) > 0 {
			issues = rejectedIssues
		}
		if len(issues) == 0 {
			pinScopedBoundaries(shells, card)
			s.inheritScopedAppSites(shells, card, windowStart, windowEnd)
			return shells, nil
		}
	}
	return nil, fmt.Errorf("%w: card %d failed validation after 3 attempts: %s",
		ErrCardsInvalid, card.ID, strings.Join(issues, "; "))
}

// pinScopedBoundaries snaps the accepted cards onto the window the rewrite
// owns. Validation tolerates a minute of clock rounding; storage does not — it
// expands the deletion range to whatever the cards resolve to, so a card ending
// a minute past the window would make the rewrite overlap the neighbour it must
// not touch (and the whole transaction would roll back). Writing the card's own
// stored strings back makes the written range identical to the range read.
func pinScopedBoundaries(shells []domain.CardShell, card domain.TimelineCard) {
	if len(shells) == 0 {
		return
	}
	shells[0].Start = card.Start
	shells[len(shells)-1].End = card.End
}

// inheritScopedAppSites gives the replaced card's appSites to the output card
// that covers most of the window when the model named no app at all. The icon
// came from these same minutes, so a regeneration that drops it would be a
// downgrade; a model that named an app keeps its choice.
func (s *Service) inheritScopedAppSites(shells []domain.CardShell, card domain.TimelineCard,
	windowStart, windowEnd time.Time) {

	inherited := appSitesOfMetadata(card.Metadata)
	if inherited == nil || inherited.Primary == nil || *inherited.Primary == "" {
		return
	}
	longest, longestSpan := -1, time.Duration(0)
	loc := s.loc()
	anchor := windowStart.Add(windowEnd.Sub(windowStart) / 2)
	for i, shell := range shells {
		start, err := timeutil.ResolveClock(shell.Start, anchor, loc)
		if err != nil {
			continue
		}
		end, err := timeutil.ResolveClock(shell.End, anchor, loc)
		if err != nil || !end.After(start) {
			continue
		}
		if span := end.Sub(start); span > longestSpan {
			longest, longestSpan = i, span
		}
	}
	if longest < 0 {
		return
	}

	var meta struct {
		AppSites       *appSitesMetadata     `json:"appSites"`
		Distractions   []distractionMetadata `json:"distractions"`
		ActivityPoints []cardActivityPoint   `json:"activityPoints"`
	}
	if err := json.Unmarshal([]byte(shells[longest].Metadata), &meta); err != nil {
		return
	}
	if meta.AppSites != nil && meta.AppSites.Primary != nil && *meta.AppSites.Primary != "" {
		return
	}
	meta.AppSites = inherited
	out, err := json.Marshal(meta)
	if err != nil {
		return
	}
	shells[longest].Metadata = string(out)
}

// shellSpanIssue explains why a model-returned shell cannot be owned by this
// rewrite. The correction loop uses it only when all returned shells were
// rejected; mixed valid/context-only output retains the established behavior
// of silently dropping the context hallucination.
func (s *Service) shellSpanIssue(shell domain.CardShell, spanStart, spanEnd time.Time) string {
	loc := s.loc()
	anchor := spanStart.Add(spanEnd.Sub(spanStart) / 2)
	start, err := timeutil.ResolveClock(shell.Start, anchor, loc)
	if err != nil {
		return fmt.Sprintf("has unparseable start time %q; use h:mm AM/PM inside %s-%s",
			shell.Start, formatFrameClock(spanStart), formatFrameClock(spanEnd))
	}
	end, err := timeutil.ResolveClock(shell.End, anchor, loc)
	if err != nil {
		return fmt.Sprintf("has unparseable end time %q; use h:mm AM/PM inside %s-%s",
			shell.End, formatFrameClock(spanStart), formatFrameClock(spanEnd))
	}
	if !start.Before(spanEnd) || !end.After(spanStart) {
		return fmt.Sprintf("spans %s-%s outside required rewrite window %s-%s; move it into that window",
			shell.Start, shell.End, formatFrameClock(spanStart), formatFrameClock(spanEnd))
	}
	return ""
}

// mergeOwnershipStart returns the earliest timestamp this batch's rewrite may
// own — the deterministic gate behind the prompt's merge rule (docs/04 §4.3.4).
//
// A shell claiming a start before the batch window is absorbing the cards it
// continues. Same-category predecessors may be absorbed; System predecessors
// are exempt, since their category is a fallback rather than a claim about the
// activity. A predecessor of any other category is not: absorbing it would put
// its minutes inside a card of a different category and corrupt the daily and
// weekly category totals, so the whole extension is refused and the rewrite
// starts at the window instead, leaving that predecessor's card in place.
//
// A card straddling the batch start is always owned, whatever its category:
// the batch's evidence overlaps it, and replacing only its overlap would delete
// its prefix (docs/03 §3.5).
func (s *Service) mergeOwnershipStart(shells []domain.CardShell, batch storage.Batch,
	existing []domain.TimelineCard) time.Time {

	straddling, found := time.Time{}, false
	for _, card := range existing {
		if card.StartTs < batch.Start.Unix() && card.EndTs > batch.Start.Unix() {
			if start := time.Unix(card.StartTs, 0); !found || start.Before(straddling) {
				straddling, found = start, true
			}
		}
	}
	if found {
		return straddling
	}

	claimed := make(map[string]bool, len(shells))
	earliest, any := earliestShellStart(shells, batch, s.loc())
	if !any {
		return batch.Start
	}
	anchor := batch.Start.Add(batch.End.Sub(batch.Start) / 2)
	for _, shell := range shells {
		if start, err := timeutil.ResolveClock(shell.Start, anchor, s.loc()); err == nil && start.Before(batch.Start) {
			claimed[shell.Category] = true
		}
	}

	// Close the absorbed set under overlap: a card pulled in by the claim can
	// itself start inside an earlier one, and the rewrite has to own every card
	// it deletes whole (docs/03 §3.5).
	floor := earliest
	for changed := true; changed; {
		changed = false
		for _, card := range existing {
			if card.StartTs >= batch.Start.Unix() || card.EndTs <= floor.Unix() {
				continue
			}
			if card.Category != "System" && !claimed[card.Category] {
				return batch.Start
			}
			if start := time.Unix(card.StartTs, 0); start.Before(floor) {
				floor, changed = start, true
			}
		}
	}
	return floor
}

// clampToMergeFloor pins a shell that claimed time the gate did not grant to
// the rewrite's owned start, dropping the points that came with the refused
// claim. Validation and storage use the same start, so a clamped card is
// accepted on the first attempt instead of being corrected into the refused
// merge again.
func (s *Service) clampToMergeFloor(shell domain.CardShell, floor time.Time,
	batch storage.Batch) domain.CardShell {

	loc := s.loc()
	anchor := batch.Start.Add(batch.End.Sub(batch.Start) / 2)
	start, err := timeutil.ResolveClock(shell.Start, anchor, loc)
	if err != nil || !start.Before(floor) {
		return shell
	}
	// Clocks carry minutes while a batch start carries the first frame's
	// seconds. Rounding the owned start down would put the card back inside the
	// predecessor the gate just refused, and the rewrite would then delete a
	// card it does not own; round up instead.
	hour, minutes, _ := floor.Clock()
	year, month, day := floor.Date()
	minute := time.Date(year, month, day, hour, minutes, 0, 0, loc)
	if minute.Before(floor) {
		minute = minute.Add(time.Minute)
	}
	shell.Start = timeutil.FormatClock(minute, loc)
	shell.Metadata = dropPreWindowPoints(shell.Metadata, floor, anchor, loc)
	return shell
}

// inheritMergedAppSites gives an absorbed predecessor's icon back to the card
// that swallowed it. The merged card's appSites come from the model's output
// for the combined span alone, so a merge that names no app leaves the card
// with no icon at all — and the predecessor that used to carry one is gone.
// Only an empty list is filled; a model that named an app keeps its choice.
func (s *Service) inheritMergedAppSites(shell *domain.CardShell, floor time.Time,
	batch storage.Batch, existing []domain.TimelineCard) {

	loc := s.loc()
	anchor := batch.Start.Add(batch.End.Sub(batch.Start) / 2)
	start, err := timeutil.ResolveClock(shell.Start, anchor, loc)
	if err != nil || !start.Before(batch.Start) {
		return
	}
	var meta struct {
		AppSites       *appSitesMetadata     `json:"appSites"`
		Distractions   []distractionMetadata `json:"distractions"`
		ActivityPoints []cardActivityPoint   `json:"activityPoints"`
	}
	if err := json.Unmarshal([]byte(shell.Metadata), &meta); err != nil {
		return
	}
	if meta.AppSites != nil && meta.AppSites.Primary != nil && *meta.AppSites.Primary != "" {
		return
	}
	inherited := s.absorbedAppSites(floor, batch, existing)
	if inherited == nil {
		return
	}
	meta.AppSites = inherited
	out, err := json.Marshal(meta)
	if err != nil {
		return
	}
	shell.Metadata = string(out)
}

// absorbedAppSites returns the appSites of the predecessor nearest the window
// among the cards this rewrite absorbs, or nil when none of them named an app.
func (s *Service) absorbedAppSites(floor time.Time, batch storage.Batch,
	existing []domain.TimelineCard) *appSitesMetadata {

	var nearest *domain.TimelineCard
	for i := range existing {
		card := &existing[i]
		if card.EndTs > batch.Start.Unix() || card.EndTs <= floor.Unix() {
			continue
		}
		if nearest == nil || card.EndTs > nearest.EndTs {
			nearest = card
		}
	}
	if nearest == nil {
		return nil
	}
	return appSitesOfMetadata(nearest.Metadata)
}

// dropPreWindowPoints removes a rejected merge's absorbed activity points:
// every point whose clock resolves before the batch window no longer belongs
// to this card. Points that do not resolve are kept — they are display
// metadata, and an unresolvable clock must not silently delete content. When
// nothing is dropped the original metadata string is returned untouched.
func dropPreWindowPoints(metadata string, windowStart time.Time, anchor time.Time, loc *time.Location) string {
	// Every field must round-trip: the rewrite below re-marshals this struct
	// over the original metadata, so a field the struct cannot decode is a
	// field the rewrite silently drops.
	var meta struct {
		AppSites       *appSitesMetadata     `json:"appSites"`
		Distractions   []distractionMetadata `json:"distractions"`
		ActivityPoints []cardActivityPoint   `json:"activityPoints"`
	}
	if err := json.Unmarshal([]byte(metadata), &meta); err != nil {
		return metadata
	}
	kept := meta.ActivityPoints[:0]
	dropped := false
	for _, p := range meta.ActivityPoints {
		t, err := timeutil.ResolveClock(p.Time, anchor, loc)
		if err == nil && t.Before(windowStart) {
			dropped = true
			continue
		}
		kept = append(kept, p)
	}
	if !dropped {
		return metadata
	}
	meta.ActivityPoints = kept
	out, err := json.Marshal(meta)
	if err != nil {
		return metadata
	}
	return string(out)
}

// Limits for the model-facing summary rules, enforced as a deterministic
// backstop so a model that ignores the prompt cannot grow a merged card's
// log without bound. Rune counts match the docs' 字符, not bytes.
const (
	detailedSummaryMaxParagraphs = 15
	detailedSummaryMaxRunes      = 2500
	summaryMaxRunes              = 135
)

// boundSummary caps the one-sentence summary at summaryMaxRunes, cut at a
// word boundary. Shorter input passes through untouched.
func boundSummary(text string) string {
	runes := []rune(text)
	if len(runes) <= summaryMaxRunes {
		return text
	}
	bounded := string(runes[:summaryMaxRunes])
	if cut := strings.LastIndexAny(bounded, " ，,；;"); cut > 0 {
		bounded = bounded[:cut]
	}
	return bounded
}

// boundDetailedSummary caps a generated detailed summary: at most 15
// paragraphs and 2500 runes, cut at paragraph boundaries from the end
// (recent detail matters more than old detail). Single-paragraph overflow is
// hard-truncated at a word boundary. Empty input passes through untouched.
func boundDetailedSummary(text string) string {
	if text == "" {
		return text
	}
	paragraphs := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(paragraphs) > detailedSummaryMaxParagraphs {
		keep := paragraphs[len(paragraphs)-detailedSummaryMaxParagraphs:]
		paragraphs = append([]string(nil), keep...)
	}
	total := 0
	for i, p := range paragraphs {
		n := utf8.RuneCountInString(p)
		if total+n <= detailedSummaryMaxRunes {
			total += n + 1
			continue
		}
		remaining := detailedSummaryMaxRunes - total
		if remaining > 0 {
			runes := []rune(p)
			prefix := string(runes[:remaining])
			cut := strings.LastIndexAny(prefix, " ，,；;")
			if cut <= 0 {
				cut = remaining
			}
			paragraphs[i] = string(runes[:cut])
			paragraphs = paragraphs[:i+1]
			break
		}
		paragraphs = paragraphs[:i]
		break
	}
	return strings.Join(paragraphs, "\n")
}

// earliestShellStart resolves every shell's start clock and returns the
// earliest one that lies before the batch window — the point a merge extended
// the rewrite to. Shells that do not resolve are ignored here; they become
// SkippedCards inside the rewrite and fail the batch loudly.
func earliestShellStart(shells []domain.CardShell, batch storage.Batch, loc *time.Location) (time.Time, bool) {
	anchor := batch.Start.Add(batch.End.Sub(batch.Start) / 2)
	var earliest time.Time
	found := false
	for _, shell := range shells {
		start, err := timeutil.ResolveClock(shell.Start, anchor, loc)
		if err != nil {
			continue
		}
		if start.Before(batch.Start) && (!found || start.Before(earliest)) {
			earliest = start
			found = true
		}
	}
	return earliest, found
}

// groupFrames slices time-ordered frames into consecutive groups of at most
// maxImages frames with a total-size safety budget below ai.MaxTotalBytes.
func groupFrames(frames []storage.AnalysisFrame, maxImages int) [][]storage.AnalysisFrame {
	if len(frames) == 0 {
		return nil
	}
	maxImages = ai.ClampMaxImages(maxImages)
	const byteBudget = 18 << 20 // headroom below ai.MaxTotalBytes for prompt text
	var groups [][]storage.AnalysisFrame
	current := []storage.AnalysisFrame{frames[0]}
	total := frames[0].FileSize
	for _, f := range frames[1:] {
		if len(current) >= maxImages || (f.FileSize > 0 && total+f.FileSize > byteBudget) {
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

// evenlySpacedIndices returns up to maxCount indices evenly spaced across
// itemCount items, anchored to 0 and itemCount-1. Matches Dayflow's
// ClaudeTranscriptionInputBuilder.evenlySpacedIndices reference.
func evenlySpacedIndices(itemCount, maxCount int) []int {
	if itemCount <= 0 || maxCount <= 0 {
		return nil
	}
	if itemCount <= maxCount || maxCount == 1 {
		count := itemCount
		if count > maxCount {
			count = maxCount
		}
		indices := make([]int, count)
		for i := range indices {
			indices[i] = i
		}
		return indices
	}

	indices := make([]int, maxCount)
	for i := 0; i < maxCount; i++ {
		pos := float64(i) * float64(itemCount-1) / float64(maxCount-1)
		indices[i] = int(pos + 0.5)
	}
	return indices
}

// sampleFrames selects up to maxCount frames evenly distributed across frames,
// keeping the earliest and latest frames to preserve the window boundaries.
func sampleFrames(frames []storage.AnalysisFrame, maxCount int) []storage.AnalysisFrame {
	indices := evenlySpacedIndices(len(frames), maxCount)
	if len(indices) == len(frames) {
		return frames
	}
	sampled := make([]storage.AnalysisFrame, len(indices))
	for i, idx := range indices {
		sampled[i] = frames[idx]
	}
	return sampled
}

// isRateLimitError checks whether an error is caused by rate limiting (HTTP 429
// or per-minute token quota exhaustion).
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	if failureKind(err) == "rate_limited" {
		return true
	}
	var aiErr *ai.Error
	if errors.As(err, &aiErr) {
		if aiErr.Kind == ai.ErrorRateLimited || aiErr.HTTPStatus == 429 {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "rate_limit") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "quota") ||
		strings.Contains(msg, "resource_exhausted") ||
		strings.Contains(msg, "tokens per minute") ||
		strings.Contains(msg, "tpm")
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
	// SetBatchStatus just incremented attempts in the store; mirror it on the
	// snapshot the callback sees, or the final failure's Retryable flag is
	// computed one attempt short and promises an automatic retry that
	// RequeueFailed will refuse to make.
	batch.Attempts++
	if s.cfg.OnBatchFailed != nil {
		s.cfg.OnBatchFailed(batch, kind, note)
	}
}

// failureKind maps an error to the user-facing failure classification of
// docs/04 §4.3.3. Only genuine ai.Error values may map to provider-facing
// kinds: ai.ErrorKindOf defaults every unknown error to ErrorUnavailable, so
// switching on it directly would render a missing segment file or a storage
// failure as "network, will retry". Notes carry only already-sanitized fixed
// strings.
func failureKind(err error) string {
	if errors.Is(err, ai.ErrNoProvider) {
		return "no_provider"
	}
	var aiErr *ai.Error
	if errors.As(err, &aiErr) {
		switch aiErr.Kind {
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
		case ai.ErrorCanceled:
			return "canceled"
		}
		return "internal"
	}
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
	loc := s.loc()
	days := []string{timeutil.LogicalDay(from, loc)}
	if end := timeutil.LogicalDay(to, loc); end != days[0] {
		days = append(days, end)
	}
	s.cfg.OnCardsCommitted(days)
}

// truncate cuts s to at most n bytes without splitting a multi-byte rune:
// a mid-rune cut would corrupt failure notes in every non-ASCII language.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := s[:n]
	// Back off at most 3 bytes (the longest rune encoding minus one) until
	// the cut point sits on a rune boundary. DecodeLastRuneInString reports
	// RuneError with size 1 for an incomplete trailing sequence.
	for i := 0; i < 3 && len(cut) > 0; i++ {
		r, size := utf8.DecodeLastRuneInString(cut)
		if r != utf8.RuneError || size > 1 {
			break
		}
		cut = cut[:len(cut)-1]
	}
	return cut
}
