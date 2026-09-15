package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// A tiny valid JPEG (1x1 pixel) for the frame source.
var tinyJPEG = []byte{
	0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x00, 0x00, 0x01,
	0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9,
}

// harness wires a Service against a real temp-dir store, a fake chain of one
// scripted provider, and an in-memory frame source.
type harness struct {
	store          *storage.Store
	provider       *fakeProvider
	service        *Service
	days           []string
	failures       []string
	failedAttempts []int
	framesDir      string
}

func newHarness(t *testing.T, responses map[string]string) *harness {
	t.Helper()
	store, err := storage.Open(context.Background(), storage.Options{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	framesDir := t.TempDir()
	h := &harness{store: store, provider: newFakeProvider(responses), framesDir: framesDir}

	// Tests reference a Coding category beyond the built-ins.
	if err := store.Categories().Save(context.Background(), []domain.Category{{
		ID: "00000000-0000-4000-8000-0000000000aa", Name: "Coding", ColorHex: "#1E90FF", SortOrder: 1,
	}}); err != nil {
		t.Fatalf("seed Coding category: %v", err)
	}

	service, err := New(Config{
		Store:            store.Analysis(),
		Cards:            store.Cards(),
		Categories:       store.Categories(),
		Providers:        fakeChainSource{provider: h.provider},
		Media:            dirFrameSource{dir: framesDir},
		Now:              func() time.Time { return testNow },
		TickEvery:        time.Hour, // tests drive tick() directly
		Workers:          1,
		OnCardsCommitted: func(days []string) { h.days = append(h.days, days...) },
		OnBatchFailed: func(batch storage.Batch, kind, _ string) {
			h.failures = append(h.failures, kind)
			h.failedAttempts = append(h.failedAttempts, batch.Attempts)
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	h.service = service
	return h
}

var testNow = time.Date(2026, 9, 12, 20, 0, 0, 0, time.Local)

// commitFrames writes frame files and their DB rows, spaced by interval.
func (h *harness) commitFrames(t *testing.T, at time.Time, n int, interval time.Duration, idle func(i int) *int) []storage.AnalysisFrame {
	t.Helper()
	frames := make([]storage.AnalysisFrame, 0, n)
	for i := range n {
		path := filepath.ToSlash(filepath.Join("staging", at.Add(time.Duration(i)*interval).Format("150405.000000000")+".jpg"))
		if err := os.MkdirAll(filepath.Join(h.framesDir, filepath.FromSlash(filepath.Dir(path))), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(h.framesDir, filepath.FromSlash(path)), tinyJPEG, 0o600); err != nil {
			t.Fatalf("write frame: %v", err)
		}
		id, err := h.store.Captures().Begin(context.Background(), path,
			at.Add(time.Duration(i)*interval), idle(i), 1280, 720, false)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		if err := h.store.Captures().Commit(context.Background(), id, int64(len(tinyJPEG))); err != nil {
			t.Fatalf("commit: %v", err)
		}
		sid, err := h.frameID(path)
		if err != nil {
			t.Fatalf("read frame id: %v", err)
		}
		frames = append(frames, storage.AnalysisFrame{
			ID: sid, SegmentPath: path, CapturedAt: at.Add(time.Duration(i) * interval),
			IdleSeconds: idle(i), FileSize: int64(len(tinyJPEG)),
		})
	}
	return frames
}

// fakeProvider answers by purpose with canned JSON.
type fakeProvider struct {
	mu        sync.Mutex
	responses map[string]string
	err       error
	calls     []ai.Request
	block     chan struct{}
}

func newFakeProvider(responses map[string]string) *fakeProvider {
	return &fakeProvider{responses: responses}
}

func (p *fakeProvider) Generate(_ context.Context, request ai.Request) (ai.Result, error) {
	p.mu.Lock()
	p.calls = append(p.calls, request)
	response, ok := p.responses[string(request.Purpose)]
	err := p.err
	block := p.block
	p.mu.Unlock()

	if block != nil {
		<-block
	}
	if err != nil {
		return ai.Result{}, err
	}
	if !ok {
		return ai.Result{}, ai.NewError(ai.ErrorInvalidRequest, "no scripted response for purpose "+string(request.Purpose), 0, nil)
	}
	return ai.Result{Text: response, Model: "fixture-model"}, nil
}

func (p *fakeProvider) callCount(purpose string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, r := range p.calls {
		if string(r.Purpose) == purpose {
			n++
		}
	}
	return n
}

type fakeChainSource struct{ provider *fakeProvider }

func (f fakeChainSource) AnalysisChain(context.Context) (*ai.Chain, error) {
	return ai.NewChain([]ai.ChainEntry{{ID: "fixture", Provider: f.provider}}, 0), nil
}

func (fakeChainSource) ImageCap(context.Context) int { return 0 }

type dirFrameSource struct{ dir string }

func (d dirFrameSource) FrameBytes(_ context.Context, segmentPath string, frameIndex int) ([]byte, error) {
	if frameIndex != 0 {
		return nil, errors.New("frame index must be 0 for staging frames")
	}
	return os.ReadFile(filepath.Join(d.dir, filepath.FromSlash(segmentPath)))
}

// The end-to-end happy path: frames → transcribe → observations → cards →
// succeeded batch, cards visible through the real CardRepo, day notification
// fired.
func TestPipelineHappyPath(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"Working in an editor","apps":["Code"]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"editor","title":"Editing code","summary":"Working in an editor.","detailed_summary":"","appSites":["Code"],"distractions":[],"activityPoints":[]}]}`,
	})

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	// 92 frames at 10s: the 92nd seals a 91-frame batch; the tail stays out.
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })

	h.service.tick(context.Background())

	batches := mustBatches(t, h.store)
	if len(batches) != 1 || batches[0].Status != storage.BatchSucceeded {
		t.Fatalf("batches = %+v, want one succeeded", batches)
	}

	cards, err := h.store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if err != nil {
		t.Fatalf("CardsForDay: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("cards = %d, want 1", len(cards))
	}
	card := cards[0]
	if card.Category != "Coding" || card.Title != "Editing code" {
		t.Fatalf("card = %+v", card)
	}
	var meta struct {
		AppSites []string `json:"appSites"`
	}
	if err := json.Unmarshal([]byte(card.Metadata), &meta); err != nil || len(meta.AppSites) != 1 || meta.AppSites[0] != "Code" {
		t.Fatalf("metadata = %q (%v)", card.Metadata, err)
	}

	if len(h.days) != 1 || h.days[0] != "2026-09-12" {
		t.Fatalf("notified days = %v", h.days)
	}
	if h.provider.callCount(string(ai.PurposeTranscribe)) == 0 {
		t.Fatal("transcription never called")
	}
	if h.provider.callCount(string(ai.PurposeCards)) != 1 {
		t.Fatalf("card calls = %d, want 1", h.provider.callCount(string(ai.PurposeCards)))
	}
}

// The idle fast path: no provider call at all, an Idle card lands directly.
func TestPipelineIdleFastPath(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[]}`,
	})
	// A provider call would be a design violation, so fail loudly if made.
	h.provider.err = ai.NewError(ai.ErrorInvalidRequest, "idle path must not call the provider", 0, nil)

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(600) })

	h.service.tick(context.Background())

	batches := mustBatches(t, h.store)
	if len(batches) != 1 || batches[0].Status != storage.BatchSucceeded {
		t.Fatalf("batches = %+v, want one succeeded idle batch", batches)
	}
	cards, _ := h.store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if len(cards) != 1 || cards[0].Category != "Idle" {
		t.Fatalf("cards = %+v, want one Idle card", cards)
	}
}

// A provider error fails the batch with the mapped kind and notifies.
func TestPipelineProviderFailure(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{}`,
	})
	h.provider.err = ai.NewError(ai.ErrorAuthentication, "key rejected", 401, nil)

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })

	h.service.tick(context.Background())

	batches := mustBatches(t, h.store)
	if len(batches) != 1 || batches[0].Status != storage.BatchFailed {
		t.Fatalf("batches = %+v, want one failed", batches)
	}
	if batches[0].FailureKind != "auth" {
		t.Fatalf("failure kind = %q, want auth", batches[0].FailureKind)
	}
	if len(h.failures) != 1 || h.failures[0] != "auth" {
		t.Fatalf("failure notifications = %v", h.failures)
	}
	// The store bumped attempts to 1 when entering the failed state; the
	// event's snapshot must carry the same value, or the UI's Retryable flag
	// runs one attempt behind and promises a retry the requeue loop refuses.
	if len(h.failedAttempts) != 1 || h.failedAttempts[0] != 1 {
		t.Fatalf("failure event attempts = %v, want [1]", h.failedAttempts)
	}
}

// failureKind must not let non-ai errors inherit ErrorKindOf's
// ErrorUnavailable default: a missing frame file is not a network problem.
func TestFailureKindClassification(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"no provider", ai.ErrNoProvider, "no_provider"},
		{"frame read failure", fmt.Errorf("read frame staging/x.jpg: %w", os.ErrNotExist), "internal"},
		{"plain error", errors.New("decode cards: unexpected EOF"), "internal"},
		{"canceled", context.Canceled, "canceled"},
		{"auth", ai.NewError(ai.ErrorAuthentication, "key rejected", 401, nil), "auth"},
		{"rate limited", ai.NewError(ai.ErrorRateLimited, "slow down", 429, nil), "rate_limited"},
		{"unavailable", ai.NewError(ai.ErrorUnavailable, "down", 503, nil), "network"},
		{"timeout", ai.NewError(ai.ErrorTimeout, "timed out", 0, nil), "network"},
		{"invalid output", ai.NewError(ai.ErrorInvalidOutput, "bad json", 0, nil), "invalid_output"},
		{"invalid request", ai.NewError(ai.ErrorInvalidRequest, "bad param", 400, nil), "invalid_request"},
		{"unsupported", ai.NewError(ai.ErrorUnsupportedFeature, "no json mode", 400, nil), "invalid_request"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := failureKind(c.err); got != c.want {
				t.Fatalf("failureKind(%v) = %q, want %q", c.err, got, c.want)
			}
		})
	}
}

// A frame file that vanished from disk must fail the batch as internal, not
// network: ai.ErrorKindOf's default would otherwise promise a pointless retry.
func TestPipelineMissingFrameFailsInternal(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{}`,
	})
	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	frames := h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	if err := os.Remove(filepath.Join(h.framesDir, filepath.FromSlash(frames[0].SegmentPath))); err != nil {
		t.Fatalf("remove frame: %v", err)
	}

	h.service.tick(context.Background())

	batches := mustBatches(t, h.store)
	if len(batches) != 1 || batches[0].Status != storage.BatchFailed {
		t.Fatalf("batches = %+v, want one failed", batches)
	}
	if batches[0].FailureKind != "internal" {
		t.Fatalf("failure kind = %q, want internal", batches[0].FailureKind)
	}
}

// Transcription that yields zero observations marks failed_empty.
func TestPipelineEmptyTranscription(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[]}`,
	})

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })

	h.service.tick(context.Background())

	batches := mustBatches(t, h.store)
	if len(batches) != 1 || batches[0].Status != storage.BatchFailedEmpty {
		t.Fatalf("batches = %+v, want one failed_empty", batches)
	}
}

// A closed batch below the minimum duration is skipped_short and never
// reaches the provider.
func TestPipelineShortBatchSkipped(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{}`,
	})
	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	// 20 frames (190s), then a gap, then 2 frames: the first run is closed
	// by the gap but below 5 minutes.
	frames := h.commitFrames(t, base, 20, 10*time.Second, func(int) *int { return intPtr(5) })
	h.commitFrames(t, frames[19].CapturedAt.Add(3*time.Minute), 2, 10*time.Second, func(int) *int { return intPtr(5) })

	h.service.tick(context.Background())

	batches := mustBatches(t, h.store)
	if len(batches) != 1 || batches[0].Status != storage.BatchSkippedShort {
		t.Fatalf("batches = %+v, want one skipped_short", batches)
	}
	if h.provider.callCount(string(ai.PurposeTranscribe)) != 0 {
		t.Fatal("provider called for a skipped_short batch")
	}
}

// The latest run below the target span stays unbatched: no batch, no call.
func TestPipelineLatestRunWaits(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{}`,
	})
	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	// 50 frames = 490s, no gap: all latest, below target.
	h.commitFrames(t, base, 50, 10*time.Second, func(int) *int { return intPtr(5) })

	h.service.tick(context.Background())

	if batches := mustBatches(t, h.store); len(batches) != 0 {
		t.Fatalf("batches = %+v, want none (latest run still filling)", batches)
	}
}

// An unknown category from the model maps to System, never a new category.
func TestPipelineUnknownCategoryBecomesSystem(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"working","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Not A Real Category","subcategory":"","title":"T","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
	})

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })

	h.service.tick(context.Background())

	cards, _ := h.store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if len(cards) != 1 || cards[0].Category != "System" {
		t.Fatalf("cards = %+v, want one System card", cards)
	}
}

// The model may not assign the built-in Idle category from screen content —
// even when it echoes the name it saw in nearby-card context, the card falls
// back to System; only the hardware idle fast path writes Idle (docs/04 §4.4).
func TestPipelineModelCannotAssignIdleCategory(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"staring at a static screen","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Idle","subcategory":"","title":"T","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
	})

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })

	h.service.tick(context.Background())

	cards, _ := h.store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if len(cards) != 1 || cards[0].Category != "System" {
		t.Fatalf("cards = %+v, want one System card (model Idle rejected)", cards)
	}
}

// Cancellation mid-transcription leaves the batch in processing (adopted on
// the next run) and does not count as a failure.
func TestPipelineCancellationKeepsProcessing(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{}`,
	})
	block := make(chan struct{})
	h.provider.block = block

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		h.service.tick(ctx)
		close(done)
	}()
	// Let the provider call start, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()
	close(block)
	<-done

	batches := mustBatches(t, h.store)
	if len(batches) != 1 || batches[0].Status != storage.BatchProcessing {
		t.Fatalf("batches = %+v, want one still processing", batches)
	}
	if len(h.failures) != 0 {
		t.Fatalf("cancellation recorded as failure: %v", h.failures)
	}
}

// A card the model places entirely outside the batch window is dropped, not
// duplicated outside the rewrite range.
func TestPipelineOutOfWindowCardDropped(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"working","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"","title":"In window","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]},{"start":"8:00 AM","end":"8:30 AM","category":"Coding","subcategory":"","title":"Way outside","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
	})

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })

	h.service.tick(context.Background())

	cards, _ := h.store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if len(cards) != 1 || cards[0].Title != "In window" {
		t.Fatalf("cards = %+v, want only the in-window card", cards)
	}
}

// mustBatches reads all batches through the repository's range query over an
// unbounded window — the public surface an external package can use.
func mustBatches(t *testing.T, store *storage.Store) []storage.Batch {
	t.Helper()
	batches, err := store.Analysis().BatchesInRange(context.Background(),
		time.Unix(0, 0), time.Unix(1<<40, 0))
	if err != nil {
		t.Fatalf("query batches: %v", err)
	}
	return batches
}

// frameID finds the screenshot id via the unbatched-frames query: the frames
// we just committed are by definition unbatched.
func (h *harness) frameID(path string) (int64, error) {
	frames, err := h.store.Analysis().UnbatchedFrames(context.Background(), time.Unix(0, 0), time.Unix(1<<40, 0))
	if err != nil {
		return 0, err
	}
	for _, f := range frames {
		if f.SegmentPath == path {
			return f.ID, nil
		}
	}
	return 0, fmt.Errorf("frame %s not found", path)
}

// A provider that emits glued meridiem clock strings ("10:21AM") — a common
// model deviation from the prompt's format — must still land cards, not fail
// the batch as unresolvable forever.
func TestPipelineGluedMeridiemClocks(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"Working in an editor","apps":["Code"]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00AM","end":"10:15AM","category":"Coding","subcategory":"editor","title":"Glued clocks","summary":"Working in an editor.","detailed_summary":"","appSites":["Code"],"distractions":[],"activityPoints":[]}]}`,
	})

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })

	h.service.tick(context.Background())

	batches := mustBatches(t, h.store)
	if len(batches) != 1 || batches[0].Status != storage.BatchSucceeded {
		t.Fatalf("batches = %+v, want one succeeded", batches)
	}
	cards, _ := h.store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if len(cards) != 1 || cards[0].Title != "Glued clocks" {
		t.Fatalf("cards = %+v, want the glued-clock card", cards)
	}
	if cards[0].StartTs != base.Unix() {
		t.Fatalf("start_ts = %d, want %d (10:00 AM local)", cards[0].StartTs, base.Unix())
	}
}

// A batch that fails deterministically stops being requeued once its attempts
// are exhausted: the cooldown loop must not re-run LLM calls forever.
func TestPipelineAttemptsExhaustedStopsRetrying(t *testing.T) {
	// The card stage returns a card whose end clock is unresolvable, so the
	// batch fails on SkippedCards every time it runs.
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"working","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"garbage","category":"Coding","subcategory":"","title":"T","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
	})

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })

	// The harness clock is fixed at testNow, so the cooldown (updated_at <
	// now - 10min) never passes on later ticks. Drive the requeue directly:
	// one tick creates and fails the batch, then each iteration simulates
	// the cooldown having elapsed by requeueing through the repository.
	h.service.tick(context.Background())
	cardCalls := h.provider.callCount(string(ai.PurposeCards))
	if cardCalls != 1 {
		t.Fatalf("card calls after first tick = %d, want 1", cardCalls)
	}

	ctx := context.Background()
	for i := range storage.MaxBatchAttempts {
		// Cooldown elapsed: requeue the failed batch and fail it again. The
		// processBatch path stamps updated_at with the harness's fixed clock,
		// so the requeue timestamp is derived from the batch's own updated_at
		// (one second past it) rather than a wall clock.
		ua := mustBatches(t, h.store)[0].UpdatedAt
		at := ua.Add(time.Second)
		n, err := h.store.Analysis().RequeueFailed(ctx, at, at)
		if err != nil {
			t.Fatalf("requeue %d: %v", i, err)
		}
		if n == 0 {
			if i < storage.MaxBatchAttempts-1 {
				t.Fatalf("requeue refused after %d failures, before the limit", i)
			}
			break
		}
		pending := mustPending(t, h.store)
		if len(pending) != 1 {
			t.Fatalf("pending after requeue %d = %d, want 1", i, len(pending))
		}
		if err := h.service.processBatch(ctx, pending[0]); err != nil {
			h.service.failBatch(ctx, pending[0], err)
		}
	}

	// After the loop the batch is terminal: further requeue attempts move
	// nothing and no LLM call happens.
	callsAfterLoop := h.provider.callCount(string(ai.PurposeCards))
	ua := mustBatches(t, h.store)[0].UpdatedAt
	at := ua.Add(time.Hour)
	if _, err := h.store.Analysis().RequeueFailed(ctx, at, at); err != nil {
		t.Fatalf("final requeue: %v", err)
	}
	if pending := mustPending(t, h.store); len(pending) != 0 {
		t.Fatalf("pending after exhaustion = %d, want 0", len(pending))
	}
	if got := h.provider.callCount(string(ai.PurposeCards)); got != callsAfterLoop {
		t.Fatalf("card calls after exhaustion = %d, want %d (no further LLM spend)", got, callsAfterLoop)
	}
	if got := h.provider.callCount(string(ai.PurposeCards)); got > storage.MaxBatchAttempts+1 {
		t.Fatalf("card calls = %d, want at most %d (capped, not unbounded)", got, storage.MaxBatchAttempts+1)
	}
}

func mustPending(t *testing.T, store *storage.Store) []storage.Batch {
	t.Helper()
	batches, err := store.Analysis().PendingBatches(context.Background())
	if err != nil {
		t.Fatalf("query pending: %v", err)
	}
	return batches
}

// A merge card absorbs the nearby card it continues: the model emits one card
// starting at the earlier card's start, and the rewrite must remove the
// earlier card instead of leaving both in parallel (docs/03 §3.5 overlap
// predicate lets a boundary-touching predecessor escape otherwise).
func TestPipelineMergeCardAbsorbsPredecessor(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"working","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"","title":"first","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
	})

	// Batch 1 (10:00–10:15): one card.
	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	h.service.tick(context.Background())

	cards, _ := h.store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if len(cards) != 1 || cards[0].Title != "first" {
		t.Fatalf("seed card = %+v, want one 'first'", cards)
	}

	// Batch 2 (10:15–10:30): the model merges with the first card, emitting
	// one card whose start is the first card's start.
	h.provider.mu.Lock()
	h.provider.responses[string(ai.PurposeCards)] = `{"cards":[{"start":"10:00 AM","end":"10:30 AM","category":"Coding","subcategory":"","title":"merged","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[{"time":"10:00 AM","description":"first"},{"time":"10:20 AM","description":"second"}]}]}`
	h.provider.mu.Unlock()
	base2 := time.Date(2026, 9, 12, 10, 16, 0, 0, time.Local)
	h.commitFrames(t, base2, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	h.service.tick(context.Background())

	cards, _ = h.store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if len(cards) != 1 {
		t.Fatalf("cards after merge = %+v, want exactly one", cards)
	}
	merged := cards[0]
	if merged.Title != "merged" || merged.Start != "10:00 AM" || merged.End != "10:30 AM" {
		t.Fatalf("merged card = %s – %s %q, want 10:00 AM – 10:30 AM merged", merged.Start, merged.End, merged.Title)
	}
}

// The same merge, but the predecessor is a System fallback card (the model
// emitted an unknown category name). Sparing other batches' System cards in
// the rewrite left both cards on screen in parallel — the reported bug.
func TestPipelineMergeCardAbsorbsSystemPredecessor(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"working","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"DeepFocus","subcategory":"","title":"first","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
	})

	// Batch 1: the unknown category falls back to System (docs/04 §4.3.4).
	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	h.service.tick(context.Background())

	cards, _ := h.store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if len(cards) != 1 || cards[0].Category != "System" {
		t.Fatalf("seed card = %+v, want one System fallback card", cards)
	}

	// Batch 2 merges into the System card.
	h.provider.mu.Lock()
	h.provider.responses[string(ai.PurposeCards)] = `{"cards":[{"start":"10:00 AM","end":"10:30 AM","category":"Coding","subcategory":"","title":"merged","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[{"time":"10:00 AM","description":"first"},{"time":"10:20 AM","description":"second"}]}]}`
	h.provider.mu.Unlock()
	base2 := time.Date(2026, 9, 12, 10, 16, 0, 0, time.Local)
	h.commitFrames(t, base2, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	h.service.tick(context.Background())

	cards, _ = h.store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if len(cards) != 1 {
		t.Fatalf("cards after merge = %+v, want exactly one (System predecessor absorbed)", cards)
	}
	merged := cards[0]
	if merged.Title != "merged" || merged.Category != "Coding" ||
		merged.Start != "10:00 AM" || merged.End != "10:30 AM" {
		t.Fatalf("merged card = %s – %s %s %q, want 10:00 AM – 10:30 AM Coding merged",
			merged.Start, merged.End, merged.Category, merged.Title)
	}
}

// The merge gate: a shell claiming a merge into a predecessor card of a
// different category is clamped back to the batch window. The predecessor
// survives beside a card covering only the current window, and the absorbed
// activityPoints from before the window are dropped.
func TestPipelineMergeGateRejectsCategoryMismatch(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"working","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"","title":"first","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
	})
	// A second real category, so the mismatch is between two known names and
	// not an unknown name that would fall back to System.
	if err := h.store.Categories().Save(context.Background(), []domain.Category{
		{ID: "00000000-0000-4000-8000-0000000000aa", Name: "Coding", ColorHex: "#1E90FF", SortOrder: 1},
		{ID: "00000000-0000-4000-8000-0000000000bb", Name: "Communication", ColorHex: "#32CD32", SortOrder: 2},
	}); err != nil {
		t.Fatalf("seed categories: %v", err)
	}

	// Batch 1 (10:00–10:15): one Coding card.
	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	h.service.tick(context.Background())

	cards, _ := h.store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if len(cards) != 1 || cards[0].Category != "Coding" {
		t.Fatalf("seed card = %+v, want one Coding card", cards)
	}

	// Batch 2 (10:15–10:30): the model claims a merge with the Coding card
	// (start at its start) but categorizes the combined activity Communication.
	h.provider.mu.Lock()
	h.provider.responses[string(ai.PurposeCards)] = `{"cards":[{"start":"10:00 AM","end":"10:30 AM","category":"Communication","subcategory":"","title":"merged","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[{"time":"10:00 AM","description":"first"},{"time":"10:20 AM","description":"second"}]}]}`
	h.provider.mu.Unlock()
	base2 := time.Date(2026, 9, 12, 10, 16, 0, 0, time.Local)
	h.commitFrames(t, base2, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	h.service.tick(context.Background())

	cards, _ = h.store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if len(cards) != 2 {
		t.Fatalf("cards = %+v, want the predecessor preserved beside the clamped card", cards)
	}
	predecessor, clamped := cards[0], cards[1]
	if predecessor.Title != "first" || predecessor.Category != "Coding" ||
		predecessor.Start != "10:00 AM" || predecessor.End != "10:15 AM" {
		t.Fatalf("predecessor = %s – %s %s %q, want the untouched Coding 'first'",
			predecessor.Start, predecessor.End, predecessor.Category, predecessor.Title)
	}
	if clamped.Title != "merged" || clamped.Category != "Communication" ||
		clamped.Start != "10:15 AM" || clamped.End != "10:30 AM" {
		t.Fatalf("clamped card = %s – %s %s %q, want 10:15 AM – 10:30 AM Communication merged",
			clamped.Start, clamped.End, clamped.Category, clamped.Title)
	}
	var meta struct {
		ActivityPoints []cardActivityPoint `json:"activityPoints"`
	}
	if err := json.Unmarshal([]byte(clamped.Metadata), &meta); err != nil {
		t.Fatalf("decode clamped card metadata: %v", err)
	}
	if len(meta.ActivityPoints) != 1 || meta.ActivityPoints[0].Time != "10:20 AM" {
		t.Fatalf("activityPoints = %+v, want only the in-window 10:20 AM point", meta.ActivityPoints)
	}
}

// The idle fast path merges consecutive idle batches into one card
// (commitIdleCard, docs/04 §4.4): the preceding Idle card must be absorbed,
// not left beside its replacement.
func TestPipelineIdleMergeAbsorbsPrecedingIdleCard(t *testing.T) {
	h := newHarness(t, map[string]string{})
	h.provider.err = ai.NewError(ai.ErrorInvalidRequest, "idle path must not call the provider", 0, nil)

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(600) })
	h.service.tick(context.Background())

	// The next idle batch starts within AdjacentIdleMergeGap of the first
	// card's end, so it must merge rather than open a parallel card.
	base2 := base.Add(16 * time.Minute)
	h.commitFrames(t, base2, 92, 10*time.Second, func(int) *int { return intPtr(600) })
	h.service.tick(context.Background())

	batches := mustBatches(t, h.store)
	if len(batches) != 2 {
		t.Fatalf("batches = %+v, want two succeeded idle batches", batches)
	}
	second := batches[len(batches)-1]

	cards, _ := h.store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if len(cards) != 1 {
		t.Fatalf("cards = %+v, want exactly one merged Idle card", cards)
	}
	merged := cards[0]
	if merged.Category != "Idle" || merged.Start != "10:00 AM" || merged.StartTs != base.Unix() {
		t.Fatalf("merged idle card = %s – %s %s, want start 10:00 AM Idle", merged.Start, merged.End, merged.Category)
	}
	// Card clocks are minute-granularity strings, so the merged end is the
	// second batch's end truncated to the minute.
	if want := second.End.Truncate(time.Minute).Unix(); merged.EndTs != want {
		t.Fatalf("merged idle card end_ts = %d, want %d (the second batch's end)", merged.EndTs, want)
	}
}

func TestBoundSummary(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty passes through", "", ""},
		{"within limit untouched", "在编辑器里调试登录接口", "在编辑器里调试登录接口"},
		{
			"over limit cuts at a word boundary",
			strings.Repeat("word ", 30),
			strings.Repeat("word ", 26) + "word",
		},
		{
			"counts runes not bytes",
			strings.Repeat("汉", 160),
			strings.Repeat("汉", 135),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := boundSummary(tc.in); got != tc.want {
				t.Fatalf("boundSummary(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestBoundDetailedSummary(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty passes through", "", ""},
		{"within limits untouched", "10:00 AM–10:05 AM: worked", "10:00 AM–10:05 AM: worked"},
		{
			"over 15 paragraphs keeps the newest 15",
			"p1\np2\np3\np4\np5\np6\np7\np8\np9\np10\np11\np12\np13\np14\np15\np16",
			"p2\np3\np4\np5\np6\np7\np8\np9\np10\np11\np12\np13\np14\np15\np16",
		},
		{
			"over 2500 runes keeps the newest paragraph truncated",
			strings.Repeat("a", 1500) + "\n" + strings.Repeat("b", 1500),
			strings.Repeat("a", 1500) + "\n" + strings.Repeat("b", 999),
		},
		{
			"single huge paragraph truncates at a word boundary",
			strings.Repeat("word ", 600),
			strings.Repeat("word ", 499) + "word",
		},
		{
			"counts runes not bytes",
			strings.Repeat("汉", 2600),
			strings.Repeat("汉", 2500),
		},
		{
			"crlf normalized before splitting",
			"one\r\n\r\ntwo",
			"one\n\ntwo",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := boundDetailedSummary(tc.in); got != tc.want {
				t.Fatalf("boundDetailedSummary(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
