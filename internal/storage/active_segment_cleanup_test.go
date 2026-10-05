package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanupRechecksRentalBeforeDeletion(t *testing.T) {
	s := openWriter(t, t.TempDir())
	root, _ := seedCleanupWorld(t, s)
	ctx := context.Background()
	candidates, err := s.cleanupCandidates(ctx)
	if err != nil || len(candidates) == 0 {
		t.Fatalf("candidates=%+v err=%v", candidates, err)
	}
	var id int64
	if err := s.Read(ctx, "fixture frame", func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, "SELECT id FROM screenshots WHERE segment_path=?", candidates[0].segmentPath).Scan(&id)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Analysis().CreateBatch(ctx, []AnalysisFrame{{ID: id, CapturedAt: time.Unix(100, 0)}}, BatchPending, time.Now()); err != nil {
		t.Fatal(err)
	}
	deleted, _, removed, err := s.softDeleteSegments(ctx, candidates[:1])
	if err != nil || deleted != 0 {
		t.Fatalf("rented after selection: deleted=%d err=%v", deleted, err)
	}
	if len(removed) != 0 {
		t.Fatalf("rented paths reached file deletion: %+v", removed)
	}
	if _, err := os.Stat(filepath.Join(root, candidates[0].segmentPath)); err != nil {
		t.Fatal(err)
	}
}

func TestCreateBatchRefusesFramesDeletedSinceDiscovery(t *testing.T) {
	s := openWriter(t, t.TempDir())
	root, _ := seedCleanupWorld(t, s)
	ctx := context.Background()
	var id int64
	if err := s.Read(ctx, "fixture frame", func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, "SELECT MIN(id) FROM screenshots").Scan(&id)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CleanupRecordings(ctx, root, 4000); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Analysis().CreateBatch(ctx, []AnalysisFrame{{ID: id, CapturedAt: time.Unix(100, 0)}}, BatchPending, time.Now()); err == nil {
		t.Fatal("batch rented a deleted frame")
	}
}

func TestCleanupProtectsCommittedSegmentUntilFinalized(t *testing.T) {
	s := openWriter(t, t.TempDir())
	ctx := context.Background()
	root := t.TempDir()
	rel := "segments/anonymous-active.mp4"
	file := filepath.Join(root, filepath.FromSlash(rel))
	writeFileAt(t, file, 1000, time.Unix(100, 0))
	id, err := s.Captures().Begin(ctx, rel, 0, time.Unix(100, 0), nil, 1920, 1080, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Captures().Commit(ctx, id, 1000); err != nil {
		t.Fatal(err)
	}
	// Between discrete captures no intent is pending, but the MP4 remains open.
	for _, failAccounting := range []bool{false, true} {
		if failAccounting {
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			if err := s.Captures().AmortizeSegment(cancelled, rel, 1000); err == nil {
				t.Fatal("cancelled accounting unexpectedly succeeded")
			}
		}
		result, err := s.CleanupRecordings(ctx, root, 500)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(file); err != nil || result.Deleted != 0 || liveFrameCount(t, s) != 1 {
			t.Fatalf("active segment lost: cleanup=%+v stat=%v", result, err)
		}
	}
	if err := s.Captures().AmortizeSegment(ctx, rel, 1000); err != nil {
		t.Fatal(err)
	}
	result, err := s.CleanupRecordings(ctx, root, 500)
	if err != nil || result.Deleted != 1 {
		t.Fatalf("finalized cleanup=%+v err=%v", result, err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("finalized segment was not deleted: %v", err)
	}
}

func TestOrphanSweepProtectsOpenSegmentAfterAbandonedFrame(t *testing.T) {
	s := openWriter(t, t.TempDir())
	ctx := context.Background()
	root := t.TempDir()
	rel := "segments/anonymous-abandoned.mp4"
	file := filepath.Join(root, filepath.FromSlash(rel))
	writeFileAt(t, file, 1000, time.Unix(100, 0))
	id, err := s.Captures().Begin(ctx, rel, 0, time.Unix(100, 0), nil, 1920, 1080, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Captures().Abandon(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CleanupRecordings(ctx, root, 500); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("open orphan segment was removed: %v", err)
	}
	if err := s.Captures().AmortizeSegment(ctx, rel, 1000); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CleanupRecordings(ctx, root, 500); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("closed orphan was not swept: %v", err)
	}
}
