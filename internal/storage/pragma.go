package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// The fixed pragma set from docs/03 §3.3 and docs/05 §5.6.2 rule 2. These are
// not tunable: a different combination would change durability and contention
// behavior that other modules' tests assert against.
const (
	pragmaJournalMode = "WAL"
	pragmaSynchronous = "NORMAL"
	pragmaBusyTimeout = 5000
)

// pragmaDSN builds the connection query string for the requested mode.
//
// Pragmas are applied through the driver's _pragma query parameters rather than
// as statements after opening, because several of them (notably journal_mode and
// busy_timeout) are per-connection, not per-database. A statement run once on
// one pooled connection would leave the others unconfigured.
//
// query_only is added for a read-only instance as the second half of the
// connection-layer guarantee in docs/07 §7.5. mode=ro is the first half: it
// opens the file with SQLITE_OPEN_READONLY, so the refusal survives even if a
// caller later resets query_only.
func pragmaDSN(path string, mode Mode) string {
	params := fmt.Sprintf(
		"?_pragma=journal_mode(%s)&_pragma=synchronous(%s)&_pragma=busy_timeout(%d)",
		pragmaJournalMode, pragmaSynchronous, pragmaBusyTimeout,
	)
	if mode == ModeReadOnly {
		return "file:" + path + params + "&mode=ro&_pragma=query_only(1)"
	}
	return "file:" + path + params
}

// verifyPragmas reads the applied settings back and asserts they took effect
// (DB-6). Asserting only that no error was returned would not detect a pragma
// that was silently ignored.
func verifyPragmas(ctx context.Context, db *sql.DB, mode Mode) error {
	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		return wrap("read journal_mode", err)
	}
	// SQLite reports the journal mode in lower case even though the keyword is
	// upper case, so compare case-insensitively rather than against the literal
	// used to set it.
	if !strings.EqualFold(journalMode, pragmaJournalMode) {
		return fmt.Errorf("%w: journal_mode is %q, want %q", ErrPragmaMismatch, journalMode, pragmaJournalMode)
	}

	var synchronous int
	if err := db.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&synchronous); err != nil {
		return wrap("read synchronous", err)
	}
	// PRAGMA synchronous reports a numeric level; NORMAL is 1. Reading it back
	// as an integer avoids depending on the driver's name mapping.
	if synchronous != 1 {
		return fmt.Errorf("%w: synchronous is %d, want 1 (NORMAL)", ErrPragmaMismatch, synchronous)
	}

	var busyTimeout int
	if err := db.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		return wrap("read busy_timeout", err)
	}
	if busyTimeout != pragmaBusyTimeout {
		return fmt.Errorf("%w: busy_timeout is %d, want %d", ErrPragmaMismatch, busyTimeout, pragmaBusyTimeout)
	}

	if mode == ModeReadOnly {
		var queryOnly int
		if err := db.QueryRowContext(ctx, "PRAGMA query_only").Scan(&queryOnly); err != nil {
			return wrap("read query_only", err)
		}
		if queryOnly != 1 {
			return fmt.Errorf("%w: query_only is %d, want 1", ErrPragmaMismatch, queryOnly)
		}
	}
	return nil
}
