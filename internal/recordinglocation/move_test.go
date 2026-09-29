package recordinglocation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type memoryRepo map[string]string

func (r memoryRepo) Get(_ context.Context, k string) (string, bool, error) {
	v, ok := r[k]
	return v, ok, nil
}
func (r memoryRepo) SetMany(_ context.Context, values map[string]string) error {
	for k, v := range values {
		r[k] = v
	}
	return nil
}
func (r memoryRepo) Delete(_ context.Context, k string) error { delete(r, k); return nil }

func TestMovePreservesSourceUntilCommitAndSupportsRetry(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	source := filepath.Join(base, "old")
	target := filepath.Join(base, "new")
	if err := os.MkdirAll(filepath.Join(source, "segments"), 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(source, "segments", "a.mp4")
	if err := os.WriteFile(file, []byte("anonymous fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	repo := memoryRepo{}
	if err := Move(ctx, repo, source, target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("source removed before cleanup: %v", err)
	}
	root, err := Active(ctx, repo, source)
	if err != nil || root != target {
		t.Fatalf("active = %q, %v", root, err)
	}
	if err := Finish(ctx, repo); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("source remains after cleanup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "segments")); !os.IsNotExist(err) {
		t.Fatalf("old segment directory remains after cleanup: %v", err)
	}
	if info, err := os.Stat(source); err != nil || !info.IsDir() {
		t.Fatalf("old recording root missing: %v", err)
	}
	if _, _, phase, err := Pending(ctx, repo); err != nil || phase != "" {
		t.Fatalf("migration remains: %s %v", phase, err)
	}
}

func TestMoveFailedCopyKeepsOldRoot(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	source := filepath.Join(base, "old")
	target := filepath.Join(base, "new")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "a"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	repo := memoryRepo{}
	broken, cancel := context.WithCancel(ctx)
	cancel()
	if err := Move(broken, repo, source, target); err == nil {
		t.Fatal("canceled migration succeeded")
	}
	root, err := Active(ctx, repo, source)
	if err != nil || root != source {
		t.Fatalf("active after failure = %q, %v", root, err)
	}
	if err := Move(ctx, repo, source, target); err != nil {
		t.Fatalf("retry: %v", err)
	}
}

func TestMoveRejectsNestedAndOccupiedTarget(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	source := filepath.Join(base, "old")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	repo := memoryRepo{}
	if err := Move(ctx, repo, source, filepath.Join(source, "child")); err == nil {
		t.Fatal("nested target accepted")
	}
	target := filepath.Join(base, "new")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "existing"), []byte("user data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Move(ctx, repo, source, target); err == nil {
		t.Fatal("occupied target accepted")
	}
}

func TestMoveRetryRejectsLinkedTargetDirectory(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	source := filepath.Join(base, "old")
	target := filepath.Join(base, "new")
	outside := filepath.Join(base, "outside")
	for _, dir := range []string{filepath.Join(source, "segments"), target, outside} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "segments", "a.mp4"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(target, "segments")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	state, err := json.Marshal(migration{Source: source, Target: target, Phase: "copying"})
	if err != nil {
		t.Fatal(err)
	}
	repo := memoryRepo{MigrationKey: string(state)}
	if err := Move(ctx, repo, source, target); err == nil {
		t.Fatal("linked target directory accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "a.mp4")); !os.IsNotExist(err) {
		t.Fatalf("outside directory changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "segments", "a.mp4")); err != nil {
		t.Fatalf("source changed: %v", err)
	}
}

func TestFinishRejectsLinkedTargetDirectory(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	source := filepath.Join(base, "old")
	target := filepath.Join(base, "new")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(filepath.Join(source, "segments"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(source, "segments", "a.mp4")
	if err := os.WriteFile(file, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	repo := memoryRepo{}
	if err := Move(ctx, repo, source, target); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "a.mp4"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(target, "segments"), filepath.Join(target, "original-segments")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(target, "segments")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if err := Finish(ctx, repo); err == nil {
		t.Fatal("cleanup accepted linked target directory")
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("source was removed: %v", err)
	}
}

func TestFinishKeepsSourceWhenTargetDirectoryIsReplaced(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	source := filepath.Join(base, "old")
	target := filepath.Join(base, "new")
	if err := os.MkdirAll(filepath.Join(source, "segments"), 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(source, "segments", "a.mp4")
	if err := os.WriteFile(file, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	repo := memoryRepo{}
	if err := Move(ctx, repo, source, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(target, "segments"), filepath.Join(target, "original-segments")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "segments"), []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Finish(ctx, repo); err == nil {
		t.Fatal("cleanup accepted a replaced target directory")
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("source was removed: %v", err)
	}
}
