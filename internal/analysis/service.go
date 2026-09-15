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
	Now        func() time.Time
	TickEvery  time.Duration
	Workers    int
	IdleRules  IdleRules
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
	if cfg.Location == nil {
		cfg.Location = time.Local
	}
	return &Service{cfg: cfg}, nil
}

// loc is the single zone accessor for the service.
func (s *Service) loc() *time.Location { return s.cfg.Location }

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

	shells, err := s.generateCards(ctx, chain, batch, existing, observations, categories)
	if err != nil {
		return err
	}
	// A merged card starts at the nearby card's start, before this batch's
	// window. The rewrite range must cover that start or the merged-into card
	// survives next to its replacement — two cards where the model emitted
	// one. Extend from to the earliest resolved shell start (the idle path
	// does the same with its preceding-card merge).
	replaceFrom, ok := earliestShellStart(shells, batch, s.loc())
	if !ok {
		replaceFrom = batch.Start
	}
	result, err := s.cfg.Cards.ReplaceCardsInRange(ctx, replaceFrom, batch.End, shells, batch.ID)
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

	// The image cap comes from the chain source, not the request: grouping
	// must size every group so ANY chain entry can serve it, since fallback
	// may hand a group to a provider with a lower gateway limit mid-flight.
	groups := groupFrames(frames, s.cfg.Providers.ImageCap(ctx))
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

	var shells []domain.CardShell
	for _, c := range envelope.Cards {
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
			"appSites":       c.AppSites,
			"distractions":   c.Distractions,
			"activityPoints": points,
		})
		shell := domain.CardShell{
			Start:           c.Start,
			End:             c.End,
			Category:        category,
			Subcategory:     c.Subcategory,
			Title:           c.Title,
			Summary:         boundSummary(c.Summary),
			DetailedSummary: boundDetailedSummary(c.DetailedSummary),
			Metadata:        string(metadata),
		}
		// A card the model resolved entirely outside the batch window is a
		// context-merge hallucination; drop it rather than let the rewrite
		// duplicate it outside the range we own.
		if s.shellOverlapsWindow(shell, batch) {
			s.enforceMergeGate(&shell, batch, existing)
			shells = append(shells, shell)
		}
	}
	return shells, nil
}

// enforceMergeGate is the deterministic backstop behind the prompt's merge
// rule. A shell whose resolved start precedes the batch window declares a
// merge into earlier cards; that merge is honored only when every predecessor
// card it would absorb shares the shell's category. System predecessors are
// exempt (their category is unknown by construction), and with no absorbable
// mismatching predecessor the claim is left as the model made it. On
// rejection the shell start is clamped to the window start and activityPoints
// from before the window are dropped, so the predecessor survives beside a
// card that covers only this batch.
func (s *Service) enforceMergeGate(shell *domain.CardShell, batch storage.Batch,
	existing []domain.TimelineCard) {

	loc := s.loc()
	anchor := batch.Start.Add(batch.End.Sub(batch.Start) / 2)
	start, err := timeutil.ResolveClock(shell.Start, anchor, loc)
	if err != nil || !start.Before(batch.Start) {
		return
	}
	for _, card := range existing {
		if card.Category == "System" || card.Category == shell.Category {
			continue
		}
		if card.StartTs < batch.Start.Unix() && card.EndTs > start.Unix() {
			shell.Start = timeutil.FormatClock(batch.Start, loc)
			shell.Metadata = dropPreWindowPoints(shell.Metadata, batch.Start, anchor, loc)
			return
		}
	}
}

// dropPreWindowPoints removes a rejected merge's absorbed activity points:
// every point whose clock resolves before the batch window no longer belongs
// to this card. Points that do not resolve are kept — they are display
// metadata, and an unresolvable clock must not silently delete content. When
// nothing is dropped the original metadata string is returned untouched.
func dropPreWindowPoints(metadata string, windowStart time.Time, anchor time.Time, loc *time.Location) string {
	var meta struct {
		AppSites       []string            `json:"appSites"`
		Distractions   []string            `json:"distractions"`
		ActivityPoints []cardActivityPoint `json:"activityPoints"`
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

// shellOverlapsWindow pre-resolves the shell's clocks and keeps only cards
// overlapping the batch window. Shells whose clocks do not resolve at all are
// kept — ReplaceCardsInRange reports them as SkippedCards, which fails the
// batch loudly instead of silently here.
func (s *Service) shellOverlapsWindow(shell domain.CardShell, batch storage.Batch) bool {
	loc := s.loc()
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
