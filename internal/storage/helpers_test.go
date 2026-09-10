package storage

import (
	"context"
	"testing"
)

// newDir returns an isolated directory for one test. Every storage test runs
// against its own directory: docs/modules/data.md requires isolated test
// directories, and the instance locks mean two tests sharing a directory would
// contend with each other.
func newDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// openWriter opens a read-write instance and closes it when the test ends.
func openWriter(t *testing.T, dir string) *Store {
	t.Helper()
	store, err := Open(context.Background(), Options{Dir: dir})
	if err != nil {
		t.Fatalf("Open(%s) writer: %v", dir, err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if store.Mode() != ModeReadWrite {
		t.Fatalf("Mode() = %q, want %q", store.Mode(), ModeReadWrite)
	}
	return store
}

// openReader opens a second instance that must degrade to read-only because the
// write lock is already held.
func openReader(t *testing.T, dir string) *Store {
	t.Helper()
	store, err := Open(context.Background(), Options{Dir: dir})
	if err != nil {
		t.Fatalf("Open(%s) reader: %v", dir, err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if store.Mode() != ModeReadOnly {
		t.Fatalf("Mode() = %q, want %q", store.Mode(), ModeReadOnly)
	}
	return store
}

// assertKind fails unless err carries the expected storage kind.
func assertKind(t *testing.T, err error, want Kind) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error of kind %q, got nil", want)
	}
	got, ok := KindOf(err)
	if !ok {
		t.Fatalf("error %v is not a storage error", err)
	}
	if got != want {
		t.Fatalf("kind = %q, want %q (err: %v)", got, want, err)
	}
}

// rowCount reports how many rows a table holds. It opens no transaction of its
// own, so it can run while the store is mid-test without contending.
func rowCount(t *testing.T, store *Store, table string) int {
	t.Helper()
	var count int
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}
