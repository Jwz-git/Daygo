package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/Jwz-git/Daygo/internal/recordinglocation"
	"github.com/Jwz-git/Daygo/internal/storage"
)

func TestRecordingRootCacheTracksDirectoryCommits(t *testing.T) {
	ctx := t.Context()
	backend, _ := writerBackendWithStore(t, t.TempDir())
	defaultRoot := filepath.Join(filepath.Dir(backend.store().Path()), "recordings")
	cache := &recordingRootCache{}
	reader, err := storage.OpenReadOnly(ctx, backend.store().Path(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()

	resolve := func(cache *recordingRootCache, store *storage.Store, want string) {
		t.Helper()
		if root, err := cache.resolve(ctx, store, defaultRoot); err != nil || root != want {
			t.Fatalf("recording root = %q, %v, want %q", root, err, want)
		}
	}
	resolve(cache, backend.store(), defaultRoot)
	first := cache.cached.Load()
	resolve(cache, backend.store(), defaultRoot)
	if first == nil || cache.cached.Load() != first {
		t.Fatal("writer should reuse a resolved root until a settings commit")
	}
	readOnlyCache := &recordingRootCache{}
	resolve(readOnlyCache, reader, defaultRoot)
	destination := t.TempDir()
	value, err := json.Marshal(destination)
	if err != nil {
		t.Fatal(err)
	}
	repo := backend.store().Settings()
	if err := repo.SetMany(ctx, map[string]string{
		recordinglocation.DirectoryKey: string(value), recordinglocation.MigrationKey: `{"phase":"committed"}`,
	}); err != nil {
		t.Fatal(err)
	}
	resolve(cache, backend.store(), destination)
	resolve(readOnlyCache, reader, destination)
	if readOnlyCache.cached.Load() != nil {
		t.Fatal("read-only root cache would hide another process's migration")
	}
	if err := repo.Set(ctx, recordinglocation.DirectoryKey, `"relative"`); err != nil {
		t.Fatal(err)
	}
	if root, err := cache.resolve(ctx, backend.store(), defaultRoot); err == nil || root != "" {
		t.Fatalf("malformed new setting resolved to stale root %q, %v", root, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := cache.resolve(canceled, backend.store(), defaultRoot); err == nil {
		t.Fatal("cancellation should be observed")
	}
}
