package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// seedCleanupWorld builds the full DB-9 scene: a recordings root with per-frame
// JPEG files, committed screenshots rows oldest-first, one ACTIVE pending
// capture (its staging file must never be deleted), and one frame rented by a
// pending analysis batch. Captured times are stepped so "oldest first" is
// deterministic regardless of test execution speed.
func seedCleanupWorld(t *testing.T, store *Store) (root string, activeFile string) {
	t.Helper()
	ctx := context.Background()
	root = filepath.Join(newDir(t), "recordings")

	// Ten committed frames, 1000 bytes each, one minute apart.
	base := time.Unix(1_700_000_000-10*60, 0)
	for i := 0; i < 10; i++ {
		rel := fmt.Sprintf("staging/frame-%04d.jpg", i)
		capturedAt := base.Add(time.Duration(i) * time.Minute)
		writeFileAt(t, filepath.Join(root, filepath.FromSlash(rel)), 1000, capturedAt)

		id, err := store.Captures().Begin(ctx, rel, 0, capturedAt, nil, 1920, 1080, false)
		if err != nil {
			t.Fatalf("begin %d: %v", i, err)
		}
		if err := store.Captures().Commit(ctx, id, 1000); err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
	}

	// The ACTIVE segment: a pending capture with its staging file on disk.
	activeFile = filepath.Join(root, "staging", "active.jpg")
	writeFileAt(t, activeFile, 2048, time.Now())
	if _, err := store.Captures().Begin(ctx, "staging/active.jpg", 0,
		time.Now(), nil, 1920, 1080, false); err != nil {
		t.Fatalf("begin active: %v", err)
	}

	// One frame rented by a pending analysis batch: the NEWEST one, so the
	// main test's oldest-first deletion never touches it.
	if err := store.Write(ctx, "seed rented batch", func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO analysis_batches (id, start_ts, end_ts, status, created_at, updated_at)
			 VALUES (500, 0, 60, 'pending', 1, 1)`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO batch_screenshots (batch_id, screenshot_id)
			 SELECT 500, MAX(id) FROM screenshots`)
		return err
	}); err != nil {
		t.Fatalf("seed rented batch: %v", err)
	}
	return root, activeFile
}

func writeFileAt(t *testing.T, p string, size int64, at time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
	if err := os.WriteFile(p, make([]byte, size), 0o600); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	if err := os.Chtimes(p, at, at); err != nil {
		t.Fatalf("chtimes %s: %v", p, err)
	}
}

func liveFrameCount(t *testing.T, store *Store) int {
	t.Helper()
	var n int
	if err := store.Read(context.Background(), "count live", func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM screenshots WHERE is_deleted = 0").Scan(&n)
	}); err != nil {
		t.Fatalf("count live: %v", err)
	}
	return n
}

// DB-9: overfill past a small limit; recordings converge and the ACTIVE
// segment is never deleted. docs/08 §8.4.
func TestCleanupConvergesAndKeepsActiveSegment(t *testing.T) {
	store := openWriter(t, newDir(t))
	root, activeFile := seedCleanupWorld(t, store)
	ctx := context.Background()

	// Ten 1000-byte frames = 10000 bytes; the limit keeps the four newest.
	result, err := store.CleanupRecordings(ctx, root, 4000)
	if err != nil {
		t.Fatalf("CleanupRecordings: %v", err)
	}
	if result.Deleted != 6 {
		t.Fatalf("deleted %d frames, want 6 (the six oldest)", result.Deleted)
	}
	if result.FreedBytes != 6000 {
		t.Fatalf("freed %d bytes, want 6000", result.FreedBytes)
	}

	usage, err := store.recordingsUsage(ctx)
	if err != nil {
		t.Fatalf("usage: %v", err)
	}
	if usage != 4000 {
		t.Fatalf("usage = %d after cleanup, want 4000", usage)
	}

	// The six oldest files are gone from disk; the four newest remain.
	for i := 0; i < 6; i++ {
		rel := fmt.Sprintf("staging/frame-%04d.jpg", i)
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("frame %d survived cleanup", i)
		}
	}
	for i := 6; i < 10; i++ {
		rel := fmt.Sprintf("staging/frame-%04d.jpg", i)
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("kept frame %d is missing: %v", i, err)
		}
	}

	// The ACTIVE segment file is untouched.
	if _, err := os.Stat(activeFile); err != nil {
		t.Fatalf("active staging file was deleted: %v", err)
	}

	// Deleted rows are soft-deleted, not gone.
	if got := liveFrameCount(t, store); got != 4 {
		t.Fatalf("live rows = %d, want 4 (soft delete, not hard delete)", got)
	}
}

// The frames an in-flight analysis batch rents are excluded even when the
// limit cannot be reached without them: converging must never force-delete
// work the pipeline is still using.
func TestCleanupKeepsBatchRentedFrames(t *testing.T) {
	store := openWriter(t, newDir(t))
	root, _ := seedCleanupWorld(t, store)
	ctx := context.Background()

	// Rent EVERY live frame with a second batch. A tiny limit cannot be met
	// without them, and they all stay.
	if err := store.Write(ctx, "rent all frames", func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO analysis_batches (id, start_ts, end_ts, status, created_at, updated_at)
			 VALUES (501, 0, 60, 'pending', 1, 1)`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO batch_screenshots (batch_id, screenshot_id)
			 SELECT 501, id FROM screenshots WHERE is_deleted = 0`)
		return err
	}); err != nil {
		t.Fatalf("rent all: %v", err)
	}
	result, err := store.CleanupRecordings(context.Background(), root, 100)
	if err != nil {
		t.Fatalf("CleanupRecordings: %v", err)
	}
	if result.Deleted != 0 {
		t.Fatalf("deleted %d rented frames, want 0", result.Deleted)
	}
	if result.SkippedRented != 10 {
		t.Fatalf("skipped = %d, want all 10", result.SkippedRented)
	}
	usage, _ := store.recordingsUsage(context.Background())
	if usage != 10000 {
		t.Fatalf("usage = %d, want the untouched 10000", usage)
	}
}

// limitBytes <= 0 is the documented "no limit".
func TestCleanupZeroLimitIsNoop(t *testing.T) {
	store := openWriter(t, newDir(t))
	root, _ := seedCleanupWorld(t, store)

	result, err := store.CleanupRecordings(context.Background(), root, 0)
	if err != nil {
		t.Fatalf("CleanupRecordings: %v", err)
	}
	if result.Deleted != 0 {
		t.Fatalf("deleted %d frames under an unlimited setting", result.Deleted)
	}
}

// The sweep recovers a crash between the two phases: rows soft-deleted but
// files still on disk are removed on a later pass, while a young unreferenced
// file (possibly mid-write) survives.
func TestCleanupSweepsOrphansFromCrashedPhase(t *testing.T) {
	store := openWriter(t, newDir(t))
	root, _ := seedCleanupWorld(t, store)
	ctx := context.Background()

	// Simulate the crash: soft-delete the oldest row, leave its file behind.
	if err := store.Write(ctx, "simulate crash", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			"UPDATE screenshots SET is_deleted = 1 WHERE id = (SELECT MIN(id) FROM screenshots)")
		return err
	}); err != nil {
		t.Fatalf("simulate: %v", err)
	}

	// A young unreferenced file must survive the sweep; an old one must go.
	youngRel := "staging/young-orphan.jpg"
	writeFileAt(t, filepath.Join(root, filepath.FromSlash(youngRel)), 10, time.Now())
	oldRel := "staging/old-orphan.jpg"
	writeFileAt(t, filepath.Join(root, filepath.FromSlash(oldRel)), 10,
		time.Now().Add(-2*time.Hour))

	if _, err := store.CleanupRecordings(ctx, root, 4000); err != nil {
		t.Fatalf("CleanupRecordings: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(oldRel))); !os.IsNotExist(err) {
		t.Error("old orphan file survived the sweep")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(youngRel))); err != nil {
		t.Errorf("young file was swept: %v", err)
	}
}

// The sweep must run even when usage is UNDER the byte limit. Reconcile-abandon
// and a failed phase-2 removal leave orphan files while usage stays low, and if
// the sweep only ran on the over-limit path those files would never be
// reclaimed.
func TestCleanupSweepsOrphansWhenUnderLimit(t *testing.T) {
	store := openWriter(t, newDir(t))
	root, _ := seedCleanupWorld(t, store)
	ctx := context.Background()

	// An old orphan file that no row references, created by e.g. Abandon.
	oldRel := "staging/abandoned-orphan.jpg"
	writeFileAt(t, filepath.Join(root, filepath.FromSlash(oldRel)), 10,
		time.Now().Add(-2*time.Hour))

	// Usage is 10000 bytes; the limit is far above it, so the over-limit
	// deletion path never runs — only the unconditional sweep can reclaim.
	result, err := store.CleanupRecordings(ctx, root, 1<<30)
	if err != nil {
		t.Fatalf("CleanupRecordings: %v", err)
	}
	if result.Deleted != 0 {
		t.Fatalf("deleted %d frames while under the limit", result.Deleted)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(oldRel))); !os.IsNotExist(err) {
		t.Error("orphan file survived the sweep under the limit")
	}
	// Live frames are untouched: the sweep only removes unreferenced files.
	if got := liveFrameCount(t, store); got != 10 {
		t.Fatalf("live rows = %d after under-limit cleanup, want 10", got)
	}
}

// A read-only instance never deletes anything: it holds no write lock, so
// another process may be mid-capture.
func TestCleanupRefusedOnReadOnlyInstance(t *testing.T) {
	dir := newDir(t)
	holder := openWriter(t, dir)
	reader := openReader(t, dir)

	_, err := reader.CleanupRecordings(context.Background(), dir, 100)
	if !IsKind(err, KindReadOnly) {
		t.Fatalf("error = %v, want a read-only refusal", err)
	}
	_ = holder
}

func TestCleanupMultiFrameSegmentDeletesAllRowsAndFile(t *testing.T) {
	store := openWriter(t, newDir(t))
	root := filepath.Join(newDir(t), "recordings")
	ctx := context.Background()

	// Segment 1: 3 frames sharing segment-1.mp4, 500 bytes each = 1500 bytes total
	seg1 := "segments/segment-1.mp4"
	writeFileAt(t, filepath.Join(root, filepath.FromSlash(seg1)), 1500, time.Unix(100, 0))
	for f := 0; f < 3; f++ {
		id, err := store.Captures().Begin(ctx, seg1, f, time.Unix(int64(100+f), 0), nil, 1920, 1080, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Captures().Commit(ctx, id, 500); err != nil {
			t.Fatal(err)
		}
	}

	// Segment 2: 2 frames sharing segment-2.mp4, 500 bytes each = 1000 bytes total
	seg2 := "segments/segment-2.mp4"
	writeFileAt(t, filepath.Join(root, filepath.FromSlash(seg2)), 1000, time.Unix(200, 0))
	for f := 0; f < 2; f++ {
		id, err := store.Captures().Begin(ctx, seg2, f, time.Unix(int64(200+f), 0), nil, 1920, 1080, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Captures().Commit(ctx, id, 500); err != nil {
			t.Fatal(err)
		}
	}

	// Limit is 1000 bytes: segment 1 (1500 bytes) must be deleted entirely, keeping segment 2 (1000 bytes)
	res, err := store.CleanupRecordings(ctx, root, 1000)
	if err != nil {
		t.Fatalf("CleanupRecordings: %v", err)
	}
	if res.Deleted != 3 {
		t.Fatalf("res.Deleted = %d, want 3 (all 3 frames of segment 1)", res.Deleted)
	}
	if res.FreedBytes != 1500 {
		t.Fatalf("res.FreedBytes = %d, want 1500", res.FreedBytes)
	}

	// File for segment 1 must be removed
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(seg1))); !os.IsNotExist(err) {
		t.Fatal("segment 1 file was not removed")
	}
	// File for segment 2 must still exist
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(seg2))); err != nil {
		t.Fatal("segment 2 file was removed")
	}
}
