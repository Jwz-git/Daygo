package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	store     *storage.Store
	provider  *fakeProvider
	service   *Service
	days      []string
	failures  []string
	framesDir string
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
		OnBatchFailed:    func(_ storage.Batch, kind, _ string) { h.failures = append(h.failures, kind) },
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
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"editor","title":"Editing code","summary":"Working in an editor.","detailed_summary":"","appSites":["Code"],"distractions":[]}]}`,
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
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Not A Real Category","subcategory":"","title":"T","summary":"S","detailed_summary":"","appSites":[],"distractions":[]}]}`,
	})

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })

	h.service.tick(context.Background())

	cards, _ := h.store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if len(cards) != 1 || cards[0].Category != "System" {
		t.Fatalf("cards = %+v, want one System card", cards)
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
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"","title":"In window","summary":"S","detailed_summary":"","appSites":[],"distractions":[]},{"start":"8:00 AM","end":"8:30 AM","category":"Coding","subcategory":"","title":"Way outside","summary":"S","detailed_summary":"","appSites":[],"distractions":[]}]}`,
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
