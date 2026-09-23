package storage

import (
	"context"
	"database/sql"
	"encoding/json"
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
// v8 lands the analysis pipeline tables: the batch/frame join and the
// per-batch frame transcriptions.
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
	{
		version: 8,
		name:    "analysis: batch_screenshots and observations",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			// Tables follow docs/03 §3.3.1 verbatim. idx_batch_screenshots_screenshot
			// is an addition over that schema: the unbatched-frames query probes
			// NOT EXISTS by screenshot_id, and the composite PK's leading column
			// is batch_id, which that probe cannot use — without this index every
			// scheduler tick would scan the whole join table.
			for _, stmt := range []string{
				`CREATE TABLE batch_screenshots (
					batch_id      INTEGER NOT NULL REFERENCES analysis_batches(id) ON DELETE CASCADE,
					screenshot_id INTEGER NOT NULL REFERENCES screenshots(id),
					PRIMARY KEY (batch_id, screenshot_id)
				)`,
				`CREATE INDEX idx_batch_screenshots_screenshot ON batch_screenshots (screenshot_id)`,
				`CREATE TABLE observations (
					id          INTEGER PRIMARY KEY,
					batch_id    INTEGER NOT NULL REFERENCES analysis_batches(id) ON DELETE CASCADE,
					start_ts    INTEGER NOT NULL,
					end_ts      INTEGER NOT NULL,
					observation TEXT    NOT NULL,
					metadata    TEXT,
					created_at  INTEGER NOT NULL
				)`,
				`CREATE INDEX idx_observations_batch ON observations (batch_id, start_ts)`,
				`CREATE INDEX idx_observations_span ON observations (start_ts, end_ts)`,
			} {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return wrap("create v8 analysis tables", err)
				}
			}
			return nil
		},
	},
	{
		version: 9,
		name:    "analysis: batch attempt counter",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			// attempts counts how often a batch has entered a failed state.
			// RequeueFailed refuses to requeue a batch whose attempts have
			// reached MaxBatchAttempts, so a deterministically failing batch
			// (unreadable frame file, provider that always emits unresolvable
			// clock strings) stops consuming LLM calls instead of retrying
			// forever on the cooldown clock. Existing rows start at 0.
			if _, err := tx.ExecContext(ctx,
				`ALTER TABLE analysis_batches ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0`); err != nil {
				return wrap("add batch attempts column", err)
			}
			return nil
		},
	},
	{
		version: 10,
		name:    "analysis: batch soft delete",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			// is_deleted marks a failed batch the user dismissed from the
			// timeline's failure panel. The row and its batch_screenshots
			// membership stay: without the membership the frames would
			// resurface as unbatched and be re-analyzed, resurrecting the
			// failure the user just removed. Existing rows start at 0.
			if _, err := tx.ExecContext(ctx,
				`ALTER TABLE analysis_batches ADD COLUMN is_deleted INTEGER NOT NULL DEFAULT 0`); err != nil {
				return wrap("add batch is_deleted column", err)
			}
			return nil
		},
	},
	{
		version: 11,
		name:    "providers: per-provider image cap",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			// max_images caps how many image parts one provider request may
			// carry: gateways reject requests past their own limit
			// (terminal_error_too_many_images) and the recognition
			// enhancement multiplies one frame into five images, so the cap
			// must be adjustable per provider. 0 means the ai.MaxImages
			// default; existing rows start there.
			if _, err := tx.ExecContext(ctx,
				`ALTER TABLE providers ADD COLUMN max_images INTEGER NOT NULL DEFAULT 0`); err != nil {
				return wrap("add provider max_images column", err)
			}
			return nil
		},
	},
	{
		version: 12,
		name:    "categories: first-run starter set",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			// Seed the starter user categories (docs/decisions
			// timeline-starter-categories): the Dayflow-derived set of focus /
			// communication / learning / research / distraction / personal,
			// with the semantic details that steer the LLM's classification.
			// The seed is keyed on name, not id: a database that already has
			// any user-defined category (a name outside the built-ins) is
			// considered customized and gets nothing.
			rows, err := tx.QueryContext(ctx,
				`SELECT COUNT(*) FROM categories WHERE is_system = 0`)
			if err != nil {
				return wrap("count user categories", err)
			}
			var existing int
			if rows.Next() {
				if err := rows.Scan(&existing); err != nil {
					_ = rows.Close()
					return wrap("count user categories", err)
				}
			}
			_ = rows.Close()
			if existing > 0 {
				return nil
			}
			return seedStarterCategories(ctx, tx)
		},
	},
	{
		version: 13,
		name:    "daily: standup entries for AI-generated recap",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			// daily_standup_entries stores AI-generated daily recaps.
			// The standup_day column uses the calendar day (midnight boundary),
			// not the logical day (4am boundary), as this is the day users
			// think in when reviewing "today's" work.
			for _, stmt := range []string{
				`CREATE TABLE daily_standup_entries (
					standup_day      TEXT PRIMARY KEY,
					highlights_title TEXT NOT NULL,
					highlights       TEXT NOT NULL,
					tasks_title      TEXT NOT NULL,
					tasks            TEXT NOT NULL,
					blockers_title   TEXT NOT NULL,
					blockers_body    TEXT NOT NULL,
					generated_at     INTEGER NOT NULL
				)`,
			} {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return wrap("create v13 daily standup table", err)
				}
			}
			return nil
		},
	},
	{
		version: 14,
		name:    "timeline: per-card review verdicts",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			// card_reviews stores the user's focus verdict per card from the
			// review flow. One row per card: re-judging overwrites; 撤销
			// deletes the row. minutes snapshots the card's duration at
			// judgment time so day totals stay stable even if the card is
			// later edited. Verdicts are statistics only — the card's own
			// category is never rewritten by a judgment.
			for _, stmt := range []string{
				`CREATE TABLE card_reviews (
					card_id    INTEGER PRIMARY KEY REFERENCES timeline_cards(id) ON DELETE CASCADE,
					day        TEXT    NOT NULL,
					verdict    TEXT    NOT NULL CHECK (verdict IN ('distraction', 'neutral', 'focus')),
					minutes    INTEGER NOT NULL,
					created_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL
				)`,
				`CREATE INDEX idx_card_reviews_day ON card_reviews (day)`,
			} {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return wrap("create v14 card_reviews table", err)
				}
			}
			return nil
		},
	},
	{
		version: 15,
		name:    "recording: pending_captures frame_index for segment appends",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			// pending_captures previously had UNIQUE(relative_path), assuming
			// one JPEG file per capture. Multi-frame HEVC segments append frames
			// into the same segment file, so pending rows share relative_path
			// and distinguish frames by frame_index.
			for _, stmt := range []string{
				`CREATE TABLE pending_captures_v15 (
					id            INTEGER PRIMARY KEY,
					relative_path TEXT    NOT NULL,
					frame_index   INTEGER NOT NULL DEFAULT 0,
					captured_at   INTEGER NOT NULL,
					idle_seconds  INTEGER,
					width         INTEGER NOT NULL,
					height        INTEGER NOT NULL,
					redacted      INTEGER NOT NULL DEFAULT 0,
					file_size     INTEGER NOT NULL DEFAULT 0,
					state         TEXT    NOT NULL,
					created_at    INTEGER NOT NULL,
					UNIQUE(relative_path, frame_index)
				)`,
				`INSERT INTO pending_captures_v15 (id, relative_path, frame_index, captured_at, idle_seconds, width, height, redacted, file_size, state, created_at)
				 SELECT id, relative_path, 0, captured_at, idle_seconds, width, height, redacted, file_size, state, created_at
				 FROM pending_captures`,
				`DROP TABLE pending_captures`,
				`ALTER TABLE pending_captures_v15 RENAME TO pending_captures`,
				`CREATE INDEX idx_pending_captures_state ON pending_captures (state, id)`,
			} {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return wrap("migrate v15 pending_captures table", err)
				}
			}
			return nil
		},
	},
	{
		version: 16,
		name:    "recording: amortize multi-frame segment screenshots file_size",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			// Multi-frame segments previously stored cumulative file_size on
			// each frame append. Amortize them per docs/03 §3.4 and AGENTS.md:
			// screenshots.file_size is the amortized per-frame share.
			_, err := tx.ExecContext(ctx, `
				UPDATE screenshots
				SET file_size = MAX(1, (
					SELECT MAX(s2.file_size) / COUNT(*)
					FROM screenshots s2
					WHERE s2.segment_path = screenshots.segment_path AND s2.is_deleted = 0
				))
				WHERE segment_path LIKE '%.mp4'
			`)
			if err != nil {
				return wrap("amortize v16 segment screenshots file_size", err)
			}
			return nil
		},
	},
	{
		version: 17,
		name:    "providers: single model to models json array",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			// A provider gains an ordered list of models under one endpoint/key
			// (decisions/providers-multi-model): the single `model` column
			// becomes `models`, a JSON array of model strings. The routing chain
			// migrates value-level in internal/settings (a bare id folds into a
			// {providerId, model:""} pair, which resolves to the provider's first
			// model), so no routing SQL runs here.
			//
			// Rebuild rather than ALTER: SQLite cannot drop the old column in
			// place, and the backfill (wrap the old model in a one-element array)
			// is done in Go so the JSON is encoded correctly rather than
			// hand-built with string concatenation.
			type oldRow struct {
				id    string
				model string
			}
			rows, err := tx.QueryContext(ctx, `SELECT id, model FROM providers`)
			if err != nil {
				return wrap("read v17 providers", err)
			}
			var existing []oldRow
			for rows.Next() {
				var r oldRow
				if err := rows.Scan(&r.id, &r.model); err != nil {
					_ = rows.Close()
					return wrap("scan v17 provider", err)
				}
				existing = append(existing, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return wrap("iterate v17 providers", err)
			}
			_ = rows.Close()

			for _, stmt := range []string{
				`CREATE TABLE providers_v17 (
					id           TEXT PRIMARY KEY,
					display_name TEXT    NOT NULL,
					protocol     TEXT    NOT NULL,
					endpoint     TEXT    NOT NULL,
					models       TEXT    NOT NULL DEFAULT '[]',
					max_images   INTEGER NOT NULL DEFAULT 0,
					created_at   INTEGER NOT NULL,
					updated_at   INTEGER NOT NULL
				)`,
				`INSERT INTO providers_v17 (id, display_name, protocol, endpoint, models, max_images, created_at, updated_at)
				 SELECT id, display_name, protocol, endpoint, '[]', max_images, created_at, updated_at
				 FROM providers`,
			} {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return wrap("build v17 providers table", err)
				}
			}
			for _, r := range existing {
				models := []string{}
				if r.model != "" {
					models = []string{r.model}
				}
				encoded, err := json.Marshal(models)
				if err != nil {
					return wrap("encode v17 provider models", err)
				}
				if _, err := tx.ExecContext(ctx,
					`UPDATE providers_v17 SET models = ? WHERE id = ?`, string(encoded), r.id); err != nil {
					return wrap("backfill v17 provider models", err)
				}
			}
			for _, stmt := range []string{
				`DROP TABLE providers`,
				`ALTER TABLE providers_v17 RENAME TO providers`,
			} {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return wrap("swap v17 providers table", err)
				}
			}
			return nil
		},
	},
	{
		version: 18,
		name:    "timeline: per-card summary ratings",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			// card_ratings stores the user's thumbs up/down on one card's
			// AI summary. One row per card: re-rating overwrites; tapping the
			// active thumb again deletes the row. Ratings are feedback on the
			// summary text only — they never rewrite the summary or the card's
			// category, so they live apart from card_reviews, where verdict is
			// NOT NULL and a rating-only row would have nothing to store.
			_, err := tx.ExecContext(ctx, `CREATE TABLE card_ratings (
					card_id    INTEGER PRIMARY KEY REFERENCES timeline_cards(id) ON DELETE CASCADE,
					rating     TEXT    NOT NULL CHECK (rating IN ('up', 'down')),
					created_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL
				)`)
			if err != nil {
				return wrap("create v18 card_ratings table", err)
			}
			return nil
		},
	},
	{
		version: 19,
		name:    "daily: drop journal_entries.summary",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			// The daily standup recap is the day's AI summary; a separate
			// journal-level summary was never generated and is being removed.
			// Rebuild the table without the column (the SQLite table-rebuild
			// pattern used by v15/v17) so the drop works on every SQLite build.
			stmts := []string{
				`CREATE TABLE journal_entries_v19 (
					day         TEXT PRIMARY KEY,
					intentions  TEXT,
					notes       TEXT,
					goals       TEXT,
					reflections TEXT,
					status      TEXT    NOT NULL,
					updated_at  INTEGER NOT NULL
				)`,
				`INSERT INTO journal_entries_v19 (day, intentions, notes, goals, reflections, status, updated_at)
				 SELECT day, intentions, notes, goals, reflections, status, updated_at FROM journal_entries`,
				`DROP TABLE journal_entries`,
				`ALTER TABLE journal_entries_v19 RENAME TO journal_entries`,
			}
			for _, stmt := range stmts {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return wrap("drop v19 journal summary", err)
				}
			}
			return nil
		},
	},
}

// seedStarterCategories inserts the starter user category set. Fixed IDs (like
// the built-ins) so a re-seed collides with itself; ON CONFLICT keeps any row
// the user already renamed into place. Names are English — they are data the
// LLM matches against, not UI copy.
func seedStarterCategories(ctx context.Context, tx *sql.Tx) error {
	const seededAt = 0 // migration time, like the built-ins
	rows := []struct {
		id        string
		name      string
		hex       string
		details   string
		sortOrder int
	}{
		{
			"00000000-0000-4000-8000-000000000011", "Focus Work", "#6A7EFF",
			"Focused work: writing, refactoring, or debugging code in an IDE or terminal; deep hands-on building",
			1,
		},
		{
			"00000000-0000-4000-8000-000000000012", "Communication", "#FFAE8C",
			"Meetings, standups, Slack, email, video calls, messaging, and syncs",
			2,
		},
		{
			"00000000-0000-4000-8000-000000000013", "Learning", "#56CFEE",
			"Lectures, reading docs or courses, flashcards, tutorials, and deliberately studying new skills",
			3,
		},
		{
			"00000000-0000-4000-8000-000000000014", "Research", "#C787F7",
			"Exploring tools and APIs, reading papers or Stack Overflow, and writing design docs or technical specs",
			4,
		},
		{
			"00000000-0000-4000-8000-000000000015", "Distraction", "#FF4721",
			"Unfocused browsing and passive content consumption: social media feeds, random videos, idle scrolling, entertainment with no clear intent, and gaming",
			5,
		},
		{
			"00000000-0000-4000-8000-000000000016", "Personal", "#ADE3E3",
			"Intentional non-work activity with a purpose: messaging friends and family, managing finances, booking travel, errands, life admin, and hobbies",
			6,
		},
	}
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO categories (id, name, color_hex, details, sort_order, is_system, is_idle, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 0, 0, ?, ?)
			ON CONFLICT(id) DO NOTHING`,
			r.id, r.name, r.hex, r.details, r.sortOrder, seededAt, seededAt); err != nil {
			return wrap("seed starter category "+r.name, err)
		}
	}
	return nil
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
