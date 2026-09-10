package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// ErrPragmaMismatch reports that a fixed pragma did not take effect.
var ErrPragmaMismatch = errors.New("storage: pragma mismatch")

// migration is one step in the versioned schema chain (docs/03 §3.3).
//
// INVARIANT, enforced by review and by migrate_test.go: every entry added to
// `migrations` must arrive in the same commit as an "old database to new
// database" fixture under testdata/ (docs/05 §5.6.2 rule 3). A version without
// a fixture is not mergeable, because the only way to prove a migration
// preserves data is to run it against a database written by the previous
// version.
//
// Migrations are append-only. An already-released version must never be edited:
// users' databases are already at that version, so an edit would be applied to
// no one and would desynchronize the chain from what those databases contain.
type migration struct {
	version int
	// name is diagnostic only; it appears in breadcrumbs and error messages.
	name string
	// apply runs inside a transaction. Returning an error rolls the whole
	// migration back, leaving user_version unchanged.
	apply func(ctx context.Context, tx *sql.Tx) error
}

// migrations is the ordered chain.
//
// v1 creates app_settings only. It is the one table db-core must own: settings
// live in the database rather than a plist so they can change in the same
// transaction as the data they affect (docs/03 §3.1), and docs/modules/data.md
// makes this table data's deliverable. Typed access over it belongs to the
// settings-store capability and is not part of this version.
//
// Feature modules append their own tables here through the same chain, one
// version per change (docs/09 §9.3). db-core deliberately does not pre-create
// tables for features that do not exist yet: that would freeze a schema before
// its design is settled.
var migrations = []migration{
	{
		version: 1,
		name:    "app_settings",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, `
				CREATE TABLE app_settings (
					key        TEXT PRIMARY KEY,
					value      TEXT NOT NULL,
					updated_at INTEGER NOT NULL
				)`); err != nil {
				return wrap("create app_settings", err)
			}
			return nil
		},
	},
}

// schemaVersion is the version this build expects after migrating.
func schemaVersion() int {
	if len(migrations) == 0 {
		return 0
	}
	return migrations[len(migrations)-1].version
}

// migrate brings the database up to schemaVersion. It runs only on the writer
// instance; a read-only instance skipped it in connect.
//
// Each step is its own transaction so a failure part-way leaves the database at
// the last fully applied version rather than in a half-migrated state. The
// chain is forward-only by design: docs/modules/data.md states that an actual
// schema upgrade cannot be rolled back with git revert, so there is no down
// path to get wrong.
func (s *Store) migrate(ctx context.Context) error {
	current, err := s.userVersion(ctx)
	if err != nil {
		return err
	}

	target := schemaVersion()
	if current > target {
		// The database was written by a newer build. Refusing is the only safe
		// answer: writing would corrupt data this build cannot interpret.
		return &Error{
			Kind: KindEnvironment,
			Op:   fmt.Sprintf("migrate: database is at version %d, this build supports %d", current, target),
		}
	}
	if current == target {
		return nil
	}

	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		if m.version != current+1 {
			return &Error{
				Kind: KindEnvironment,
				Op:   fmt.Sprintf("migrate: chain gap, have %d, next is %d", current, m.version),
			}
		}
		if err := s.applyMigration(ctx, m); err != nil {
			return err
		}
		current = m.version
		s.observeBreadcrumb(breadcrumbMigrated)
	}
	return nil
}

// applyMigration runs one migration and records its version in the same
// transaction. Committing the schema change and the version bump together is
// what makes a re-run after a crash safe: either both landed or neither did, so
// migration is idempotent by construction (DB-1).
func (s *Store) applyMigration(ctx context.Context, m migration) error {
	start := time.Now()
	err := s.Write(ctx, fmt.Sprintf("migrate v%d (%s)", m.version, m.name), func(ctx context.Context, tx *sql.Tx) error {
		if err := m.apply(ctx, tx); err != nil {
			return err
		}
		// PRAGMA user_version does not accept a bound parameter, and the value
		// is an integer from this package's own chain, never user input. The
		// formatting is therefore not an injection surface.
		return execRaw(ctx, tx, fmt.Sprintf("PRAGMA user_version = %d", m.version))
	})
	s.observeQuery("migrate", start, err)
	return err
}

// userVersion reads PRAGMA user_version.
func (s *Store) userVersion(ctx context.Context) (int, error) {
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, wrap("read user_version", err)
	}
	return version, nil
}

// execRaw runs a statement that cannot take bound parameters, such as a PRAGMA
// assignment.
func execRaw(ctx context.Context, tx *sql.Tx, statement string) error {
	if _, err := tx.ExecContext(ctx, statement); err != nil {
		return wrap("exec", err)
	}
	return nil
}

// IsCorrupt reports whether err indicates the database image is unusable and
// the open-and-recover path is warranted. Environment failures deliberately
// return false: recovering would delete an intact file (DB-7, docs/05 §5.6.2
// rule 5).
//
// The recovery action itself belongs to the maintenance slice; this predicate
// exists now so its callers cannot invent a second, laxer definition of
// "corrupt".
func IsCorrupt(err error) bool {
	kind, ok := KindOf(err)
	return ok && kind.Recoverable()
}

// classifyOpenFailure decides whether a failed open is corruption. It exists so
// the distinction is made once, in this package, rather than re-derived by each
// caller that opens a database.
func classifyOpenFailure(err error) error {
	if err == nil {
		return nil
	}
	var se *sqlite.Error
	if errors.As(err, &se) && se != nil {
		switch se.Code() & 0xff {
		case sqlite3.SQLITE_CORRUPT, sqlite3.SQLITE_NOTADB:
			return &Error{Kind: KindCorrupt, Op: "open", err: err}
		}
	}
	return wrap("open", err)
}
