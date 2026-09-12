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
// v2 lands the cards capability (docs/09 §9.3): timeline_cards, categories,
// and the analysis_batches shell. analysis_batches is created early only
// because timeline_cards.batch_id references it — the batch state machine,
// screenshots, and observations arrive with the analysis pipeline (v3).
// timeline_cards.batch_id stays nullable so cards can exist before batches
// are written by anyone.
//
// The two built-in categories are seeded inside this same transaction:
// System (excluded from totals) and Idle (counts toward idle time). Seeding
// here rather than on first read means every database, including one whose
// only writer crashed mid-migration, either has both rows or neither.
// v5 lands the daily tables (journal entries and day goals). v6 lands the chat
// agent slice: the llm_calls audit table and the chat tool columns. v7 adds the
// per-conversation chat model override: the empty string follows the
// provider's configured model, any other value is used for that thread's turns.
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
	{
		version: 2,
		name:    "cards: timeline_cards, categories, analysis_batches",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			stmts := []string{
				`CREATE TABLE analysis_batches (
					id           INTEGER PRIMARY KEY,
					start_ts     INTEGER NOT NULL,
					end_ts       INTEGER NOT NULL,
					status       TEXT    NOT NULL,
					failure_kind TEXT,
					failure_note TEXT,
					created_at   INTEGER NOT NULL,
					updated_at   INTEGER NOT NULL
				)`,
				`CREATE INDEX idx_batches_status ON analysis_batches (status)`,
				`CREATE TABLE timeline_cards (
					id                 INTEGER PRIMARY KEY,
					batch_id           INTEGER REFERENCES analysis_batches(id),
					day                TEXT    NOT NULL,
					start              TEXT    NOT NULL,
					end                TEXT    NOT NULL,
					start_ts           INTEGER NOT NULL,
					end_ts             INTEGER NOT NULL,
					category           TEXT    NOT NULL,
					subcategory        TEXT,
					title              TEXT    NOT NULL,
					summary            TEXT    NOT NULL,
					detailed_summary   TEXT,
					video_summary_path TEXT,
					metadata           TEXT,
					is_deleted         INTEGER NOT NULL DEFAULT 0,
					created_at         INTEGER NOT NULL,
					updated_at         INTEGER NOT NULL
				)`,
				`CREATE INDEX idx_cards_day  ON timeline_cards (day, start_ts)`,
				`CREATE INDEX idx_cards_span ON timeline_cards (start_ts, end_ts)`,
				`CREATE TABLE categories (
					id          TEXT    PRIMARY KEY,
					name        TEXT    NOT NULL UNIQUE,
					color_hex   TEXT    NOT NULL,
					details     TEXT    NOT NULL DEFAULT '',
					sort_order  INTEGER NOT NULL,
					is_system   INTEGER NOT NULL DEFAULT 0,
					is_idle     INTEGER NOT NULL DEFAULT 0,
					created_at  INTEGER NOT NULL,
					updated_at  INTEGER NOT NULL
				)`,
			}
			for _, stmt := range stmts {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return wrap("create v2 tables", err)
				}
			}
			if err := seedBuiltInCategories(ctx, tx); err != nil {
				return err
			}
			return nil
		},
	},
	{
		version: 3,
		name:    "recording: pending captures and screenshots",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			for _, stmt := range []string{
				`CREATE TABLE pending_captures (id INTEGER PRIMARY KEY, relative_path TEXT NOT NULL UNIQUE, captured_at INTEGER NOT NULL, idle_seconds INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER NOT NULL DEFAULT 0, state TEXT NOT NULL, created_at INTEGER NOT NULL)`,
				`CREATE INDEX idx_pending_captures_state ON pending_captures (state, id)`,
				`CREATE TABLE screenshots (id INTEGER PRIMARY KEY, segment_path TEXT NOT NULL, frame_index INTEGER NOT NULL, captured_at INTEGER NOT NULL, idle_seconds_at_capture INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER, is_deleted INTEGER NOT NULL DEFAULT 0, UNIQUE(segment_path, frame_index))`,
				`CREATE INDEX idx_screenshots_captured_at ON screenshots (captured_at)`,
			} {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return wrap("create v3 recording tables", err)
				}
			}
			return nil
		},
	},
	{
		version: 4,
		name:    "providers and chat conversations/messages",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			// provider_id and role/status follow the shape docs/03 §3.3.4/§3.3.5
			// specifies; the tool_call/tool_result columns arrive with the chat
			// agent slice, not now. provider_id is nullable: NULL means no
			// provider is selected yet (decisions/chat-session-model).
			for _, stmt := range []string{
				`CREATE TABLE providers (
					id           TEXT PRIMARY KEY,
					display_name TEXT    NOT NULL,
					protocol     TEXT    NOT NULL,
					endpoint     TEXT    NOT NULL,
					model        TEXT    NOT NULL,
					created_at   INTEGER NOT NULL,
					updated_at   INTEGER NOT NULL
				)`,
				`CREATE TABLE chat_conversations (
					id          TEXT    PRIMARY KEY,
					title       TEXT,
					provider_id TEXT,
					created_at  INTEGER NOT NULL,
					updated_at  INTEGER NOT NULL
				)`,
				`CREATE TABLE chat_messages (
					id              INTEGER PRIMARY KEY,
					conversation_id TEXT    NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
					role            TEXT    NOT NULL,
					content         TEXT    NOT NULL,
					status          TEXT,
					created_at      INTEGER NOT NULL
				)`,
				`CREATE INDEX idx_chat_messages_conversation ON chat_messages (conversation_id, id)`,
			} {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return wrap("create v4 providers and chat tables", err)
				}
			}
			return nil
		},
	},
	{
		version: 5,
		name:    "daily: journal entries and day goals",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			// Columns follow docs/03 §3.3.4 verbatim. daily_standup_entries is
			// deliberately absent: no writer exists for it yet (the recap
			// generation slice, pending decision #19) and db-core does not
			// pre-create tables whose design is not settled with a consumer.
			for _, stmt := range []string{
				`CREATE TABLE journal_entries (
					day          TEXT PRIMARY KEY,
					intentions   TEXT,
					notes        TEXT,
					goals        TEXT,
					reflections  TEXT,
					summary      TEXT,
					status       TEXT NOT NULL,
					updated_at   INTEGER NOT NULL
				)`,
				`CREATE TABLE day_goals (
					day                       TEXT PRIMARY KEY,
					focus_target_minutes      INTEGER NOT NULL,
					distraction_limit_minutes INTEGER NOT NULL,
					is_skipped                INTEGER NOT NULL DEFAULT 0,
					updated_at                INTEGER NOT NULL
				)`,
				`CREATE TABLE day_goal_categories (
					day         TEXT NOT NULL REFERENCES day_goals(day) ON DELETE CASCADE,
					category_id TEXT NOT NULL REFERENCES categories(id),
					role        TEXT NOT NULL,
					sort_order  INTEGER NOT NULL,
					PRIMARY KEY (day, category_id, role)
				)`,
			} {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return wrap("create v5 daily tables", err)
				}
			}
			return nil
		},
	},
	{
		version: 6,
		name:    "chat agent: llm_calls audit and chat tool columns",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			// llm_calls follows docs/03 §3.3.1: attempt metadata only — no
			// endpoint, request/response body, image, key, or cost columns, ever.
			// The chat_messages columns were deferred from v4 per docs/03 §3.3.4
			// and arrive with the agent slice that writes them.
			for _, stmt := range []string{
				`CREATE TABLE llm_calls (
					id                 INTEGER PRIMARY KEY,
					batch_id           INTEGER REFERENCES analysis_batches(id),
					purpose            TEXT    NOT NULL,
					attempt_no         INTEGER NOT NULL,
					provider_id        TEXT    NOT NULL,
					protocol           TEXT    NOT NULL,
					requested_model    TEXT    NOT NULL,
					actual_model       TEXT,
					started_at         INTEGER NOT NULL,
					finished_at        INTEGER NOT NULL,
					latency_ms         INTEGER NOT NULL,
					outcome            TEXT    NOT NULL,
					error_kind         TEXT,
					http_status        INTEGER,
					input_tokens       INTEGER,
					output_tokens      INTEGER,
					cache_read_tokens  INTEGER,
					cache_write_tokens INTEGER
				)`,
				`CREATE INDEX idx_llm_calls_batch ON llm_calls (batch_id, purpose, attempt_no)`,
				`ALTER TABLE chat_messages ADD COLUMN tool_name TEXT`,
				`ALTER TABLE chat_messages ADD COLUMN tool_arguments TEXT`,
			} {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return wrap("create v6 llm_calls and chat tool columns", err)
				}
			}
			return nil
		},
	},
	{
		version: 7,
		name:    "chat: per-conversation model override",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			// '' (the default) means "use the provider's configured model";
			// a non-empty value overrides it for that conversation's turns.
			if _, err := tx.ExecContext(ctx,
				`ALTER TABLE chat_conversations ADD COLUMN model TEXT NOT NULL DEFAULT ''`); err != nil {
				return wrap("add chat conversation model column", err)
			}
			return nil
		},
	},
}

// seedBuiltInCategories inserts the two built-in categories. IDs are fixed
// constants, not generated per database: a re-seeded row must collide with
// itself (and be skipped) rather than duplicate under a second UUID.
func seedBuiltInCategories(ctx context.Context, tx *sql.Tx) error {
	const (
		systemID = "00000000-0000-4000-8000-000000000001"
		idleID   = "00000000-0000-4000-8000-000000000002"
		seededAt = 0 // migration time; the categories predate any user data
	)
	rows := []struct {
		id     string
		name   string
		hex    string
		isIdle int
	}{
		{systemID, "System", "#8E8E93", 0},
		{idleID, "Idle", "#C7C7CC", 1},
	}
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO categories (id, name, color_hex, details, sort_order, is_system, is_idle, created_at, updated_at)
			VALUES (?, ?, ?, '', 0, 1, ?, ?, ?)
			ON CONFLICT(id) DO NOTHING`,
			r.id, r.name, r.hex, r.isIdle, seededAt, seededAt); err != nil {
			return wrap("seed built-in category "+r.name, err)
		}
	}
	return nil
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
