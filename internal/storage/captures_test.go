package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCaptureCommitIsIdempotent(t *testing.T) {
	dir := newDir(t)
	store := openWriter(t, dir)
	repo := store.Captures()
	id, err := repo.Begin(context.Background(), "staging/frame.jpg", time.Unix(100, 0), nil, 16, 9, false)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "recordings", "staging", "frame.jpg")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("jpeg"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := repo.Commit(context.Background(), id, 4); err != nil {
		t.Fatal(err)
	}
	if err := repo.Commit(context.Background(), id, 4); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM screenshots WHERE segment_path=?", "staging/frame.jpg").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("screenshot rows=%d, want 1", count)
	}
}

// commitFrame plants a committed screenshot row plus a plausible file so
// frame-listing tests have data at a known timestamp.
func commitFrame(t *testing.T, dir string, repo *CaptureRepo, capturedAt int64) int64 {
	t.Helper()
	rel := "staging/frame-" + time.Unix(capturedAt, 0).UTC().Format("20060102-150405") + ".jpg"
	id, err := repo.Begin(context.Background(), rel, time.Unix(capturedAt, 0), nil, 16, 9, false)
	if err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(dir, "recordings", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte("jpeg"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := repo.Commit(context.Background(), id, 4); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestFramesInRangeSamplesEvenly(t *testing.T) {
	dir := newDir(t)
	store := openWriter(t, dir)
	repo := store.Captures()
	// Nine frames ten seconds apart; a limit of 3 must sample every third.
	for index := 0; index < 9; index++ {
		commitFrame(t, dir, repo, int64(1000+index*10))
	}
	frames, err := repo.FramesInRange(context.Background(), 0, 2000, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 3 {
		t.Fatalf("frames=%d, want 3", len(frames))
	}
	for i, frame := range frames {
		if want := int64(1000 + i*30); frame.CapturedAt != want {
			t.Fatalf("frame[%d].capturedAt=%d, want %d", i, frame.CapturedAt, want)
		}
	}
}

func TestFramesInRangeRespectsBoundsAndDeletion(t *testing.T) {
	dir := newDir(t)
	store := openWriter(t, dir)
	repo := store.Captures()
	inside := commitFrame(t, dir, repo, 1500)
	before := commitFrame(t, dir, repo, 900)
	after := commitFrame(t, dir, repo, 2500)

	frames, err := repo.FramesInRange(context.Background(), 1000, 2000, 600)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 1 || frames[0].ID != inside {
		t.Fatalf("frames=%+v, want only %d", frames, inside)
	}

	if _, err := store.db.Exec(`UPDATE screenshots SET is_deleted = 1 WHERE id = ?`, inside); err != nil {
		t.Fatal(err)
	}
	frames, err = repo.FramesInRange(context.Background(), 1000, 2000, 600)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 0 {
		t.Fatalf("deleted frame still listed: %+v", frames)
	}
	_ = before
	_ = after
}

func TestFramePathResolvesAndRejectsDeleted(t *testing.T) {
	dir := newDir(t)
	store := openWriter(t, dir)
	repo := store.Captures()
	id := commitFrame(t, dir, repo, 1000)

	got, err := repo.FramePath(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if want := "staging/frame-" + time.Unix(1000, 0).UTC().Format("20060102-150405") + ".jpg"; got != want {
		t.Fatalf("frame path=%q, want %q", got, want)
	}

	if _, err := store.db.Exec(`UPDATE screenshots SET is_deleted = 1 WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FramePath(context.Background(), id); err == nil {
		t.Fatal("deleted frame resolved a path; want error")
	}
}
