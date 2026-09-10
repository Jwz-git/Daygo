package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"

	// The pure-Go SQLite driver. It is imported here, in the one package that
	// owns SQL, so no other package can reach SQLite (docs/05 §5.6.2 rule 1).
	// Being pure Go is what keeps CGO_ENABLED=0 viable (docs/08 §8.8).
	_ "modernc.org/sqlite"
)

// File names inside the application support directory (docs/03 §3.1). The lock
// files are siblings of the database rather than entries in a table so a
// read-only instance, which cannot write SQL, can still observe contention.
const (
	DatabaseFileName = "daygo.sqlite"
	writeLockName    = "daygo.sqlite.lock"
	ownerLockName    = "capture.lock"
)

// maxOpenConns bounds the pool. SQLite has a single writer, so a large pool
// would only add contention; the value leaves room for concurrent reads under
// WAL while keeping the writer path predictable.
const maxOpenConns = 4

// Options configures Open. Dir is the only required field.
type Options struct {
	// Dir is the application support directory holding the database and lock
	// files. Open creates it when missing.
	Dir string
	// Observer receives diagnostics. Nil means NopObserver.
	Observer Observer
	// CaptureOwnerRequested asks for the capture-owner lock. The caller decides
	// whether this process should capture; Open only reports whether it got it.
	CaptureOwnerRequested bool
}

// Open opens the business database, taking the instance locks that decide the
// mode.
//
// Order matters and is deliberate:
//
//  1. Take the write lock. Holding it means this is the single writer, so the
//     database may be opened read-write and migrated.
//  2. Failing that, open read-only. A second instance must never write, and
//     must never migrate: a newer database version belongs to a process this
//     build does not understand (docs/03 §3.3).
//  3. Verify the pragmas actually took effect before returning, so a caller
//     never operates on a connection whose durability settings were ignored.
//
// The capture-owner lock is taken after the database is usable, and losing it
// is not an error: this process still reads, it just must not drive capture.
func Open(ctx context.Context, opts Options) (*Store, error) {
	if opts.Dir == "" {
		return nil, errors.New("storage: Open: Dir is required")
	}
	observer := opts.Observer
	if observer == nil {
		observer = NopObserver{}
	}

	if err := os.MkdirAll(opts.Dir, 0o700); err != nil {
		return nil, wrap("create application support directory", err)
	}

	path := filepath.Join(opts.Dir, DatabaseFileName)
	store := &Store{path: path, observer: observer}

	writeLk, err := tryLock(filepath.Join(opts.Dir, writeLockName))
	switch {
	case err == nil:
		store.mode = ModeReadWrite
		store.writeLk = writeLk
	case errors.Is(err, ErrLockBusy):
		store.mode = ModeReadOnly
		store.observeBreadcrumb(breadcrumbDegradedOpen)
	default:
		return nil, wrap("acquire write lock", err)
	}

	if err := store.connect(ctx); err != nil {
		// connect owns the descriptors it opened, but the locks taken above
		// belong to this function, so releasing them is this function's job.
		_ = store.Close()
		return nil, err
	}

	if opts.CaptureOwnerRequested && store.mode == ModeReadWrite {
		ownerLk, err := tryLock(filepath.Join(opts.Dir, ownerLockName))
		switch {
		case err == nil:
			store.ownerLk = ownerLk
			store.owner = true
		case errors.Is(err, ErrLockBusy):
			// Another process captures. This instance is a reader for capture
			// purposes only; docs/05 §5.5.1 still lets its write methods run.
			store.observeBreadcrumb(breadcrumbLockBusy)
		default:
			_ = store.Close()
			return nil, wrap("acquire capture-owner lock", err)
		}
	}

	return store, nil
}

// connect opens the SQL driver, configures the pragmas, verifies them and, for
// the writer, brings the schema up to date.
//
// A failure that classification identifies as corruption is retried once after
// running the integrity check, because SQLite can report SQLITE_NOTADB on the
// first statement against a file it has not yet inspected. An environment
// failure is never retried and never repaired: a valid database on a full disk
// must stay untouched (DB-7).
func (s *Store) connect(ctx context.Context) error {
	ctx, cancel := withDefaultTimeout(ctx, writeTimeout)
	defer cancel()

	db, err := sql.Open("sqlite", pragmaDSN(s.path, s.mode))
	if err != nil {
		return classifyOpenFailure(err)
	}
	// A single writer means a large pool buys nothing and spreads WAL
	// contention across more descriptors. The bound also makes "no busy-lock
	// storms" (DB-8) a property of the configuration rather than of luck.
	db.SetMaxOpenConns(maxOpenConns)
	s.db = db

	if err := verifyPragmas(ctx, db, s.mode); err != nil {
		return classifyOpenFailure(err)
	}
	if s.mode == ModeReadOnly {
		// Migrations are the writer's job. A read-only instance must not even
		// consider them: the file may be a newer version than this build.
		return nil
	}
	if err := s.migrate(ctx); err != nil {
		return classifyOpenFailure(err)
	}
	return nil
}
