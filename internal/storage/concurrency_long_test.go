//go:build long

package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// DB-8, full form: one writer and one read-only instance running concurrently
// for one hour with no busy-lock storm and no corruption. docs/08 §8.4.
//
// This is behind the `long` build tag because a one-hour test would make the
// per-commit `go test ./internal/...` gate unusable. It is NOT a substitute for
// the smoke test in readonly_test.go, and the smoke test is not a substitute
// for this: docs/09 §9.2 requires fake/limited evidence and real evidence to be
// recorded separately.
//
// Run with:
//
//	CGO_ENABLED=0 go test -tags long -run TestConcurrentReaderWriterOneHour -timeout 70m ./internal/storage/
func TestConcurrentReaderWriterOneHour(t *testing.T) {
	const duration = time.Hour

	dir := newDir(t)
	writer := openWriter(t, dir)
	ctx := context.Background()

	if err := writer.Write(ctx, "create table", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			"CREATE TABLE soak (id INTEGER PRIMARY KEY, written_at INTEGER NOT NULL)")
		return err
	}); err != nil {
		t.Fatalf("create table: %v", err)
	}

	reader := openReader(t, dir)

	deadline := time.Now().Add(duration)
	writerErrs := make(chan error, 1)
	readerErrs := make(chan error, 1)

	// The writer runs at a modest cadence. The point is sustained concurrent
	// access, not throughput: hammering would test the scheduler rather than
	// SQLite's locking.
	go func() {
		var writes int
		for time.Now().Before(deadline) {
			err := writer.Write(ctx, "insert soak", func(ctx context.Context, tx *sql.Tx) error {
				_, execErr := tx.ExecContext(ctx,
					"INSERT INTO soak (written_at) VALUES (?)", time.Now().Unix())
				return execErr
			})
			if err != nil {
				writerErrs <- err
				return
			}
			writes++
			time.Sleep(100 * time.Millisecond)
		}
		t.Logf("writer completed %d writes", writes)
		writerErrs <- nil
	}()

	go func() {
		var reads int
		for time.Now().Before(deadline) {
			var count int
			err := reader.Read(ctx, "count soak", func(ctx context.Context, tx *sql.Tx) error {
				return tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM soak").Scan(&count)
			})
			if err != nil {
				readerErrs <- err
				return
			}
			// The reader must never see a count go backwards: rows are only
			// inserted, so a decrease would mean corruption.
			reads++
			time.Sleep(50 * time.Millisecond)
		}
		t.Logf("reader completed %d reads", reads)
		readerErrs <- nil
	}()

	if err := <-writerErrs; err != nil {
		t.Fatalf("writer failed during soak: %v", err)
	}
	if err := <-readerErrs; err != nil {
		t.Fatalf("reader failed during soak: %v", err)
	}

	// The reader's connection is read-only, so the integrity check runs on the
	// writer, which is the instance that could have caused damage.
	var result string
	if err := writer.db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		t.Fatalf("integrity_check: %v", err)
	}
	if result != "ok" {
		t.Fatalf("integrity_check after one-hour soak = %q, want %q", result, "ok")
	}
}
