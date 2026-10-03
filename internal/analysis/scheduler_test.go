package analysis

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// A broken historical request must not prevent today's frames from being
// batched, or make today's first request wait for the historical backlog.
func TestSchedulerDiscoversTodayBeforeHistoricalRequests(t *testing.T) {
	h := newHarness(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	old := h.commitFrames(t, testNow.AddDate(0, 0, -3), 31, 30*time.Second, func(int) *int { return nil })
	batch, err := h.store.Analysis().CreateBatch(ctx, old, storage.BatchPending, testNow)
	if err != nil {
		t.Fatal(err)
	}
	fresh := h.commitFrames(t, testNow.Add(-time.Hour), 31, 30*time.Second, func(int) *int { return nil })
	provider := schedulerProvider{generate: func(callCtx context.Context, req ai.Request) (ai.Result, error) {
		pending := mustPending(t, h.store)
		if len(pending) != 1 || pending[0].ID != batch.ID {
			t.Errorf("first request left pending = %+v, want only historical batch (today already processing)", pending)
		}
		unbatched, err := h.store.Analysis().UnbatchedFrames(callCtx, fresh[0].CapturedAt, testNow)
		if err != nil || len(unbatched) != 0 {
			t.Errorf("today's unbatched frames at first request = %d, err = %v; want 0", len(unbatched), err)
		}
		cancel()
		return ai.Result{}, context.Canceled
	}}
	h.service.cfg.Providers = schedulerChainSource{provider}
	h.service.tick(ctx)
}

// Both classes make progress, and each class remains chronological. Idle
// fixtures make the commit order observable without depending on model text.
func TestSchedulerAlternatesTodayAndHistory(t *testing.T) {
	h := newHarness(t, nil)
	var want []time.Time
	var oldBatches, currentBatches []storage.Batch
	for _, hours := range []int{3, 1} {
		for _, historical := range []bool{true, false} {
			at := testNow.Add(-time.Duration(hours) * time.Hour)
			if historical {
				at = at.AddDate(0, 0, -2).Add(30 * time.Minute)
			}
			frames := h.commitFrames(t, at, 31, 30*time.Second, func(int) *int { return intPtr(3600) })
			batch, err := h.store.Analysis().CreateBatch(context.Background(), frames, storage.BatchPending, testNow)
			if err != nil {
				t.Fatal(err)
			}
			if historical {
				oldBatches = append(oldBatches, batch)
			} else {
				currentBatches = append(currentBatches, batch)
			}
		}
	}
	want = []time.Time{currentBatches[0].Start, oldBatches[0].Start, currentBatches[1].Start, oldBatches[1].Start}
	var got []time.Time
	h.service.cfg.OnCardsCommitted = func([]string) {
		for _, batch := range mustBatches(t, h.store) {
			if batch.Status == storage.BatchSucceeded {
				seen := false
				for _, at := range got {
					seen = seen || at.Equal(batch.Start)
				}
				if !seen {
					got = append(got, batch.Start)
				}
			}
		}
	}
	h.service.tick(context.Background())
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commit order = %v, want %v", got, want)
	}
}

// Recording continues while an old request runs. Discover the newly completed
// live batch at the next request boundary instead of finishing the old queue.
func TestSchedulerDiscoversFramesBetweenRequests(t *testing.T) {
	h := newHarness(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, hours := range []int{3, 1} {
		frames := h.commitFrames(t, testNow.AddDate(0, 0, -2).Add(-time.Duration(hours)*time.Hour+30*time.Minute), 31, 30*time.Second, func(int) *int { return nil })
		if _, err := h.store.Analysis().CreateBatch(ctx, frames, storage.BatchPending, testNow); err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	h.service.cfg.Providers = schedulerChainSource{schedulerProvider{generate: func(callCtx context.Context, req ai.Request) (ai.Result, error) {
		calls++
		if calls == 1 {
			h.commitFrames(t, testNow.Add(-time.Hour), 31, 30*time.Second, func(int) *int { return nil })
			return ai.Result{}, ai.NewError(ai.ErrorInvalidRequest, "anonymous rejected request", 400, nil)
		}
		for _, batch := range mustBatches(t, h.store) {
			if batch.Status == storage.BatchProcessing && batch.Start.Before(testNow.Add(-24*time.Hour)) {
				t.Errorf("second request is historical: %+v; want the newly discovered live batch", batch)
			}
		}
		frames, err := h.store.Analysis().UnbatchedFrames(callCtx, testNow.Add(-time.Hour), testNow)
		if err != nil || len(frames) != 0 {
			t.Errorf("live unbatched frames at second request = %d, err = %v; want 0", len(frames), err)
		}
		cancel()
		return ai.Result{}, context.Canceled
	}}}
	h.service.tick(ctx)
	if calls != 2 {
		t.Fatalf("requests = %d, want 2", calls)
	}
}

func TestSchedulerDiscoversFramesDuringRateLimitCooldown(t *testing.T) {
	h := newHarness(t, nil)
	h.commitFrames(t, testNow.Add(-time.Hour), 31, 30*time.Second, func(int) *int { return nil })
	h.service.rateLimitBackoffUntil = testNow.Add(time.Minute)
	h.service.tick(context.Background())
	pending := mustPending(t, h.store)
	if len(pending) != 1 || pending[0].Attempts != 0 {
		t.Fatalf("cooldown batches = %+v, want one pending with no attempts", pending)
	}
	if h.provider.callCount(string(ai.PurposeTranscribe)) != 0 {
		t.Fatal("provider called during cooldown")
	}
}

// Deferral must not spin or hide other ready batches in the same tick.
func TestSchedulerDeferredTodayDoesNotBlockHistory(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	old := h.commitFrames(t, testNow.AddDate(0, 0, -2), 31, 30*time.Second, func(int) *int { return intPtr(3600) })
	if _, err := h.store.Analysis().CreateBatch(ctx, old, storage.BatchPending, testNow); err != nil {
		t.Fatal(err)
	}
	fresh := h.commitFrames(t, testNow.Add(-time.Hour), 31, 30*time.Second, func(int) *int { return nil })
	h.activeSegment = fresh[len(fresh)-1].SegmentPath
	h.service.tick(ctx)
	batches := mustBatches(t, h.store)
	if len(batches) != 2 || batches[0].Status != storage.BatchSucceeded || batches[1].Status != storage.BatchPending || batches[1].Attempts != 0 {
		t.Fatalf("deferred batches = %+v, want historical succeeded and live pending without attempts", batches)
	}
}

// Before 04:00 the previous calendar date is still the live logical day.
// This also protects stores whose zone differs from the process time zone.
func TestSchedulerPriorityUsesLogicalDay(t *testing.T) {
	for _, zone := range []string{"Asia/Shanghai", "Asia/Kathmandu", "America/New_York"} {
		t.Run(zone, func(t *testing.T) {
			loc, err := time.LoadLocation(zone)
			if err != nil {
				t.Fatal(err)
			}
			h := newHarness(t, nil)
			now := time.Date(2026, 9, 13, 3, 30, 0, 0, loc)
			h.service.cfg.Location = loc
			h.service.cfg.Now = func() time.Time { return now }
			var liveID int64
			for i, at := range []time.Time{
				time.Date(2026, 9, 12, 3, 0, 0, 0, loc),
				time.Date(2026, 9, 12, 23, 0, 0, 0, loc),
				time.Date(2026, 9, 13, 1, 0, 0, 0, loc),
			} {
				frames := h.commitFrames(t, at, 31, 30*time.Second, func(int) *int { return nil })
				batch, err := h.store.Analysis().CreateBatch(context.Background(), frames, storage.BatchPending, now)
				if err != nil {
					t.Fatal(err)
				}
				if i == 1 {
					liveID = batch.ID
				}
			}
			got, ok, err := h.service.nextPendingBatch(context.Background(), nil, true)
			if err != nil || !ok || got.ID != liveID {
				t.Fatalf("next = %+v, ok = %v, err = %v; want previous calendar evening batch %d", got, ok, err, liveID)
			}
		})
	}
}

func TestSchedulerCrossBoundaryBatchIsCurrent(t *testing.T) {
	h := newHarness(t, nil)
	now := time.Date(2026, 9, 13, 5, 0, 0, 0, time.Local)
	h.service.cfg.Now = func() time.Time { return now }
	var liveID int64
	for i, at := range []time.Time{now.Add(-2 * time.Hour), now.Add(-65 * time.Minute)} {
		frames := h.commitFrames(t, at, 31, 30*time.Second, func(int) *int { return nil })
		batch, err := h.store.Analysis().CreateBatch(context.Background(), frames, storage.BatchPending, now)
		if err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			liveID = batch.ID
		}
	}
	got, ok, err := h.service.nextPendingBatch(context.Background(), nil, true)
	if err != nil || !ok || got.ID != liveID {
		t.Fatalf("next = %+v, ok = %v, err = %v; want 03:55-04:10 batch %d", got, ok, err, liveID)
	}
}

// A live rewrite can absorb the predecessor before 04:00. Its pending
// historical retry must run first, or running it later would truncate the
// freshly merged live card at the historical batch's end.
func TestSchedulerPendingPredecessorRunsBeforeLiveMerge(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	now := time.Date(2026, 9, 13, 5, 0, 0, 0, time.Local)
	h.service.cfg.Now = func() time.Time { return now }
	oldStart := now.Add(-90 * time.Minute)
	oldFrames := h.commitFrames(t, oldStart, 31, 30*time.Second, func(int) *int { return nil })
	oldBatch, err := h.store.Analysis().CreateBatch(ctx, oldFrames, storage.BatchPending, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.Cards().ReplaceCardsInRange(ctx, oldStart, oldStart.Add(15*time.Minute), []domain.CardShell{{
		Start: "3:30 AM", End: "3:45 AM", Category: "Coding", Title: "Anonymous predecessor", Summary: "Fixture",
	}}, oldBatch.ID); err != nil {
		t.Fatal(err)
	}
	fresh := h.commitFrames(t, now.Add(-74*time.Minute), 31, 30*time.Second, func(int) *int { return nil })
	if _, err := h.store.Analysis().CreateBatch(ctx, fresh, storage.BatchPending, now); err != nil {
		t.Fatal(err)
	}
	got, ok, err := h.service.nextPendingBatch(ctx, nil, true)
	if err != nil || !ok || got.ID != oldBatch.ID {
		t.Fatalf("next = %+v, ok = %v, err = %v; want pending predecessor %d before live merge", got, ok, err, oldBatch.ID)
	}
	// If the predecessor was deferred in its active segment, leave the live
	// merge pending too instead of running it ahead of the deferred rewrite.
	got, ok, err = h.service.nextPendingBatch(ctx, map[int64]bool{oldBatch.ID: true}, true)
	if err != nil || ok {
		t.Fatalf("after predecessor deferral: next = %+v, ok = %v, err = %v; want no runnable merge", got, ok, err)
	}
}

type schedulerProvider struct {
	generate func(context.Context, ai.Request) (ai.Result, error)
}

func (p schedulerProvider) Generate(ctx context.Context, req ai.Request) (ai.Result, error) {
	if p.generate == nil {
		return ai.Result{}, errors.New("missing scheduler fixture")
	}
	return p.generate(ctx, req)
}

type schedulerChainSource struct{ provider ai.Provider }

func (s schedulerChainSource) AnalysisChain(context.Context) (*ai.Chain, error) {
	return ai.NewChain([]ai.ChainEntry{{ID: "scheduler-fixture", Provider: s.provider}}, 0), nil
}

func (schedulerChainSource) ImageCap(context.Context) int { return 5 }
