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
