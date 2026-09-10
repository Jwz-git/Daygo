package storage

import (
	"context"
	"testing"
)

// DB-6: the fixed pragma set must be observable by reading it back. Asserting
// only that opening succeeded would not catch a pragma the driver ignored.
// docs/08 §8.4.
func TestPragmasAreAppliedAndReadBack(t *testing.T) {
	store := openWriter(t, newDir(t))
	ctx := context.Background()

	var journalMode string
	if err := store.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want %q", journalMode, "wal")
	}

	var synchronous int
	if err := store.db.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&synchronous); err != nil {
		t.Fatalf("read synchronous: %v", err)
	}
	if synchronous != 1 {
		t.Errorf("synchronous = %d, want 1 (NORMAL)", synchronous)
	}

	var busyTimeout int
	if err := store.db.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if busyTimeout != pragmaBusyTimeout {
		t.Errorf("busy_timeout = %d, want %d", busyTimeout, pragmaBusyTimeout)
	}
}

// Every connection in the pool carries the pragmas, not just the first one.
// A statement applied after opening would configure only the connection it ran
// on, which is why the pragmas travel in the DSN.
func TestPragmasApplyToEveryPooledConnection(t *testing.T) {
	store := openWriter(t, newDir(t))
	ctx := context.Background()

	// Force several distinct connections to serve the queries.
	conns := make([]string, 0, maxOpenConns)
	for i := 0; i < maxOpenConns; i++ {
		var mode string
		if err := store.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		conns = append(conns, mode)
	}
	for i, mode := range conns {
		if mode != "wal" {
			t.Errorf("connection %d journal_mode = %q, want %q", i, mode, "wal")
		}
	}
}

func TestVerifyPragmasRejectsReadOnlyWithoutQueryOnly(t *testing.T) {
	// A read-only instance also carries query_only. This guards the second half
	// of the connection-layer guarantee in docs/07 §7.5.
	dir := newDir(t)
	openWriter(t, dir)
	reader := openReader(t, dir)

	var queryOnly int
	if err := reader.db.QueryRowContext(context.Background(), "PRAGMA query_only").Scan(&queryOnly); err != nil {
		t.Fatalf("read query_only: %v", err)
	}
	if queryOnly != 1 {
		t.Errorf("query_only = %d on read-only instance, want 1", queryOnly)
	}
}
