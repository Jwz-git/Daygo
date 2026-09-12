package storage

import (
	"context"
	"testing"
	"time"
)

// insertFrames commits screenshot rows through the recorder's Commit path
// (pending → screenshots), each with a unique segment path, and returns them
// as AnalysisFrame values spaced 10 seconds apart.
func insertFrames(t *testing.T, store *Store, at time.Time, count int) []AnalysisFrame {
	t.Helper()
	frames := make([]AnalysisFrame, 0, count)
	for i := 0; i < count; i++ {
		path := "staging/frame-" + at.Format("150405") + "-" + string(rune('a'+i)) + ".jpg"
		pending, err := store.Captures().Begin(context.Background(),
			path, at.Add(time.Duration(i)*10*time.Second), nil, 1280, 720, false)
		if err != nil {
			t.Fatalf("Begin frame %d: %v", i, err)
		}
		if err := store.Captures().Commit(context.Background(), pending, 2048); err != nil {
			t.Fatalf("Commit frame %d: %v", i, err)
		}
		var id int64
		if err := store.db.QueryRowContext(context.Background(),
			"SELECT id FROM screenshots WHERE segment_path = ?", path).Scan(&id); err != nil {
			t.Fatalf("read inserted frame %d: %v", i, err)
		}
		frames = append(frames, AnalysisFrame{
			ID: id, SegmentPath: path, FrameIndex: 0,
			CapturedAt: at.Add(time.Duration(i) * 10 * time.Second), FileSize: 2048,
		})
	}
	return frames
}

// UnbatchedFrames returns only committed frames outside any batch; a frame in
// a failed batch stays out — retry is the batch's job.
func TestUnbatchedFramesExcludesBatchedFrames(t *testing.T) {
	store := openWriter(t, newDir(t))
	ctx := context.Background()
	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)

	frames := insertFrames(t, store, base, 6)
	now := base.Add(time.Hour)

	if got := len(mustUnbatched(t, store, base.Add(-time.Minute), now)); got != 6 {
		t.Fatalf("unbatched = %d before any batch, want 6", got)
	}

	batch, err := store.Analysis().CreateBatch(ctx, frames[:3], BatchPending, now)
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	if got := len(mustUnbatched(t, store, base.Add(-time.Minute), now)); got != 3 {
		t.Fatalf("unbatched = %d after one batch, want 3", got)
	}

	// Frames of a failed batch must not resurface.
	if err := store.Analysis().SetBatchStatus(ctx, batch.ID, BatchProcessing, "", "", now); err != nil {
		t.Fatalf("SetBatchStatus processing: %v", err)
	}
	if err := store.Analysis().SetBatchStatus(ctx, batch.ID, BatchFailed, "network", "note", now); err != nil {
		t.Fatalf("SetBatchStatus failed: %v", err)
	}
	if got := len(mustUnbatched(t, store, base.Add(-time.Minute), now)); got != 3 {
		t.Fatalf("unbatched = %d after the batch failed, want 3 (retry belongs to the batch)", got)
	}

	// The window is half-open: a frame at until is out.
	if got := len(mustUnbatched(t, store, frames[5].CapturedAt.Add(time.Second), now)); got != 0 {
		t.Fatalf("unbatched = %d with since past all frames, want 0", got)
	}
}

func mustUnbatched(t *testing.T, store *Store, since, until time.Time) []AnalysisFrame {
	t.Helper()
	frames, err := store.Analysis().UnbatchedFrames(context.Background(), since, until)
	if err != nil {
		t.Fatalf("UnbatchedFrames: %v", err)
	}
	return frames
}

// CreateBatch is atomic: the batch row and every join row land together, and
// the span is first-to-last frame, not wall time.
func TestCreateBatchPersistsSpanAndMembership(t *testing.T) {
	store := openWriter(t, newDir(t))
	ctx := context.Background()
	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)

	frames := insertFrames(t, store, base, 4)
	now := base.Add(time.Hour)

	batch, err := store.Analysis().CreateBatch(ctx, frames, BatchPending, now)
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	if !batch.Start.Equal(frames[0].CapturedAt) || !batch.End.Equal(frames[3].CapturedAt) {
		t.Fatalf("batch span = %v..%v, want %v..%v",
			batch.Start, batch.End, frames[0].CapturedAt, frames[3].CapturedAt)
	}

	got, err := store.Analysis().FramesForBatch(ctx, batch.ID)
	if err != nil {
		t.Fatalf("FramesForBatch: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("frames for batch = %d, want 4", len(got))
	}

	if rowCount(t, store, "batch_screenshots") != 4 {
		t.Fatalf("batch_screenshots rows = %d, want 4", rowCount(t, store, "batch_screenshots"))
	}

	if _, err := store.Analysis().CreateBatch(ctx, nil, BatchPending, now); err == nil {
		t.Fatal("CreateBatch accepted no frames")
	}
	if _, err := store.Analysis().CreateBatch(ctx, frames, BatchSucceeded, now); err == nil {
		t.Fatal("CreateBatch accepted a non-initial status")
	}
}

// SetBatchStatus enforces the state machine: invalid transitions and failed
// states without a kind are rejected, failed info is cleared on success.
func TestSetBatchStatusEnforcesStateMachine(t *testing.T) {
	store := openWriter(t, newDir(t))
	ctx := context.Background()
	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	frames := insertFrames(t, store, base, 2)
	now := base.Add(time.Hour)

	batch, err := store.Analysis().CreateBatch(ctx, frames, BatchPending, now)
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	// pending -> succeeded skips processing: rejected.
	if err := store.Analysis().SetBatchStatus(ctx, batch.ID, BatchSucceeded, "", "", now); err == nil {
		t.Fatal("pending -> succeeded accepted")
	}
	// failed without a kind: rejected.
	if err := store.Analysis().SetBatchStatus(ctx, batch.ID, BatchProcessing, "", "", now); err != nil {
		t.Fatalf("pending -> processing: %v", err)
	}
	if err := store.Analysis().SetBatchStatus(ctx, batch.ID, BatchFailed, "", "", now); err == nil {
		t.Fatal("failed without kind accepted")
	}
	// failure info on a success state: rejected.
	if err := store.Analysis().SetBatchStatus(ctx, batch.ID, BatchFailedEmpty, "empty", "", now); err != nil {
		t.Fatalf("processing -> failed_empty: %v", err)
	}
	if err := store.Analysis().SetBatchStatus(ctx, batch.ID, BatchSucceeded, "kind", "", now); err == nil {
		t.Fatal("succeeded with failure info accepted")
	}

	// Drive a second batch to the success terminal state, which has no
	// outgoing edges.
	frames2 := insertFrames(t, store, base.Add(time.Hour), 2)
	done, err := store.Analysis().CreateBatch(ctx, frames2, BatchPending, now)
	if err != nil {
		t.Fatalf("CreateBatch second: %v", err)
	}
	if err := store.Analysis().SetBatchStatus(ctx, done.ID, BatchProcessing, "", "", now); err != nil {
		t.Fatalf("second -> processing: %v", err)
	}
	if err := store.Analysis().SetBatchStatus(ctx, done.ID, BatchSucceeded, "", "", now); err != nil {
		t.Fatalf("second -> succeeded: %v", err)
	}
	if err := store.Analysis().SetBatchStatus(ctx, done.ID, BatchPending, "", "", now); err == nil {
		t.Fatal("succeeded -> pending accepted")
	}
	if err := store.Analysis().SetBatchStatus(ctx, done.ID, BatchFailed, "kind", "", now); err == nil {
		t.Fatal("succeeded -> failed accepted")
	}

	// Unknown batch.
	err = store.Analysis().SetBatchStatus(ctx, 9999, BatchProcessing, "", "", now)
	assertKind(t, err, KindNotFound)
}

// AdoptStaleProcessing reopens interrupted batches; RequeueFailed respects
// the updated_at cooldown.
func TestAdoptAndRequeue(t *testing.T) {
	store := openWriter(t, newDir(t))
	ctx := context.Background()
	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	now := base.Add(time.Hour)

	mk := func(offset time.Duration) Batch {
		frames := insertFrames(t, store, base.Add(offset), 2)
		b, err := store.Analysis().CreateBatch(ctx, frames, BatchPending, now)
		if err != nil {
			t.Fatalf("CreateBatch: %v", err)
		}
		if err := store.Analysis().SetBatchStatus(ctx, b.ID, BatchProcessing, "", "", now); err != nil {
			t.Fatalf("SetBatchStatus: %v", err)
		}
		return b
	}

	mk(0)
	mk(time.Hour)

	n, err := store.Analysis().AdoptStaleProcessing(ctx, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("AdoptStaleProcessing: %v", err)
	}
	if n != 2 {
		t.Fatalf("adopted = %d, want 2", n)
	}
	if got := len(mustPending(t, store)); got != 2 {
		t.Fatalf("pending after adopt = %d, want 2", got)
	}
	// Idempotent.
	if n, _ = store.Analysis().AdoptStaleProcessing(ctx, now.Add(time.Minute)); n != 0 {
		t.Fatalf("second adopt moved %d, want 0", n)
	}

	// A failed batch inside the cooldown stays failed. The batch failed at
	// updated_at = now; a 10-minute cooldown means only calls whose "now minus
	// cooldown" is past that instant may requeue it.
	failed := mustPending(t, store)[0]
	if err := store.Analysis().SetBatchStatus(ctx, failed.ID, BatchProcessing, "", "", now); err != nil {
		t.Fatalf("SetBatchStatus: %v", err)
	}
	if err := store.Analysis().SetBatchStatus(ctx, failed.ID, BatchFailed, "network", "", now); err != nil {
		t.Fatalf("SetBatchStatus failed: %v", err)
	}
	if n, _ = store.Analysis().RequeueFailed(ctx, now.Add(-4*time.Minute), now.Add(6*time.Minute)); n != 0 {
		t.Fatalf("requeued inside cooldown = %d, want 0", n)
	}
	if n, _ = store.Analysis().RequeueFailed(ctx, now.Add(time.Minute), now.Add(11*time.Minute)); n != 1 {
		t.Fatalf("requeued after cooldown = %d, want 1", n)
	}
}

func mustPending(t *testing.T, store *Store) []Batch {
	t.Helper()
	batches, err := store.Analysis().PendingBatches(context.Background())
	if err != nil {
		t.Fatalf("PendingBatches: %v", err)
	}
	return batches
}

// Observations round-trip per batch and by range overlap.
func TestObservationsRoundTrip(t *testing.T) {
	store := openWriter(t, newDir(t))
	ctx := context.Background()
	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	now := base.Add(time.Hour)

	frames := insertFrames(t, store, base, 3)
	batch, err := store.Analysis().CreateBatch(ctx, frames, BatchPending, now)
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	obs := []Observation{
		{Start: base, End: base.Add(time.Minute), Observation: "reading docs", Metadata: `{"apps":["Safari"]}`},
		{Start: base.Add(time.Minute), End: base.Add(2 * time.Minute), Observation: "writing code"},
	}
	if err := store.Analysis().InsertObservations(ctx, batch.ID, obs, now); err != nil {
		t.Fatalf("InsertObservations: %v", err)
	}
	if err := store.Analysis().InsertObservations(ctx, batch.ID, nil, now); err == nil {
		t.Fatal("empty insert accepted")
	}
	if err := store.Analysis().InsertObservations(ctx, batch.ID, []Observation{{Start: base, Observation: ""}}, now); err == nil {
		t.Fatal("empty observation text accepted")
	}

	got, err := store.Analysis().ObservationsForBatch(ctx, batch.ID)
	if err != nil {
		t.Fatalf("ObservationsForBatch: %v", err)
	}
	if len(got) != 2 || got[0].Observation != "reading docs" || got[0].Metadata != `{"apps":["Safari"]}` || got[1].Metadata != "" {
		t.Fatalf("observations for batch = %+v", got)
	}

	inRange, err := store.Analysis().ObservationsInRange(ctx, base.Add(30*time.Second), base.Add(90*time.Second))
	if err != nil {
		t.Fatalf("ObservationsInRange: %v", err)
	}
	if len(inRange) != 2 {
		t.Fatalf("observations in range = %d, want 2", len(inRange))
	}

	empty, err := store.Analysis().ObservationsInRange(ctx, base.Add(time.Hour), base.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("ObservationsInRange: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("observations in far range = %d, want 0", len(empty))
	}
}

// ProcessingBatchesInRange feeds the timeline's processing indicator: pending
// and processing overlap the window, terminal states do not.
func TestProcessingBatchesInRange(t *testing.T) {
	store := openWriter(t, newDir(t))
	ctx := context.Background()
	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	now := base.Add(3 * time.Hour)

	frames := insertFrames(t, store, base, 2)
	pending, err := store.Analysis().CreateBatch(ctx, frames, BatchPending, now)
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	frames2 := insertFrames(t, store, base.Add(time.Hour), 2)
	processing, err := store.Analysis().CreateBatch(ctx, frames2, BatchPending, now)
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	if err := store.Analysis().SetBatchStatus(ctx, processing.ID, BatchProcessing, "", "", now); err != nil {
		t.Fatalf("SetBatchStatus: %v", err)
	}

	frames3 := insertFrames(t, store, base.Add(2*time.Hour), 2)
	done, err := store.Analysis().CreateBatch(ctx, frames3, BatchPending, now)
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	if err := store.Analysis().SetBatchStatus(ctx, done.ID, BatchProcessing, "", "", now); err != nil {
		t.Fatalf("SetBatchStatus: %v", err)
	}
	if err := store.Analysis().SetBatchStatus(ctx, done.ID, BatchSucceeded, "", "", now); err != nil {
		t.Fatalf("SetBatchStatus: %v", err)
	}

	got, err := store.Analysis().ProcessingBatchesInRange(ctx, base.Add(-time.Minute), base.Add(3*time.Hour))
	if err != nil {
		t.Fatalf("ProcessingBatchesInRange: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("processing in range = %d, want 2 (pending + processing, not succeeded)", len(got))
	}
	_ = pending
}
