package storage

import (
	"context"
	"database/sql"
	"testing"
)

// A read-only instance must refuse writes at the connection layer, not merely
// by asking callers to behave. docs/07 §7.5 states the refusal must survive a
// caller that does not cooperate, so this test bypasses Store.Write and issues
// raw SQL directly.
func TestReadOnlyInstanceRefusesWritesAtConnectionLayer(t *testing.T) {
	dir := newDir(t)
	openWriter(t, dir)
	reader := openReader(t, dir)

	_, err := reader.db.ExecContext(context.Background(),
		"CREATE TABLE should_not_exist (id INTEGER PRIMARY KEY)")
	if err == nil {
		t.Fatal("read-only instance accepted a write")
	}
	// The file is intact; only the connection is read-only. Reporting corruption
	// here would send a healthy database down the recovery path (DB-7).
	if IsCorrupt(err) {
		t.Fatalf("write refusal classified as corruption: %v", err)
	}
}

// Store.Write refuses before reaching the driver when the instance is
// read-only, producing a clear cause instead of a driver error.
func TestStoreWriteRefusesOnReadOnlyInstance(t *testing.T) {
	dir := newDir(t)
	openWriter(t, dir)
	reader := openReader(t, dir)

	err := reader.Write(context.Background(), "probe", func(context.Context, *sql.Tx) error {
		t.Fatal("transaction body ran on a read-only instance")
		return nil
	})
	assertKind(t, err, KindReadOnly)
}

// DB-8 smoke: a writer and a reader operating concurrently must not produce
// busy errors or corruption.
//
// The one-hour version of this assertion lives in concurrency_long_test.go
// behind the `long` build tag, because running it on every commit would make
// `go test ./...` unusable. The two are recorded separately and neither
// substitutes for the other (docs/08 §8.4, docs/09 §9.2).
func TestConcurrentReaderWriterSmoke(t *testing.T) {
	dir := newDir(t)
	writer := openWriter(t, dir)
	if err := writer.Write(context.Background(), "create smoke table", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "CREATE TABLE smoke (id INTEGER PRIMARY KEY, note TEXT NOT NULL)")
		return err
	}); err != nil {
		t.Fatalf("create table: %v", err)
	}

	reader := openReader(t, dir)

	const iterations = 50
	writerErr := make(chan error, 1)
	go func() {
		for i := 0; i < iterations; i++ {
			err := writer.Write(context.Background(), "insert smoke", func(ctx context.Context, tx *sql.Tx) error {
				_, execErr := tx.ExecContext(ctx, "INSERT INTO smoke (note) VALUES (?)", "row")
				return execErr
			})
			if err != nil {
				writerErr <- err
				return
			}
		}
		writerErr <- nil
	}()

	for i := 0; i < iterations; i++ {
		var count int
		err := reader.Read(context.Background(), "count smoke", func(ctx context.Context, tx *sql.Tx) error {
			return tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM smoke").Scan(&count)
		})
		if err != nil {
			t.Fatalf("reader iteration %d: %v", i, err)
		}
		if count < 0 {
			t.Fatalf("reader iteration %d: count = %d", i, count)
		}
	}

	if err := <-writerErr; err != nil {
		t.Fatalf("writer: %v", err)
	}
}

// A read transaction must not leave the database locked for the writer, so a
// sequence of reads followed by a write completes without contention.
func TestReadThenWriteDoesNotContend(t *testing.T) {
	dir := newDir(t)
	writer := openWriter(t, dir)
	ctx := context.Background()

	if err := writer.Write(ctx, "create table", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "CREATE TABLE t (id INTEGER PRIMARY KEY)")
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	for i := 0; i < 5; i++ {
		if err := writer.Read(ctx, "read", func(ctx context.Context, tx *sql.Tx) error {
			var n int
			return tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM t").Scan(&n)
		}); err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
	}

	if err := writer.Write(ctx, "insert", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO t DEFAULT VALUES")
		return err
	}); err != nil {
		t.Fatalf("write after reads: %v", err)
	}
}
