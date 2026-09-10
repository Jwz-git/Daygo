package storage

import (
	"database/sql"
	"errors"
	"fmt"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Kind classifies a storage failure. The binding layer maps these onto the
// closed application error-code set in docs/05 §5.4.1; classification lives
// here so that mapping is written once rather than scattered across callers.
type Kind string

const (
	// KindBusy is transient lock contention. Retryable; maps to conflict.
	KindBusy Kind = "busy"
	// KindCorrupt means the database image itself is unusable. This is the ONLY
	// kind that may trigger the open-and-recover path (docs/05 §5.6.2 rule 5).
	KindCorrupt Kind = "corrupt"
	// KindReadOnly means a write was attempted on a read-only instance. The
	// connection layer refused it, which is the intended behavior (docs/07 §7.5).
	KindReadOnly Kind = "read_only"
	// KindEnvironment covers disk-full, permission and I/O failures reported by
	// the OS. A valid database in a hostile environment must NOT be treated as
	// corrupt: these warn and never delete files (DB-7).
	KindEnvironment Kind = "environment"
	// KindConstraint is a constraint violation.
	KindConstraint Kind = "constraint"
	// KindNotFound means the requested row does not exist.
	KindNotFound Kind = "not_found"
)

// Valid reports whether k belongs to the closed kind set.
func (k Kind) Valid() bool {
	switch k {
	case KindBusy, KindCorrupt, KindReadOnly, KindEnvironment, KindConstraint, KindNotFound:
		return true
	default:
		return false
	}
}

// Retryable reports whether the caller may retry the same operation.
func (k Kind) Retryable() bool { return k == KindBusy }

// Recoverable reports whether this failure may trigger the open-and-recover
// path. Only corruption qualifies. Treating an environment failure as
// recoverable would destroy an intact database file, which DB-7 forbids.
func (k Kind) Recoverable() bool { return k == KindCorrupt }

// Error is a classified storage failure. Op names the operation and subject
// (for example "open daygo.sqlite"); the cause stays in the unwrap chain so
// callers can use errors.Is/As and never string matching (docs/05 §5.6.1).
type Error struct {
	Kind Kind
	Op   string
	err  error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.err == nil {
		return fmt.Sprintf("storage: %s: %s", e.Op, e.Kind)
	}
	return fmt.Sprintf("storage: %s: %s: %v", e.Op, e.Kind, e.err)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// wrap classifies err and attaches operation context. It returns nil when err
// is nil so callers can wrap unconditionally.
func wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Kind: classify(err), Op: op, err: err}
}

// newError builds a classified error with no underlying cause, for failures
// this package originates itself (refusing a write, declining to open).
func newError(kind Kind, op string) error {
	return &Error{Kind: kind, Op: op}
}

// KindOf reports the kind of err, or false when err is not a storage error.
func KindOf(err error) (Kind, bool) {
	var se *Error
	if errors.As(err, &se) && se != nil {
		return se.Kind, true
	}
	return "", false
}

// IsKind reports whether err carries the given storage kind. It unwraps, so an
// error returned by a repository method may carry a classification added at a
// lower layer.
func IsKind(err error, kind Kind) bool {
	k, ok := KindOf(err)
	return ok && k == kind
}

// classify maps a driver or database error onto a Kind. It is the single place
// where SQLite result codes are interpreted.
//
// SQLite result codes carry extended information in the high bits (for example
// SQLITE_IOERR_WRITE is SQLITE_IOERR | (3 << 8)), so the primary code is the
// low byte. Failing to mask would misclassify every extended code.
func classify(err error) Kind {
	if err == nil {
		return ""
	}
	if errors.Is(err, sql.ErrNoRows) {
		return KindNotFound
	}

	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) && sqliteErr != nil {
		return classifyCode(sqliteErr.Code())
	}

	// An error originated by this package is already classified; re-deriving a
	// kind from whatever wraps it would discard the decision made at the point
	// where the cause was known.
	var storageErr *Error
	if errors.As(err, &storageErr) && storageErr != nil {
		return storageErr.Kind
	}

	// An unclassified error is treated as environmental, never as corruption.
	// The safe default is "do not delete anything".
	return KindEnvironment
}

func classifyCode(code int) Kind {
	switch code & 0xff {
	case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED:
		return KindBusy
	case sqlite3.SQLITE_CORRUPT, sqlite3.SQLITE_NOTADB:
		return KindCorrupt
	case sqlite3.SQLITE_READONLY, sqlite3.SQLITE_PERM:
		return KindReadOnly
	case sqlite3.SQLITE_FULL, sqlite3.SQLITE_IOERR, sqlite3.SQLITE_CANTOPEN,
		sqlite3.SQLITE_NOMEM, sqlite3.SQLITE_NOLFS, sqlite3.SQLITE_TOOBIG:
		return KindEnvironment
	case sqlite3.SQLITE_CONSTRAINT:
		return KindConstraint
	default:
		return KindEnvironment
	}
}
