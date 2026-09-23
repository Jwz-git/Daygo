package analysis

import (
	"context"
	"database/sql"
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
	"github.com/Jwz-git/Daygo/internal/timeutil"
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
	// activeSegment is the path the fake recorder is "writing"; the Config's
	// ActiveSegment closure returns it, so a test can pin a batch's tail segment
	// as active and then clear it to model a rollover.
	activeSegment string
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
		ActiveSegment:    func() string { return h.activeSegment },
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
		id, err := h.store.Captures().Begin(context.Background(), path, 0,
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
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"editor","title":"Editing code","summary":"Working in an editor.","detailed_summary":"","appSites":["Code"],"distractions":[{"start":"10:05 AM","end":"10:07 AM","title":"checked a feed","summary":""}],"activityPoints":[]}]}`,
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
	// appSites crosses the layer boundary: the model returns a flat list, and
	// internal/app reads the primary/secondary object of docs/05 §5.5.2. The
	// shape is restated here rather than shared with the producer so the two
	// sides cannot drift together.
	var meta struct {
		AppSites *struct {
			Primary   *string `json:"primary"`
			Secondary *string `json:"secondary"`
		} `json:"appSites"`
		Distractions []struct {
			StartTime string `json:"startTime"`
			EndTime   string `json:"endTime"`
			Title     string `json:"title"`
			Summary   string `json:"summary"`
		} `json:"distractions"`
	}
	if err := json.Unmarshal([]byte(card.Metadata), &meta); err != nil {
		t.Fatalf("metadata = %q (%v)", card.Metadata, err)
	}
	if meta.AppSites == nil || meta.AppSites.Primary == nil || *meta.AppSites.Primary != "Code" {
		t.Fatalf("appSites = %+v, want primary Code from the model's flat list", meta.AppSites)
	}
	// Distractions cross the same boundary: the model names the range start/end
	// and the consumer reads the docs/05 §5.5.2 object, whose clock fields are
	// startTime/endTime. A distraction stored in any other shape is a
	// distraction the inspector cannot place on the day.
	if len(meta.Distractions) != 1 {
		t.Fatalf("distractions = %+v, want the model's one entry mapped", meta.Distractions)
	}
	mapped := meta.Distractions[0]
	if mapped.StartTime != "10:05 AM" || mapped.EndTime != "10:07 AM" || mapped.Title != "checked a feed" {
		t.Fatalf("distraction = %+v, want the model's range under the contract's field names", mapped)
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

// A batch whose frames still live in the segment the recorder is actively
// writing is deferred, not failed: the container is unfinalized and its frames
// cannot be decoded yet. It stays pending with no attempt counted, and the next
// tick processes it once the segment has finalized (active segment cleared).
func TestPipelineDefersBatchInActiveSegment(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"working","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"","title":"Editing","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
	})

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	// 92 frames at 10s seal a 91-frame batch (frames 0..90); the tail stays out.
	frames := h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })

	// The recorder is still writing the segment holding the batch's newest frame.
	h.activeSegment = frames[90].SegmentPath

	h.service.tick(context.Background())

	batches := mustBatches(t, h.store)
	if len(batches) != 1 || batches[0].Status != storage.BatchPending {
		t.Fatalf("batches = %+v, want one still pending (deferred)", batches)
	}
	if len(h.failures) != 0 {
		t.Fatalf("deferral recorded as failure: %v", h.failures)
	}
	if batches[0].Attempts != 0 {
		t.Fatalf("deferral counted an attempt: %d", batches[0].Attempts)
	}
	if n := h.provider.callCount(string(ai.PurposeTranscribe)); n != 0 {
		t.Fatalf("deferred batch called the provider %d times, want 0", n)
	}

	// The segment rolls over and finalizes: the same batch now processes on the
	// next tick with no manual retry.
	h.activeSegment = ""
	h.service.tick(context.Background())

	batches = mustBatches(t, h.store)
	if len(batches) != 1 || batches[0].Status != storage.BatchSucceeded {
		t.Fatalf("batches = %+v, want one succeeded after the segment finalized", batches)
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

// When every returned card sits outside the owned rewrite span, the
// correction pass needs the rejected clock range and the required window.
// Reporting only "no cards returned" hides the actual defect and gives the
// model no deterministic way to repair its otherwise non-empty response.
func TestPipelineAllCardsOutsideWindowReportsActionableCorrection(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"working","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"8:00 AM","end":"8:30 AM","category":"Coding","subcategory":"","title":"Wrong window","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
	})

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })

	h.service.tick(context.Background())

	batches := mustBatches(t, h.store)
	if len(batches) != 1 || batches[0].Status != storage.BatchFailed {
		t.Fatalf("batches = %+v, want one failed", batches)
	}
	if !strings.Contains(batches[0].FailureNote, `card 1 (Wrong window) spans 8:00 AM-8:30 AM outside`) {
		t.Fatalf("failure note = %q, want rejected card and clock range", batches[0].FailureNote)
	}

	h.provider.mu.Lock()
	calls := append([]ai.Request(nil), h.provider.calls...)
	h.provider.mu.Unlock()
	var correction string
	for _, call := range calls {
		if call.Purpose == ai.PurposeCards && len(call.Parts) > 0 && strings.Contains(call.Parts[0].Text(), "validation errors") {
			correction = call.Parts[0].Text()
			break
		}
	}
	if !strings.Contains(correction, "10:00 AM to 10:15 AM") {
		t.Fatalf("correction prompt = %q, want required rewrite window", correction)
	}
	if !strings.Contains(correction, `"title":"Wrong window"`) {
		t.Fatalf("correction prompt = %q, want previous JSON output", correction)
	}
	if strings.Contains(correction, "ongoing conversation") {
		t.Fatalf("correction prompt claims unavailable conversation state: %q", correction)
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
	// The correction loop spends 1 + 2 correction calls inside the first tick
	// (Dayflow's three-attempt cap) before the stage gives up.
	cardCalls := h.provider.callCount(string(ai.PurposeCards))
	if cardCalls != 3 {
		t.Fatalf("card calls after first tick = %d, want 3", cardCalls)
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
	// Per tick the card stage spends 1 + 2 correction calls; across the
	// attempt cap the spend stays bounded, just 3x the old per-tick figure.
	if got := h.provider.callCount(string(ai.PurposeCards)); got > (storage.MaxBatchAttempts+1)*3 {
		t.Fatalf("card calls = %d, want at most %d (capped, not unbounded)",
			got, (storage.MaxBatchAttempts+1)*3)
	}
}

// TestFailBatchClearsRateLimitTally guards the slow leak where a batch that was
// rate-limited and then failed kept its rateLimitCount entry forever (batch IDs
// only grow, so the map never shrank on a long-running agent).
func TestFailBatchClearsRateLimitTally(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, map[string]string{})
	frames := h.commitFrames(t, testNow, 3, 10*time.Second, func(int) *int { return nil })
	batch, err := h.store.Analysis().CreateBatch(ctx, frames, storage.BatchPending, testNow)
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	// A transient rate limit seeds a per-batch tally.
	h.service.handleBatchRateLimit(ctx, batch, errors.New("rate limited"))
	if got := h.rateLimitEntries(); got != 1 {
		t.Fatalf("after rate limit: rateLimitCount entries = %d, want 1", got)
	}

	// The batch then reaches a terminal failure; its tally must not survive.
	h.service.failBatch(ctx, batch, errors.New("boom"))
	if got := h.rateLimitEntries(); got != 0 {
		t.Fatalf("after failBatch: rateLimitCount entries = %d, want 0 (leak)", got)
	}
}

func (h *harness) rateLimitEntries() int {
	h.service.queueMu.Lock()
	defer h.service.queueMu.Unlock()
	return len(h.service.rateLimitCount)
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

// Reprocessing a batch whose earlier output had merged backward must rewrite
// that whole merged span. The old implementation treated the rerun as fresh,
// deleted the straddling card because it overlapped the batch window, and only
// rebuilt from the batch start — silently losing the card's earlier prefix.
func TestPipelineReprocessPreservesStraddlingCardPrefix(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"working","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"","title":"first","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
	})

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	h.service.tick(context.Background())

	h.provider.mu.Lock()
	h.provider.responses[string(ai.PurposeCards)] = `{"cards":[{"start":"10:00 AM","end":"10:30 AM","category":"Coding","subcategory":"","title":"merged","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`
	h.provider.mu.Unlock()
	base2 := time.Date(2026, 9, 12, 10, 16, 0, 0, time.Local)
	h.commitFrames(t, base2, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	h.service.tick(context.Background())

	batches, err := h.store.Analysis().BatchesInRange(context.Background(), base2.Add(-time.Minute), base2.Add(20*time.Minute))
	if err != nil || len(batches) != 1 {
		t.Fatalf("second batch = %+v, err=%v; want one", batches, err)
	}
	if _, err := h.store.Analysis().ReprocessBatches(context.Background(), []int64{batches[0].ID}, testNow.Add(time.Minute)); err != nil {
		t.Fatalf("reprocess batch: %v", err)
	}

	// The rerun may split the merged span again, but it must cover from 10:00
	// rather than dropping 10:00–10:16. The 10-minute tail is legal: the card
	// carrying the rewrite's end is the one card the 15-minute floor exempts
	// (2026-09-21).
	h.provider.mu.Lock()
	h.provider.responses[string(ai.PurposeCards)] = `{"cards":[{"start":"10:00 AM","end":"10:20 AM","category":"Coding","subcategory":"","title":"before","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]},{"start":"10:20 AM","end":"10:30 AM","category":"Coding","subcategory":"","title":"after","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`
	h.provider.mu.Unlock()
	h.service.tick(context.Background())

	cards, err := h.store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if err != nil {
		t.Fatalf("cards after reprocess: %v", err)
	}
	if len(cards) != 2 || cards[0].Start != "10:00 AM" || cards[0].End != "10:20 AM" ||
		cards[1].Start != "10:20 AM" || cards[1].End != "10:30 AM" {
		t.Fatalf("cards after reprocess = %+v, want continuous 10:00–10:30 coverage", cards)
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

// The clamp writes a clock string, which carries minutes: an owned start with
// seconds must round up, or the card lands back inside the predecessor the gate
// refused and the rewrite hits the storage ownership constraint. A start that is
// already on the minute must stay put — rounding it up would open a fake gap.
func TestPipelineMergeClampRoundsUpToTheMinute(t *testing.T) {
	h := newHarness(t, map[string]string{})
	batch := storage.Batch{
		Start: time.Date(2026, 9, 12, 10, 15, 10, 0, time.Local),
		End:   time.Date(2026, 9, 12, 10, 30, 0, 0, time.Local),
	}

	clamped := h.service.clampToMergeFloor(domain.CardShell{Start: "9:30 AM", End: "10:30 AM"}, batch.Start, batch)
	if clamped.Start != "10:16 AM" {
		t.Fatalf("clamped start = %q, want 10:16 AM (up from 10:15:10)", clamped.Start)
	}

	aligned := storage.Batch{
		Start: time.Date(2026, 9, 12, 10, 16, 0, 0, time.Local),
		End:   time.Date(2026, 9, 12, 10, 31, 0, 0, time.Local),
	}
	clamped = h.service.clampToMergeFloor(domain.CardShell{Start: "9:30 AM", End: "10:31 AM"}, aligned.Start, aligned)
	if clamped.Start != "10:16 AM" {
		t.Fatalf("clamped start = %q, want the owned start 10:16 AM unchanged", clamped.Start)
	}

	// A claim the gate granted is not touched: at the owned start, and later.
	kept := h.service.clampToMergeFloor(domain.CardShell{Start: "10:16 AM", End: "10:31 AM"}, aligned.Start, aligned)
	if kept.Start != "10:16 AM" {
		t.Fatalf("clamped start = %q, want the granted claim 10:16 AM untouched", kept.Start)
	}
	kept = h.service.clampToMergeFloor(domain.CardShell{Start: "10:20 AM", End: "10:31 AM"}, aligned.Start, aligned)
	if kept.Start != "10:20 AM" {
		t.Fatalf("clamped start = %q, want the in-window claim 10:20 AM untouched", kept.Start)
	}

	// A merge the gate granted owns the predecessor's start; the claim sits on
	// the floor and must survive as written.
	granted := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	kept = h.service.clampToMergeFloor(domain.CardShell{Start: "10:00 AM", End: "10:31 AM"}, granted, aligned)
	if kept.Start != "10:00 AM" {
		t.Fatalf("clamped start = %q, want the merged span start 10:00 AM untouched", kept.Start)
	}
}

// The merge gate: a shell claiming a merge into a predecessor card of a
// different category is refused. Absorbing it would file the predecessor's
// minutes under the new card's category and corrupt the day's category totals,
// so the predecessor survives and the rewrite starts at the batch window
// instead, with the claimed pre-window activityPoints dropped.
func TestPipelineMergeGateRefusesCrossCategoryPredecessor(t *testing.T) {
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
	// The gate refuses the cross-category swallow: the Coding predecessor keeps
	// its span and category, and the Communication card covers only the window.
	if len(cards) != 2 {
		t.Fatalf("cards = %+v, want the predecessor preserved beside the clamped card", cards)
	}
	predecessor, clamped := cards[0], cards[1]
	if predecessor.Category != "Coding" || predecessor.Start != "10:00 AM" || predecessor.End != "10:15 AM" {
		t.Fatalf("predecessor = %s – %s %s, want the untouched Coding card",
			predecessor.Start, predecessor.End, predecessor.Category)
	}
	if clamped.Category != "Communication" || clamped.StartTs != base2.Unix() || clamped.End != "10:30 AM" {
		t.Fatalf("clamped = %s – %s %s, want the Communication rewrite starting at the window %s",
			clamped.Start, clamped.End, clamped.Category, base2.Format("3:04 PM"))
	}
	var meta struct {
		ActivityPoints []cardActivityPoint `json:"activityPoints"`
	}
	if err := json.Unmarshal([]byte(clamped.Metadata), &meta); err != nil {
		t.Fatalf("decode clamped card metadata: %v", err)
	}
	// The refused claim's pre-window point is gone; the predecessor keeps it.
	if len(meta.ActivityPoints) != 1 || meta.ActivityPoints[0].Time != "10:20 AM" {
		t.Fatalf("activityPoints = %+v, want only the in-window 10:20 AM point", meta.ActivityPoints)
	}
}

func TestPipelineMergeGateDropsRefusedPredecessorEndingBeforeFloor(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"working","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"","title":"first","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
	})
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

	// Batch 2 (10:16–10:30): the model returns TWO cards:
	// 1. A hallucinated repetition of predecessor (10:00 AM - 10:15 AM)
	// 2. The real card for this window (10:16 AM - 10:30 AM)
	h.provider.mu.Lock()
	h.provider.responses[string(ai.PurposeCards)] = `{"cards":[
		{"start":"10:00 AM","end":"10:15 AM","category":"Communication","subcategory":"","title":"hallucinated-predecessor","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]},
		{"start":"10:16 AM","end":"10:30 AM","category":"Communication","subcategory":"","title":"current-card","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}
	]}`
	h.provider.mu.Unlock()
	base2 := time.Date(2026, 9, 12, 10, 16, 0, 0, time.Local)
	h.commitFrames(t, base2, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	h.service.tick(context.Background())

	cards, _ = h.store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if len(cards) != 2 {
		t.Fatalf("cards = %+v (len %d), want exactly 2 (predecessor and current-card, dropped hallucination)", cards, len(cards))
	}
	if cards[0].Title != "first" || cards[1].Title != "current-card" {
		t.Fatalf("cards = %+v, want first and current-card", cards)
	}
	for _, c := range cards {
		if c.EndTs <= c.StartTs || c.EndTs-c.StartTs > 4*3600 {
			t.Fatalf("card duration invalid: %+v", c)
		}
	}
}

// seedCardBefore parks one card in the lookback window of the batch under test.
// Fixtures build multi-card history this way rather than through extra batches:
// the merge gate is about what the rewrite finds on the ground, and driving four
// batches through the pipeline would test the batcher instead.
func (h *harness) seedCardBefore(t *testing.T, batchID int64, start, end time.Time,
	category, title, metadata string) {
	t.Helper()
	loc := h.store.Location()
	shell := domain.CardShell{
		Start:    timeutil.FormatClock(start, loc),
		End:      timeutil.FormatClock(end, loc),
		Category: category,
		Title:    title,
		Summary:  "S",
		Metadata: metadata,
	}
	if _, err := h.store.Cards().ReplaceCardsInRange(context.Background(), start, end,
		[]domain.CardShell{shell}, batchID); err != nil {
		t.Fatalf("seed card %q: %v", title, err)
	}
}

func (h *harness) onlyBatch(t *testing.T, at time.Time) int64 {
	t.Helper()
	batches, err := h.store.Analysis().BatchesInRange(context.Background(),
		at.Add(-time.Hour), at.Add(time.Hour))
	if err != nil || len(batches) != 1 {
		t.Fatalf("batches = %+v, err = %v; want one", batches, err)
	}
	return batches[0].ID
}

func (h *harness) cardsFor(t *testing.T, day string) []domain.TimelineCard {
	t.Helper()
	cards, err := h.store.Cards().CardsForDay(context.Background(), day)
	if err != nil {
		t.Fatalf("cards for %s: %v", day, err)
	}
	return cards
}

// Four cards lead into the batch window — the shape a user sees as "four cards
// fused into one". Same-category cards may be absorbed into the continuing
// card; the merge is allowed and the rewrite owns the whole span.
func TestPipelineMergeGateAllowsSameCategoryChain(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"working","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"","title":"first","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
	})
	if err := h.store.Categories().Save(context.Background(), []domain.Category{
		{ID: "00000000-0000-4000-8000-0000000000aa", Name: "Coding", ColorHex: "#1E90FF", SortOrder: 1},
		{ID: "00000000-0000-4000-8000-0000000000bb", Name: "Communication", ColorHex: "#32CD32", SortOrder: 2},
	}); err != nil {
		t.Fatalf("seed categories: %v", err)
	}

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	h.service.tick(context.Background())
	batchID := h.onlyBatch(t, base)

	// Three short Coding cards before the window, plus the one the batch wrote.
	h.seedCardBefore(t, batchID, base.Add(-30*time.Minute), base.Add(-20*time.Minute), "Coding", "older", "")
	h.seedCardBefore(t, batchID, base.Add(-20*time.Minute), base.Add(-10*time.Minute), "Coding", "middle", "")

	h.provider.mu.Lock()
	h.provider.responses[string(ai.PurposeCards)] = `{"cards":[{"start":"9:30 AM","end":"10:30 AM","category":"Coding","subcategory":"","title":"fused","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`
	h.provider.mu.Unlock()
	base2 := time.Date(2026, 9, 12, 10, 16, 0, 0, time.Local)
	h.commitFrames(t, base2, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	h.service.tick(context.Background())

	cards := h.cardsFor(t, "2026-09-12")
	if len(cards) != 1 {
		t.Fatalf("cards = %+v, want the four same-category cards fused into one", cards)
	}
	if cards[0].Title != "fused" || cards[0].Start != "9:30 AM" || cards[0].Category != "Coding" {
		t.Fatalf("fused card = %s – %s %s %q, want the 9:30 AM Coding rewrite",
			cards[0].Start, cards[0].End, cards[0].Category, cards[0].Title)
	}
}

// The same four-card chain with one card of another category inside it: the
// merge would file those minutes under a category they never had, so the gate
// refuses the extension outright and every earlier card keeps its own span.
func TestPipelineMergeGateRefusesMixedCategoryChain(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"working","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"","title":"first","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
	})
	if err := h.store.Categories().Save(context.Background(), []domain.Category{
		{ID: "00000000-0000-4000-8000-0000000000aa", Name: "Coding", ColorHex: "#1E90FF", SortOrder: 1},
		{ID: "00000000-0000-4000-8000-0000000000bb", Name: "Communication", ColorHex: "#32CD32", SortOrder: 2},
	}); err != nil {
		t.Fatalf("seed categories: %v", err)
	}

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	h.service.tick(context.Background())
	batchID := h.onlyBatch(t, base)

	h.seedCardBefore(t, batchID, base.Add(-30*time.Minute), base.Add(-20*time.Minute), "Coding", "older", "")
	h.seedCardBefore(t, batchID, base.Add(-20*time.Minute), base.Add(-10*time.Minute), "Communication", "chat", "")

	// The model claims one Coding card over the whole chain, chat included.
	h.provider.mu.Lock()
	h.provider.responses[string(ai.PurposeCards)] = `{"cards":[{"start":"9:30 AM","end":"10:30 AM","category":"Coding","subcategory":"","title":"fused","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`
	h.provider.mu.Unlock()
	base2 := time.Date(2026, 9, 12, 10, 16, 0, 0, time.Local)
	h.commitFrames(t, base2, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	h.service.tick(context.Background())

	cards := h.cardsFor(t, "2026-09-12")
	titles := make([]string, 0, len(cards))
	for _, card := range cards {
		titles = append(titles, card.Title)
	}
	// The predecessor chain survives intact — the batch-written card included,
	// since the refused rewrite never reaches back past the window.
	if len(cards) != 4 || titles[0] != "older" || titles[1] != "chat" || titles[2] != "first" || titles[3] != "fused" {
		t.Fatalf("cards = %v, want older + chat + first preserved beside the clamped rewrite", titles)
	}
	if cards[3].StartTs != base2.Unix() || cards[3].Category != "Coding" {
		t.Fatalf("clamped = %s – %s %s, want the rewrite starting at the window %s",
			cards[3].Start, cards[3].End, cards[3].Category, base2.Format("3:04 PM"))
	}
}

// A window sealed below the 15-minute floor is still no licence to swallow a
// cross-category predecessor: the card carrying the window's end is the one the
// floor exempts, so the gate stays shut and the predecessor keeps its span and
// category (2026-09-21 decision — the floor outranks the cross-category refusal
// for merges inside the rewrite span, not beyond its left edge).
func TestPipelineMergeGateStaysShutWhenTheWindowIsBelowTheFloor(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"working","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"9:30 AM","end":"9:45 AM","category":"Coding","subcategory":"","title":"first","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
	})
	if err := h.store.Categories().Save(context.Background(), []domain.Category{
		{ID: "00000000-0000-4000-8000-0000000000aa", Name: "Coding", ColorHex: "#1E90FF", SortOrder: 1},
		{ID: "00000000-0000-4000-8000-0000000000bb", Name: "Communication", ColorHex: "#32CD32", SortOrder: 2},
	}); err != nil {
		t.Fatalf("seed categories: %v", err)
	}

	// Batch 1 (9:30–9:45): one Coding card, the predecessor under test.
	first := time.Date(2026, 9, 12, 9, 30, 0, 0, time.Local)
	h.commitFrames(t, first, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	h.service.tick(context.Background())

	// Batch 2 (10:00–10:10) is sealed by a gap rather than the target duration,
	// so its window is shorter than the floor.
	h.provider.mu.Lock()
	h.provider.responses[string(ai.PurposeCards)] = `{"cards":[{"start":"9:30 AM","end":"10:10 AM","category":"Communication","subcategory":"","title":"merged","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[{"time":"9:30 AM","description":"first"},{"time":"10:05 AM","description":"second"}]}]}`
	h.provider.mu.Unlock()
	second := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, second, 61, 10*time.Second, func(int) *int { return intPtr(5) })
	// The trailing frames only exist to seal the short run; their own span is
	// below the target, so no third batch is created.
	h.commitFrames(t, time.Date(2026, 9, 12, 10, 30, 0, 0, time.Local), 3, 10*time.Second, func(int) *int { return intPtr(5) })
	h.service.tick(context.Background())

	if len(h.failures) != 0 {
		t.Fatalf("failures = %v, want the clamped rewrite to validate on its first attempt", h.failures)
	}
	cards := h.cardsFor(t, "2026-09-12")
	if len(cards) != 2 {
		t.Fatalf("cards = %+v, want the predecessor preserved beside the clamped card", cards)
	}
	predecessor, clamped := cards[0], cards[1]
	if predecessor.Category != "Coding" || predecessor.Start != "9:30 AM" || predecessor.End != "9:45 AM" {
		t.Fatalf("predecessor = %s – %s %s, want the untouched Coding card",
			predecessor.Start, predecessor.End, predecessor.Category)
	}
	// The clamped card is 10 minutes long and carries the rewrite's end: the
	// floor exempts it, which is why the refusal costs the model nothing.
	if clamped.Category != "Communication" || clamped.StartTs != second.Unix() || clamped.End != "10:10 AM" {
		t.Fatalf("clamped = %s – %s %s, want the Communication rewrite starting at the window %s",
			clamped.Start, clamped.End, clamped.Category, second.Format("3:04 PM"))
	}
	if end := time.Unix(clamped.EndTs, 0); end.Sub(time.Unix(clamped.StartTs, 0)) >= minCardDuration {
		t.Fatalf("clamped duration = %s, want the below-floor window this fixture is about",
			end.Sub(time.Unix(clamped.StartTs, 0)))
	}
	var meta struct {
		ActivityPoints []cardActivityPoint `json:"activityPoints"`
	}
	if err := json.Unmarshal([]byte(clamped.Metadata), &meta); err != nil {
		t.Fatalf("decode clamped card metadata: %v", err)
	}
	if len(meta.ActivityPoints) != 1 || meta.ActivityPoints[0].Time != "10:05 AM" {
		t.Fatalf("activityPoints = %+v, want only the in-window 10:05 AM point", meta.ActivityPoints)
	}
}

// A fused card that names no app inherits the icon of the card it absorbed:
// the predecessor is gone, and an empty appSites would leave the merged card
// with no icon where the user used to see one.
func TestPipelineMergedCardInheritsPredecessorAppSites(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"working","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"","title":"first","summary":"S","detailed_summary":"","appSites":["github.com","Google Chrome"],"distractions":[],"activityPoints":[]}]}`,
	})

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	h.service.tick(context.Background())

	cards := h.cardsFor(t, "2026-09-12")
	if len(cards) != 1 {
		t.Fatalf("seed cards = %+v, want one", cards)
	}
	if !strings.Contains(cards[0].Metadata, "github.com") {
		t.Fatalf("seed metadata = %s, want the seeded appSites", cards[0].Metadata)
	}

	// The merged card covers the predecessor's span but names no app of its own.
	h.provider.mu.Lock()
	h.provider.responses[string(ai.PurposeCards)] = `{"cards":[{"start":"10:00 AM","end":"10:30 AM","category":"Coding","subcategory":"","title":"merged","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`
	h.provider.mu.Unlock()
	base2 := time.Date(2026, 9, 12, 10, 16, 0, 0, time.Local)
	h.commitFrames(t, base2, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	h.service.tick(context.Background())

	cards = h.cardsFor(t, "2026-09-12")
	if len(cards) != 1 || cards[0].Title != "merged" {
		t.Fatalf("cards = %+v, want the merged card alone", cards)
	}
	var meta struct {
		AppSites *appSitesMetadata `json:"appSites"`
	}
	if err := json.Unmarshal([]byte(cards[0].Metadata), &meta); err != nil {
		t.Fatalf("decode merged metadata: %v", err)
	}
	if meta.AppSites == nil || meta.AppSites.Primary == nil || *meta.AppSites.Primary != "github.com" {
		t.Fatalf("merged appSites = %+v, want the absorbed card's primary site", meta.AppSites)
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

// A prior non-Idle card whose stored end clock overhangs the idle window (the
// LLM clock-overshoot / ongoing-card case) must not fail the idle batch:
// commitIdleCard clips the overhang to the window edge and keeps the card
// beside the Idle card, instead of hitting the ownership constraint that
// rejects a card starting before the rewrite's owned span.
func TestPipelineIdleAbsorbsOverhangingPredecessor(t *testing.T) {
	h := newHarness(t, map[string]string{})
	h.provider.err = ai.NewError(ai.ErrorInvalidRequest, "idle path must not call the provider", 0, nil)

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	cardStart := base.Add(-10 * time.Minute)
	cardEnd := base.Add(2 * time.Minute) // overhangs where the idle batch begins
	if err := h.store.Write(context.Background(), "seed overhang card", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO timeline_cards (batch_id, day, start, end, start_ts, end_ts, category, title, summary, created_at, updated_at)
			VALUES (NULL, '2026-09-12', ?, ?, ?, ?, 'Coding', 'coding', 's', 0, 0)`,
			timeutil.FormatClock(cardStart, time.Local), timeutil.FormatClock(cardEnd, time.Local),
			cardStart.Unix(), cardEnd.Unix())
		return err
	}); err != nil {
		t.Fatalf("seed overhang card: %v", err)
	}

	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(600) })
	h.service.tick(context.Background())

	batches := mustBatches(t, h.store)
	if len(batches) != 1 || batches[0].Status != storage.BatchSucceeded {
		t.Fatalf("batches = %+v, want one succeeded idle batch (the overhang must not fail it)", batches)
	}

	cards, _ := h.store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if len(cards) != 2 {
		t.Fatalf("cards = %+v, want the clipped Coding card and the Idle card", cards)
	}
	byCat := map[string]domain.TimelineCard{}
	for _, c := range cards {
		byCat[c.Category] = c
	}
	coding, hasCoding := byCat["Coding"]
	idle, hasIdle := byCat["Idle"]
	if !hasCoding || !hasIdle {
		t.Fatalf("cards = %+v, want one Coding and one Idle", cards)
	}
	// The Coding card keeps its own start and is clipped to the idle window
	// start; its category — and thus the daily/weekly totals — is preserved
	// rather than relabeled Idle.
	if coding.StartTs != cardStart.Unix() {
		t.Fatalf("coding start_ts = %d, want %d (start preserved)", coding.StartTs, cardStart.Unix())
	}
	if coding.EndTs > idle.StartTs {
		t.Fatalf("coding end_ts %d overlaps idle start %d; the overhang was not clipped", coding.EndTs, idle.StartTs)
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
