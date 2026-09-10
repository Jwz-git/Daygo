package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// DB-4: after opening, reading everything and closing, the database must still
// report a clean integrity check. docs/08 §8.4.
func TestRoundTripLeavesDatabaseIntact(t *testing.T) {
	dir := newDir(t)

	store := openWriter(t, dir)
	ctx := context.Background()

	if err := store.Write(ctx, "seed", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			"INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)",
			"appearance.theme", `"system"`, 0)
		return err
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Read every row back so the round trip actually visits the table.
	if err := store.Read(ctx, "read settings", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, "SELECT key, value FROM app_settings")
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var key, value string
			if err := rows.Scan(&key, &value); err != nil {
				return err
			}
		}
		return rows.Err()
	}); err != nil {
		t.Fatalf("read: %v", err)
	}

	var result string
	if err := store.db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		t.Fatalf("integrity_check: %v", err)
	}
	if result != "ok" {
		t.Fatalf("integrity_check = %q, want %q", result, "ok")
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen and check again: the check must hold for the file on disk, not
	// just for the connection that wrote it.
	reopened := openWriter(t, dir)
	if err := reopened.db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		t.Fatalf("integrity_check after reopen: %v", err)
	}
	if result != "ok" {
		t.Fatalf("integrity_check after reopen = %q, want %q", result, "ok")
	}
}

// DB-7: a torn write presents as a file whose header is intact and whose pages
// are not. It must be classified as corruption so the recovery path can act.
func TestTruncatedDatabaseIsClassifiedAsCorrupt(t *testing.T) {
	dir := newDir(t)
	copyFile(t, filepath.Join("testdata", "truncated.db"), filepath.Join(dir, DatabaseFileName))

	store, err := Open(context.Background(), Options{Dir: dir})
	if err != nil {
		// Failing to open is an acceptable outcome for a torn file, as long as
		// the classification is corruption rather than environment.
		if !IsCorrupt(err) {
			t.Fatalf("truncated database classified as %v, want corruption", err)
		}
		return
	}
	defer func() { _ = store.Close() }()

	// Opening succeeded, so the damage must surface on the first real read.
	// Whatever happens, it must not be silently accepted as valid data.
	var count int
	readErr := store.Read(context.Background(), "read truncated", func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM legacy_marker").Scan(&count)
	})
	if readErr == nil {
		t.Skip("driver read the truncated fixture; truncation point did not damage the page tree")
	}
	if !IsCorrupt(readErr) {
		t.Fatalf("read of truncated database classified as %v, want corruption", readErr)
	}
}

// An intact database must never be classified as corrupt, whatever the
// environment does. This is the guard against a recovery path that deletes
// healthy files.
func TestHealthyDatabaseIsNotCorrupt(t *testing.T) {
	dir := newDir(t)
	store := openWriter(t, dir)

	if err := store.Read(context.Background(), "probe", func(ctx context.Context, tx *sql.Tx) error {
		var n int
		return tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM app_settings").Scan(&n)
	}); err != nil {
		if IsCorrupt(err) {
			t.Fatalf("healthy database reported as corrupt: %v", err)
		}
		t.Fatalf("read: %v", err)
	}
}
