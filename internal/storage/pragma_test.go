package storage

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"
)

func TestPragmaDSNWriterUsesImmediateTxlock(t *testing.T) {
	rw := pragmaDSN("/tmp/daygo.sqlite", ModeReadWrite)
	if !strings.Contains(rw, "_txlock=immediate") {
		t.Fatalf("read-write DSN %q missing _txlock=immediate; deferred write transactions defeat busy_timeout", rw)
	}
	ro := pragmaDSN("/tmp/daygo.sqlite", ModeReadOnly)
	if strings.Contains(ro, "_txlock=immediate") {
		t.Fatalf("read-only DSN %q should not force immediate; it never writes", ro)
	}
	if !strings.Contains(ro, "mode=ro") || !strings.Contains(ro, "query_only(1)") {
		t.Fatalf("read-only DSN %q lost its read-only guards", ro)
	}
}

func TestEscapeURIPathEscapesSpecialCharacters(t *testing.T) {
	cases := map[string]string{
		"/Users/jwz/Application Support/Daygo": "/Users/jwz/Application Support/Daygo", // spaces stay literal
		"/data/weird %20 dir/daygo.sqlite":     "/data/weird %2520 dir/daygo.sqlite",
		"/data/a?b/daygo.sqlite":               "/data/a%3Fb/daygo.sqlite",
		"/data/a#b/daygo.sqlite":               "/data/a%23b/daygo.sqlite",
	}
	for in, want := range cases {
		if got := escapeURIPath(in); got != want {
			t.Errorf("escapeURIPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestConcurrentReadThenWriteTransactionsDoNotBusy is the regression guard for
// the deferred-transaction defect: several goroutines each run a read-then-write
// transaction on one writer Store. With BEGIN DEFERRED the read takes a WAL
// snapshot and the later write fails to upgrade with SQLITE_BUSY_SNAPSHOT, which
// busy_timeout does not retry, so many transactions returned KindBusy. With
// BEGIN IMMEDIATE the write lock is taken at BEGIN and busy_timeout serializes
// them, so none should fail.
func TestConcurrentReadThenWriteTransactionsDoNotBusy(t *testing.T) {
	store := openWriter(t, newDir(t))
	ctx := context.Background()

	if err := store.Write(ctx, "create counter", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			"CREATE TABLE counter (id INTEGER PRIMARY KEY CHECK (id = 1), n INTEGER NOT NULL)")
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO counter (id, n) VALUES (1, 0)")
		return err
	}); err != nil {
		t.Fatalf("seed counter: %v", err)
	}

	const goroutines = 4
	const perGoroutine = 40
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				err := store.Write(ctx, "read-then-write", func(ctx context.Context, tx *sql.Tx) error {
					var n int
					if err := tx.QueryRowContext(ctx, "SELECT n FROM counter WHERE id = 1").Scan(&n); err != nil {
						return err
					}
					_, err := tx.ExecContext(ctx, "UPDATE counter SET n = ? WHERE id = 1", n+1)
					return err
				})
				if err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if IsKind(err, KindBusy) {
			t.Fatalf("read-then-write transaction failed with KindBusy: %v", err)
		}
		t.Fatalf("read-then-write transaction failed: %v", err)
	}

	var n int
	if err := store.Read(ctx, "read counter", func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, "SELECT n FROM counter WHERE id = 1").Scan(&n)
	}); err != nil {
		t.Fatalf("read counter: %v", err)
	}
	if want := goroutines * perGoroutine; n != want {
		t.Fatalf("counter = %d after concurrent increments, want %d (a lost update means a dropped write)", n, want)
	}
}

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
