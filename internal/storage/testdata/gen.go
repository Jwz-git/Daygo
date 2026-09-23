//go:build ignore

// Command gen writes the anonymous binary fixtures used by the storage tests.
//
// Run from the repository root:
//
//	go run ./internal/storage/testdata/gen.go
//
// The generator is committed so the fixtures are reproducible; the .db files it
// writes are committed too, because DB-2 needs a database written by a previous
// version and that cannot be recreated by this build's own DDL.
//
// Every value here is invented. No fixture may contain real user data, and none
// does: docs/modules/data.md requires isolated, anonymous fixtures.
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func main() {
	outDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	// Allow running from the repository root or from this directory.
	if _, err := os.Stat(filepath.Join(outDir, "gen.go")); err != nil {
		outDir = filepath.Join(outDir, "internal", "storage", "testdata")
	}

	if err := writeV0WithSettings(filepath.Join(outDir, "v0-with-settings.db")); err != nil {
		log.Fatalf("v0-with-settings.db: %v", err)
	}
	if err := writeV1WithSettings(filepath.Join(outDir, "v1-with-settings.db")); err != nil {
		log.Fatalf("v1-with-settings.db: %v", err)
	}
	if err := writeV3(filepath.Join(outDir, "v3-recording.db")); err != nil {
		log.Fatalf("v3-recording.db: %v", err)
	}
	if err := writeV4(filepath.Join(outDir, "v4-chat.db")); err != nil {
		log.Fatalf("v4-chat.db: %v", err)
	}
	if err := writeV5(filepath.Join(outDir, "v5-daily.db")); err != nil {
		log.Fatalf("v5-daily.db: %v", err)
	}
	if err := writeV6(filepath.Join(outDir, "v6-chat-tools.db")); err != nil {
		log.Fatalf("v6-chat-tools.db: %v", err)
	}
	if err := writeV7(filepath.Join(outDir, "v7-chat-model.db")); err != nil {
		log.Fatalf("v7-chat-model.db: %v", err)
	}
	if err := writeV8(filepath.Join(outDir, "v8-analysis-tables.db")); err != nil {
		log.Fatalf("v8-analysis-tables.db: %v", err)
	}
	if err := writeV9(filepath.Join(outDir, "v9-batch-attempts.db")); err != nil {
		log.Fatalf("v9-batch-attempts.db: %v", err)
	}
	if err := writeV10(filepath.Join(outDir, "v10-batch-soft-delete.db")); err != nil {
		log.Fatalf("v10-batch-soft-delete.db: %v", err)
	}
	if err := writeV11(filepath.Join(outDir, "v11-starter-categories.db")); err != nil {
		log.Fatalf("v11-starter-categories.db: %v", err)
	}
	if err := writeV13(outDir); err != nil {
		log.Fatalf("v13-standup-entries.db: %v", err)
	}
	if err := writeV14(outDir); err != nil {
		log.Fatalf("v14-card-reviews.db: %v", err)
	}
	if err := writeV15(outDir); err != nil {
		log.Fatalf("v15-pending-frame-index.db: %v", err)
	}
	if err := writeV16(outDir); err != nil {
		log.Fatalf("v16-providers.db: %v", err)
	}
	if err := writeV17(outDir); err != nil {
		log.Fatalf("v17-provider-models.db: %v", err)
	}
	if err := writeV18(outDir); err != nil {
		log.Fatalf("v18-card-ratings.db: %v", err)
	}
	if err := writeTruncated(filepath.Join(outDir, "truncated.db")); err != nil {
		log.Fatalf("truncated.db: %v", err)
	}
	if err := writeNotADatabase(filepath.Join(outDir, "notadb.db")); err != nil {
		log.Fatalf("notadb.db: %v", err)
	}
	fmt.Println("fixtures written to", outDir)
}

// writeV0WithSettings builds a version-0 database holding a table this build
// does not create, so a migration test can prove the upgrade does not disturb
// pre-existing data.
func writeV0WithSettings(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE legacy_marker (
			id   INTEGER PRIMARY KEY,
			note TEXT NOT NULL
		)`,
		`INSERT INTO legacy_marker (id, note) VALUES
			(1, 'anonymous-fixture-alpha'),
			(2, 'anonymous-fixture-beta')`,
		// user_version stays 0: this database predates the migration chain.
		`PRAGMA user_version = 0`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	return nil
}

// writeV1WithSettings builds a version-1 database with app_settings rows, the
// last state a v1-only build can produce. v2's fixture test upgrades this file
// and asserts the settings survive alongside the new tables.
func writeV1WithSettings(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE app_settings (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`INSERT INTO app_settings (key, value, updated_at) VALUES
			('appearance.theme', '"system"', 1700000000),
			('capture.intervalSeconds', '15', 1700000001)`,
		`PRAGMA user_version = 1`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	return nil
}

// writeV3 builds a version-3 database (the recording tables are the last state
// a v3-only build can produce) with anonymous provider and chat rows, so the
// v4 migration test can prove the upgrade creates the new tables without
// disturbing pre-existing data. There is no earlier providers table: v4 is
// where it first exists, so its rows cannot predate the migration.
func writeV3(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE app_settings (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`INSERT INTO app_settings (key, value, updated_at) VALUES
			('providers.routing', '{"primary":"fixture-primary","secondary":"fixture-secondary"}', 1700000002)`,
		`CREATE TABLE analysis_batches (id INTEGER PRIMARY KEY, start_ts INTEGER NOT NULL, end_ts INTEGER NOT NULL, status TEXT NOT NULL, failure_kind TEXT, failure_note TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE timeline_cards (id INTEGER PRIMARY KEY, batch_id INTEGER REFERENCES analysis_batches(id), day TEXT NOT NULL, start TEXT NOT NULL, end TEXT NOT NULL, start_ts INTEGER NOT NULL, end_ts INTEGER NOT NULL, category TEXT NOT NULL, subcategory TEXT, title TEXT NOT NULL, summary TEXT NOT NULL, detailed_summary TEXT, video_summary_path TEXT, metadata TEXT, is_deleted INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE categories (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, color_hex TEXT NOT NULL, details TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL, is_system INTEGER NOT NULL DEFAULT 0, is_idle INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`INSERT INTO categories (id, name, color_hex, details, sort_order, is_system, is_idle, created_at, updated_at) VALUES
			('00000000-0000-4000-8000-000000000001', 'System', '#8E8E93', '', 0, 1, 0, 0, 0),
			('00000000-0000-4000-8000-000000000002', 'Idle', '#C7C7CC', '', 0, 1, 1, 0, 0)`,
		`CREATE TABLE pending_captures (id INTEGER PRIMARY KEY, relative_path TEXT NOT NULL UNIQUE, captured_at INTEGER NOT NULL, idle_seconds INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER NOT NULL DEFAULT 0, state TEXT NOT NULL, created_at INTEGER NOT NULL)`,
		`CREATE TABLE screenshots (id INTEGER PRIMARY KEY, segment_path TEXT NOT NULL, frame_index INTEGER NOT NULL, captured_at INTEGER NOT NULL, idle_seconds_at_capture INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER, is_deleted INTEGER NOT NULL DEFAULT 0, UNIQUE(segment_path, frame_index))`,
		// A pending capture row: migration must not touch the recording tables.
		`INSERT INTO pending_captures (id, relative_path, captured_at, idle_seconds, width, height, redacted, file_size, state, created_at)
			VALUES (7, 'seg-0001/frame-0001.png', 1700000100, NULL, 100, 100, 0, 0, 'pending', 1700000101)`,
		`PRAGMA user_version = 3`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	return nil
}

// writeV4 builds a version-4 database (the providers/chat tables are the last
// state a v4-only build can produce) with anonymous rows, so the v5 migration
// test can prove the upgrade creates the daily tables without disturbing
// pre-existing data. journal_entries/day_goals first exist in v5, so their
// rows cannot predate the migration.
func writeV4(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE app_settings (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE analysis_batches (id INTEGER PRIMARY KEY, start_ts INTEGER NOT NULL, end_ts INTEGER NOT NULL, status TEXT NOT NULL, failure_kind TEXT, failure_note TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE timeline_cards (id INTEGER PRIMARY KEY, batch_id INTEGER REFERENCES analysis_batches(id), day TEXT NOT NULL, start TEXT NOT NULL, end TEXT NOT NULL, start_ts INTEGER NOT NULL, end_ts INTEGER NOT NULL, category TEXT NOT NULL, subcategory TEXT, title TEXT NOT NULL, summary TEXT NOT NULL, detailed_summary TEXT, video_summary_path TEXT, metadata TEXT, is_deleted INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE categories (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, color_hex TEXT NOT NULL, details TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL, is_system INTEGER NOT NULL DEFAULT 0, is_idle INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE pending_captures (id INTEGER PRIMARY KEY, relative_path TEXT NOT NULL UNIQUE, captured_at INTEGER NOT NULL, idle_seconds INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER NOT NULL DEFAULT 0, state TEXT NOT NULL, created_at INTEGER NOT NULL)`,
		`CREATE TABLE screenshots (id INTEGER PRIMARY KEY, segment_path TEXT NOT NULL, frame_index INTEGER NOT NULL, captured_at INTEGER NOT NULL, idle_seconds_at_capture INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER, is_deleted INTEGER NOT NULL DEFAULT 0, UNIQUE(segment_path, frame_index))`,
		`CREATE TABLE providers (id TEXT PRIMARY KEY, display_name TEXT NOT NULL, protocol TEXT NOT NULL, endpoint TEXT NOT NULL, model TEXT NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE chat_conversations (id TEXT PRIMARY KEY, title TEXT, provider_id TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE chat_messages (id INTEGER PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE, role TEXT NOT NULL, content TEXT NOT NULL, status TEXT, created_at INTEGER NOT NULL)`,
		// A provider and a conversation with a message: the migration must
		// leave them untouched.
		`INSERT INTO providers (id, display_name, protocol, endpoint, model, created_at, updated_at)
			VALUES ('fixture-provider', 'Fixture Provider', 'openai', 'https://example.invalid/v1', 'fixture-model', 1700000000, 1700000000)`,
		`INSERT INTO chat_conversations (id, title, provider_id, created_at, updated_at)
			VALUES ('fixture-conversation', 'fixture title', 'fixture-provider', 1700000001, 1700000002)`,
		`INSERT INTO chat_messages (id, conversation_id, role, content, status, created_at)
			VALUES (1, 'fixture-conversation', 'user', 'fixture question', NULL, 1700000002)`,
		`PRAGMA user_version = 4`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	return nil
}

// writeV5 builds a version-5 database (the daily tables are the last state a
// v5-only build can produce) with anonymous rows, so the v6 migration test can
// prove the upgrade creates llm_calls and the chat tool columns without
// disturbing pre-existing data. llm_calls first exists in v6, so its rows
// cannot predate the migration.
func writeV5(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE app_settings (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE analysis_batches (id INTEGER PRIMARY KEY, start_ts INTEGER NOT NULL, end_ts INTEGER NOT NULL, status TEXT NOT NULL, failure_kind TEXT, failure_note TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE timeline_cards (id INTEGER PRIMARY KEY, batch_id INTEGER REFERENCES analysis_batches(id), day TEXT NOT NULL, start TEXT NOT NULL, end TEXT NOT NULL, start_ts INTEGER NOT NULL, end_ts INTEGER NOT NULL, category TEXT NOT NULL, subcategory TEXT, title TEXT NOT NULL, summary TEXT NOT NULL, detailed_summary TEXT, video_summary_path TEXT, metadata TEXT, is_deleted INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE categories (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, color_hex TEXT NOT NULL, details TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL, is_system INTEGER NOT NULL DEFAULT 0, is_idle INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`INSERT INTO categories (id, name, color_hex, details, sort_order, is_system, is_idle, created_at, updated_at) VALUES
			('00000000-0000-4000-8000-000000000001', 'System', '#8E8E93', '', 0, 1, 0, 0, 0),
			('00000000-0000-4000-8000-000000000002', 'Idle', '#C7C7CC', '', 0, 1, 1, 0, 0)`,
		`CREATE TABLE pending_captures (id INTEGER PRIMARY KEY, relative_path TEXT NOT NULL UNIQUE, captured_at INTEGER NOT NULL, idle_seconds INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER NOT NULL DEFAULT 0, state TEXT NOT NULL, created_at INTEGER NOT NULL)`,
		`CREATE TABLE screenshots (id INTEGER PRIMARY KEY, segment_path TEXT NOT NULL, frame_index INTEGER NOT NULL, captured_at INTEGER NOT NULL, idle_seconds_at_capture INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER, is_deleted INTEGER NOT NULL DEFAULT 0, UNIQUE(segment_path, frame_index))`,
		`CREATE TABLE providers (id TEXT PRIMARY KEY, display_name TEXT NOT NULL, protocol TEXT NOT NULL, endpoint TEXT NOT NULL, model TEXT NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE chat_conversations (id TEXT PRIMARY KEY, title TEXT, provider_id TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE chat_messages (id INTEGER PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE, role TEXT NOT NULL, content TEXT NOT NULL, status TEXT, created_at INTEGER NOT NULL)`,
		`CREATE TABLE journal_entries (day TEXT PRIMARY KEY, intentions TEXT, notes TEXT, goals TEXT, reflections TEXT, summary TEXT, status TEXT NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE day_goals (day TEXT PRIMARY KEY, focus_target_minutes INTEGER NOT NULL, distraction_limit_minutes INTEGER NOT NULL, is_skipped INTEGER NOT NULL DEFAULT 0, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE day_goal_categories (day TEXT NOT NULL REFERENCES day_goals(day) ON DELETE CASCADE, category_id TEXT NOT NULL REFERENCES categories(id), role TEXT NOT NULL, sort_order INTEGER NOT NULL, PRIMARY KEY (day, category_id, role))`,
		// Anonymous rows the migration must leave untouched.
		`INSERT INTO providers (id, display_name, protocol, endpoint, model, created_at, updated_at)
			VALUES ('fixture-provider', 'Fixture Provider', 'openai', 'https://example.invalid/v1', 'fixture-model', 1700000000, 1700000000)`,
		`INSERT INTO chat_conversations (id, title, provider_id, created_at, updated_at)
			VALUES ('fixture-conversation', 'fixture title', 'fixture-provider', 1700000001, 1700000002)`,
		`INSERT INTO chat_messages (id, conversation_id, role, content, status, created_at)
			VALUES (1, 'fixture-conversation', 'user', 'fixture question', NULL, 1700000002)`,
		`INSERT INTO journal_entries (day, intentions, notes, goals, reflections, summary, status, updated_at)
			VALUES ('2026-09-12', 'fixture intentions', NULL, NULL, NULL, NULL, 'draft', 1700000003)`,
		`INSERT INTO day_goals (day, focus_target_minutes, distraction_limit_minutes, is_skipped, updated_at)
			VALUES ('2026-09-12', 120, 30, 0, 1700000003)`,
		`PRAGMA user_version = 5`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	return nil
}

// writeV6 builds a version-6 database (the chat tool columns and llm_calls are
// the last state a v6-only build can produce) with anonymous rows, so the v7
// migration test can prove the upgrade adds the conversation model column
// without disturbing pre-existing data. The model column first exists in v7,
// so its values cannot predate the migration.
func writeV6(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE app_settings (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE analysis_batches (id INTEGER PRIMARY KEY, start_ts INTEGER NOT NULL, end_ts INTEGER NOT NULL, status TEXT NOT NULL, failure_kind TEXT, failure_note TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE timeline_cards (id INTEGER PRIMARY KEY, batch_id INTEGER REFERENCES analysis_batches(id), day TEXT NOT NULL, start TEXT NOT NULL, end TEXT NOT NULL, start_ts INTEGER NOT NULL, end_ts INTEGER NOT NULL, category TEXT NOT NULL, subcategory TEXT, title TEXT NOT NULL, summary TEXT NOT NULL, detailed_summary TEXT, video_summary_path TEXT, metadata TEXT, is_deleted INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE categories (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, color_hex TEXT NOT NULL, details TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL, is_system INTEGER NOT NULL DEFAULT 0, is_idle INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`INSERT INTO categories (id, name, color_hex, details, sort_order, is_system, is_idle, created_at, updated_at) VALUES
			('00000000-0000-4000-8000-000000000001', 'System', '#8E8E93', '', 0, 1, 0, 0, 0),
			('00000000-0000-4000-8000-000000000002', 'Idle', '#C7C7CC', '', 0, 1, 1, 0, 0)`,
		`CREATE TABLE pending_captures (id INTEGER PRIMARY KEY, relative_path TEXT NOT NULL UNIQUE, captured_at INTEGER NOT NULL, idle_seconds INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER NOT NULL DEFAULT 0, state TEXT NOT NULL, created_at INTEGER NOT NULL)`,
		`CREATE TABLE screenshots (id INTEGER PRIMARY KEY, segment_path TEXT NOT NULL, frame_index INTEGER NOT NULL, captured_at INTEGER NOT NULL, idle_seconds_at_capture INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER, is_deleted INTEGER NOT NULL DEFAULT 0, UNIQUE(segment_path, frame_index))`,
		`CREATE TABLE providers (id TEXT PRIMARY KEY, display_name TEXT NOT NULL, protocol TEXT NOT NULL, endpoint TEXT NOT NULL, model TEXT NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE chat_conversations (id TEXT PRIMARY KEY, title TEXT, provider_id TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE chat_messages (id INTEGER PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE, role TEXT NOT NULL, content TEXT NOT NULL, status TEXT, created_at INTEGER NOT NULL, tool_name TEXT, tool_arguments TEXT)`,
		`CREATE TABLE journal_entries (day TEXT PRIMARY KEY, intentions TEXT, notes TEXT, goals TEXT, reflections TEXT, summary TEXT, status TEXT NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE day_goals (day TEXT PRIMARY KEY, focus_target_minutes INTEGER NOT NULL, distraction_limit_minutes INTEGER NOT NULL, is_skipped INTEGER NOT NULL DEFAULT 0, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE day_goal_categories (day TEXT NOT NULL REFERENCES day_goals(day) ON DELETE CASCADE, category_id TEXT NOT NULL REFERENCES categories(id), role TEXT NOT NULL, sort_order INTEGER NOT NULL, PRIMARY KEY (day, category_id, role))`,
		`CREATE TABLE llm_calls (id INTEGER PRIMARY KEY, batch_id INTEGER REFERENCES analysis_batches(id), purpose TEXT NOT NULL, attempt_no INTEGER NOT NULL, provider_id TEXT NOT NULL, protocol TEXT NOT NULL, requested_model TEXT NOT NULL, actual_model TEXT, started_at INTEGER NOT NULL, finished_at INTEGER NOT NULL, latency_ms INTEGER NOT NULL, outcome TEXT NOT NULL, error_kind TEXT, http_status INTEGER, input_tokens INTEGER, output_tokens INTEGER, cache_read_tokens INTEGER, cache_write_tokens INTEGER)`,
		// Anonymous rows the migration must leave untouched.
		`INSERT INTO providers (id, display_name, protocol, endpoint, model, created_at, updated_at)
			VALUES ('fixture-provider', 'Fixture Provider', 'openai', 'https://example.invalid/v1', 'fixture-model', 1700000000, 1700000000)`,
		`INSERT INTO chat_conversations (id, title, provider_id, created_at, updated_at)
			VALUES ('fixture-conversation', 'fixture title', 'fixture-provider', 1700000001, 1700000002)`,
		`INSERT INTO chat_messages (id, conversation_id, role, content, status, created_at)
			VALUES (1, 'fixture-conversation', 'user', 'fixture question', NULL, 1700000002)`,
		`INSERT INTO journal_entries (day, intentions, notes, goals, reflections, summary, status, updated_at)
			VALUES ('2026-09-12', 'fixture intentions', NULL, NULL, NULL, NULL, 'draft', 1700000003)`,
		`INSERT INTO day_goals (day, focus_target_minutes, distraction_limit_minutes, is_skipped, updated_at)
			VALUES ('2026-09-12', 120, 30, 0, 1700000003)`,
		`PRAGMA user_version = 6`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	return nil
}

// writeV7 builds a version-7 database (the chat conversation model column is
// the last state a v7-only build can produce) with anonymous rows, so the v8
// migration test can prove the upgrade creates the analysis tables without
// disturbing pre-existing data. batch_screenshots/observations first exist in
// v8, so their rows cannot predate the migration.
func writeV7(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE app_settings (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE analysis_batches (id INTEGER PRIMARY KEY, start_ts INTEGER NOT NULL, end_ts INTEGER NOT NULL, status TEXT NOT NULL, failure_kind TEXT, failure_note TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE timeline_cards (id INTEGER PRIMARY KEY, batch_id INTEGER REFERENCES analysis_batches(id), day TEXT NOT NULL, start TEXT NOT NULL, end TEXT NOT NULL, start_ts INTEGER NOT NULL, end_ts INTEGER NOT NULL, category TEXT NOT NULL, subcategory TEXT, title TEXT NOT NULL, summary TEXT NOT NULL, detailed_summary TEXT, video_summary_path TEXT, metadata TEXT, is_deleted INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE categories (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, color_hex TEXT NOT NULL, details TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL, is_system INTEGER NOT NULL DEFAULT 0, is_idle INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`INSERT INTO categories (id, name, color_hex, details, sort_order, is_system, is_idle, created_at, updated_at) VALUES
			('00000000-0000-4000-8000-000000000001', 'System', '#8E8E93', '', 0, 1, 0, 0, 0),
			('00000000-0000-4000-8000-000000000002', 'Idle', '#C7C7CC', '', 0, 1, 1, 0, 0)`,
		`CREATE TABLE pending_captures (id INTEGER PRIMARY KEY, relative_path TEXT NOT NULL UNIQUE, captured_at INTEGER NOT NULL, idle_seconds INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER NOT NULL DEFAULT 0, state TEXT NOT NULL, created_at INTEGER NOT NULL)`,
		`CREATE TABLE screenshots (id INTEGER PRIMARY KEY, segment_path TEXT NOT NULL, frame_index INTEGER NOT NULL, captured_at INTEGER NOT NULL, idle_seconds_at_capture INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER, is_deleted INTEGER NOT NULL DEFAULT 0, UNIQUE(segment_path, frame_index))`,
		`CREATE TABLE providers (id TEXT PRIMARY KEY, display_name TEXT NOT NULL, protocol TEXT NOT NULL, endpoint TEXT NOT NULL, model TEXT NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE chat_conversations (id TEXT PRIMARY KEY, title TEXT, provider_id TEXT, model TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE chat_messages (id INTEGER PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE, role TEXT NOT NULL, content TEXT NOT NULL, status TEXT, created_at INTEGER NOT NULL, tool_name TEXT, tool_arguments TEXT)`,
		`CREATE TABLE journal_entries (day TEXT PRIMARY KEY, intentions TEXT, notes TEXT, goals TEXT, reflections TEXT, summary TEXT, status TEXT NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE day_goals (day TEXT PRIMARY KEY, focus_target_minutes INTEGER NOT NULL, distraction_limit_minutes INTEGER NOT NULL, is_skipped INTEGER NOT NULL DEFAULT 0, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE day_goal_categories (day TEXT NOT NULL REFERENCES day_goals(day) ON DELETE CASCADE, category_id TEXT NOT NULL REFERENCES categories(id), role TEXT NOT NULL, sort_order INTEGER NOT NULL, PRIMARY KEY (day, category_id, role))`,
		`CREATE TABLE llm_calls (id INTEGER PRIMARY KEY, batch_id INTEGER REFERENCES analysis_batches(id), purpose TEXT NOT NULL, attempt_no INTEGER NOT NULL, provider_id TEXT NOT NULL, protocol TEXT NOT NULL, requested_model TEXT NOT NULL, actual_model TEXT, started_at INTEGER NOT NULL, finished_at INTEGER NOT NULL, latency_ms INTEGER NOT NULL, outcome TEXT NOT NULL, error_kind TEXT, http_status INTEGER, input_tokens INTEGER, output_tokens INTEGER, cache_read_tokens INTEGER, cache_write_tokens INTEGER)`,
		// Anonymous rows the migration must leave untouched: a chat thread with
		// a model override, and a screenshot row the v8 join table must not
		// swallow into any batch.
		`INSERT INTO providers (id, display_name, protocol, endpoint, model, created_at, updated_at)
			VALUES ('fixture-provider', 'Fixture Provider', 'openai', 'https://example.invalid/v1', 'fixture-model', 1700000000, 1700000000)`,
		`INSERT INTO chat_conversations (id, title, provider_id, model, created_at, updated_at)
			VALUES ('fixture-conversation', 'fixture title', 'fixture-provider', 'fixture-override-model', 1700000001, 1700000002)`,
		`INSERT INTO chat_messages (id, conversation_id, role, content, status, created_at)
			VALUES (1, 'fixture-conversation', 'user', 'fixture question', NULL, 1700000002)`,
		`INSERT INTO screenshots (id, segment_path, frame_index, captured_at, idle_seconds_at_capture, width, height, redacted, file_size, is_deleted)
			VALUES (11, 'staging/frame-0011.jpg', 0, 1700000100, NULL, 1280, 720, 0, 2048, 0)`,
		`PRAGMA user_version = 7`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	return nil
}

// writeTruncated builds a database that starts life valid and is then cut short,
// which is how real torn writes present themselves.
// writeV8 builds a version-8 database (analysis join tables exist, no batch
// attempt counter yet). It carries anonymous analysis rows the v9 migration
// must preserve untouched: a failed batch with a failure kind, and
// observations plus batch membership referencing real screenshot rows.
func writeV8(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE app_settings (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE analysis_batches (id INTEGER PRIMARY KEY, start_ts INTEGER NOT NULL, end_ts INTEGER NOT NULL, status TEXT NOT NULL, failure_kind TEXT, failure_note TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE INDEX idx_batches_status ON analysis_batches (status)`,
		`CREATE TABLE timeline_cards (id INTEGER PRIMARY KEY, batch_id INTEGER REFERENCES analysis_batches(id), day TEXT NOT NULL, start TEXT NOT NULL, end TEXT NOT NULL, start_ts INTEGER NOT NULL, end_ts INTEGER NOT NULL, category TEXT NOT NULL, subcategory TEXT, title TEXT NOT NULL, summary TEXT NOT NULL, detailed_summary TEXT, video_summary_path TEXT, metadata TEXT, is_deleted INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE INDEX idx_cards_day  ON timeline_cards (day, start_ts)`,
		`CREATE INDEX idx_cards_span ON timeline_cards (start_ts, end_ts)`,
		`CREATE TABLE categories (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, color_hex TEXT NOT NULL, details TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL, is_system INTEGER NOT NULL DEFAULT 0, is_idle INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`INSERT INTO categories (id, name, color_hex, details, sort_order, is_system, is_idle, created_at, updated_at) VALUES
			('00000000-0000-4000-8000-000000000001', 'System', '#8E8E93', '', 0, 1, 0, 0, 0),
			('00000000-0000-4000-8000-000000000002', 'Idle', '#C7C7CC', '', 0, 1, 1, 0, 0)`,
		`CREATE TABLE pending_captures (id INTEGER PRIMARY KEY, relative_path TEXT NOT NULL UNIQUE, captured_at INTEGER NOT NULL, idle_seconds INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER NOT NULL DEFAULT 0, state TEXT NOT NULL, created_at INTEGER NOT NULL)`,
		`CREATE TABLE screenshots (id INTEGER PRIMARY KEY, segment_path TEXT NOT NULL, frame_index INTEGER NOT NULL, captured_at INTEGER NOT NULL, idle_seconds_at_capture INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER, is_deleted INTEGER NOT NULL DEFAULT 0, UNIQUE(segment_path, frame_index))`,
		`CREATE INDEX idx_screenshots_captured_at ON screenshots (captured_at)`,
		`CREATE TABLE providers (id TEXT PRIMARY KEY, display_name TEXT NOT NULL, protocol TEXT NOT NULL, endpoint TEXT NOT NULL, model TEXT NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE chat_conversations (id TEXT PRIMARY KEY, title TEXT, provider_id TEXT, model TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE chat_messages (id INTEGER PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE, role TEXT NOT NULL, content TEXT NOT NULL, status TEXT, created_at INTEGER NOT NULL, tool_name TEXT, tool_arguments TEXT)`,
		`CREATE TABLE journal_entries (day TEXT PRIMARY KEY, intentions TEXT, notes TEXT, goals TEXT, reflections TEXT, summary TEXT, status TEXT NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE day_goals (day TEXT PRIMARY KEY, focus_target_minutes INTEGER NOT NULL, distraction_limit_minutes INTEGER NOT NULL, is_skipped INTEGER NOT NULL DEFAULT 0, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE day_goal_categories (day TEXT NOT NULL REFERENCES day_goals(day) ON DELETE CASCADE, category_id TEXT NOT NULL REFERENCES categories(id), role TEXT NOT NULL, sort_order INTEGER NOT NULL, PRIMARY KEY (day, category_id, role))`,
		`CREATE TABLE llm_calls (id INTEGER PRIMARY KEY, batch_id INTEGER REFERENCES analysis_batches(id), purpose TEXT NOT NULL, attempt_no INTEGER NOT NULL, provider_id TEXT NOT NULL, protocol TEXT NOT NULL, requested_model TEXT NOT NULL, actual_model TEXT, started_at INTEGER NOT NULL, finished_at INTEGER NOT NULL, latency_ms INTEGER NOT NULL, outcome TEXT NOT NULL, error_kind TEXT, http_status INTEGER, input_tokens INTEGER, output_tokens INTEGER, cache_read_tokens INTEGER, cache_write_tokens INTEGER)`,
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
			end_ts    INTEGER NOT NULL,
			observation TEXT    NOT NULL,
			metadata    TEXT,
			created_at  INTEGER NOT NULL
		)`,
		`CREATE INDEX idx_observations_batch ON observations (batch_id, start_ts)`,
		`CREATE INDEX idx_observations_span ON observations (start_ts, end_ts)`,
		// Anonymous rows the migration must leave untouched.
		`INSERT INTO screenshots (id, segment_path, frame_index, captured_at, idle_seconds_at_capture, width, height, redacted, file_size, is_deleted)
			VALUES (21, 'staging/frame-0021.jpg', 0, 1700000200, NULL, 1280, 720, 0, 2048, 0),
			       (22, 'staging/frame-0022.jpg', 0, 1700000210, NULL, 1280, 720, 0, 2048, 0)`,
		`INSERT INTO analysis_batches (id, start_ts, end_ts, status, failure_kind, failure_note, created_at, updated_at)
			VALUES (7, 1700000200, 1700000210, 'failed', 'network', 'fixture note', 1700000300, 1700000300)`,
		`INSERT INTO batch_screenshots (batch_id, screenshot_id) VALUES (7, 21), (7, 22)`,
		`INSERT INTO observations (id, batch_id, start_ts, end_ts, observation, metadata, created_at)
			VALUES (3, 7, 1700000200, 1700000210, 'fixture observation', '{"apps":["FixtureApp"]}', 1700000300)`,
		`PRAGMA user_version = 8`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	return nil
}

// writeV9 builds a version-9 database (the batch attempt counter exists, no
// soft delete column yet). It carries a failed batch that has already burned
// two attempts plus membership and observations, so the v10 migration test can
// prove the is_deleted column arrives as 0 and the data survives untouched.
func writeV9(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE app_settings (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE analysis_batches (id INTEGER PRIMARY KEY, start_ts INTEGER NOT NULL, end_ts INTEGER NOT NULL, status TEXT NOT NULL, failure_kind TEXT, failure_note TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL, attempts INTEGER NOT NULL DEFAULT 0)`,
		`CREATE INDEX idx_batches_status ON analysis_batches (status)`,
		`CREATE TABLE timeline_cards (id INTEGER PRIMARY KEY, batch_id INTEGER REFERENCES analysis_batches(id), day TEXT NOT NULL, start TEXT NOT NULL, end TEXT NOT NULL, start_ts INTEGER NOT NULL, end_ts INTEGER NOT NULL, category TEXT NOT NULL, subcategory TEXT, title TEXT NOT NULL, summary TEXT NOT NULL, detailed_summary TEXT, video_summary_path TEXT, metadata TEXT, is_deleted INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE INDEX idx_cards_day  ON timeline_cards (day, start_ts)`,
		`CREATE INDEX idx_cards_span ON timeline_cards (start_ts, end_ts)`,
		`CREATE TABLE categories (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, color_hex TEXT NOT NULL, details TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL, is_system INTEGER NOT NULL DEFAULT 0, is_idle INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`INSERT INTO categories (id, name, color_hex, details, sort_order, is_system, is_idle, created_at, updated_at) VALUES
			('00000000-0000-4000-8000-000000000001', 'System', '#8E8E93', '', 0, 1, 0, 0, 0),
			('00000000-0000-4000-8000-000000000002', 'Idle', '#C7C7CC', '', 0, 1, 1, 0, 0)`,
		`CREATE TABLE pending_captures (id INTEGER PRIMARY KEY, relative_path TEXT NOT NULL UNIQUE, captured_at INTEGER NOT NULL, idle_seconds INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER NOT NULL DEFAULT 0, state TEXT NOT NULL, created_at INTEGER NOT NULL)`,
		`CREATE TABLE screenshots (id INTEGER PRIMARY KEY, segment_path TEXT NOT NULL, frame_index INTEGER NOT NULL, captured_at INTEGER NOT NULL, idle_seconds_at_capture INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER, is_deleted INTEGER NOT NULL DEFAULT 0, UNIQUE(segment_path, frame_index))`,
		`CREATE INDEX idx_screenshots_captured_at ON screenshots (captured_at)`,
		`CREATE TABLE providers (id TEXT PRIMARY KEY, display_name TEXT NOT NULL, protocol TEXT NOT NULL, endpoint TEXT NOT NULL, model TEXT NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE chat_conversations (id TEXT PRIMARY KEY, title TEXT, provider_id TEXT, model TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE chat_messages (id INTEGER PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE, role TEXT NOT NULL, content TEXT NOT NULL, status TEXT, created_at INTEGER NOT NULL, tool_name TEXT, tool_arguments TEXT)`,
		`CREATE TABLE journal_entries (day TEXT PRIMARY KEY, intentions TEXT, notes TEXT, goals TEXT, reflections TEXT, summary TEXT, status TEXT NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE day_goals (day TEXT PRIMARY KEY, focus_target_minutes INTEGER NOT NULL, distraction_limit_minutes INTEGER NOT NULL, is_skipped INTEGER NOT NULL DEFAULT 0, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE day_goal_categories (day TEXT NOT NULL REFERENCES day_goals(day) ON DELETE CASCADE, category_id TEXT NOT NULL REFERENCES categories(id), role TEXT NOT NULL, sort_order INTEGER NOT NULL, PRIMARY KEY (day, category_id, role))`,
		`CREATE TABLE llm_calls (id INTEGER PRIMARY KEY, batch_id INTEGER REFERENCES analysis_batches(id), purpose TEXT NOT NULL, attempt_no INTEGER NOT NULL, provider_id TEXT NOT NULL, protocol TEXT NOT NULL, requested_model TEXT NOT NULL, actual_model TEXT, started_at INTEGER NOT NULL, finished_at INTEGER NOT NULL, latency_ms INTEGER NOT NULL, outcome TEXT NOT NULL, error_kind TEXT, http_status INTEGER, input_tokens INTEGER, output_tokens INTEGER, cache_read_tokens INTEGER, cache_write_tokens INTEGER)`,
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
			end_ts    INTEGER NOT NULL,
			observation TEXT    NOT NULL,
			metadata    TEXT,
			created_at  INTEGER NOT NULL
		)`,
		`CREATE INDEX idx_observations_batch ON observations (batch_id, start_ts)`,
		`CREATE INDEX idx_observations_span ON observations (start_ts, end_ts)`,
		// Anonymous rows the migration must leave untouched.
		`INSERT INTO screenshots (id, segment_path, frame_index, captured_at, idle_seconds_at_capture, width, height, redacted, file_size, is_deleted)
			VALUES (31, 'staging/frame-0031.jpg', 0, 1700000300, NULL, 1280, 720, 0, 2048, 0),
			       (32, 'staging/frame-0032.jpg', 0, 1700000310, NULL, 1280, 720, 0, 2048, 0)`,
		`INSERT INTO analysis_batches (id, start_ts, end_ts, status, failure_kind, failure_note, created_at, updated_at, attempts)
			VALUES (9, 1700000300, 1700000310, 'failed', 'llm_error', 'fixture note', 1700000400, 1700000400, 2)`,
		`INSERT INTO batch_screenshots (batch_id, screenshot_id) VALUES (9, 31), (9, 32)`,
		`INSERT INTO observations (id, batch_id, start_ts, end_ts, observation, metadata, created_at)
			VALUES (5, 9, 1700000300, 1700000310, 'fixture observation', NULL, 1700000400)`,
		`PRAGMA user_version = 9`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	return nil
}

// writeV10 builds a version-10 database (batch soft delete exists, no
// provider image cap yet). It carries two anonymous providers so the v11
// migration test can prove max_images arrives as 0 (the default) while the
// provider rows survive untouched.
func writeV10(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE app_settings (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE analysis_batches (id INTEGER PRIMARY KEY, start_ts INTEGER NOT NULL, end_ts INTEGER NOT NULL, status TEXT NOT NULL, failure_kind TEXT, failure_note TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, is_deleted INTEGER NOT NULL DEFAULT 0)`,
		`CREATE INDEX idx_batches_status ON analysis_batches (status)`,
		`CREATE TABLE timeline_cards (id INTEGER PRIMARY KEY, batch_id INTEGER REFERENCES analysis_batches(id), day TEXT NOT NULL, start TEXT NOT NULL, end TEXT NOT NULL, start_ts INTEGER NOT NULL, end_ts INTEGER NOT NULL, category TEXT NOT NULL, subcategory TEXT, title TEXT NOT NULL, summary TEXT NOT NULL, detailed_summary TEXT, video_summary_path TEXT, metadata TEXT, is_deleted INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE INDEX idx_cards_day  ON timeline_cards (day, start_ts)`,
		`CREATE INDEX idx_cards_span ON timeline_cards (start_ts, end_ts)`,
		`CREATE TABLE categories (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, color_hex TEXT NOT NULL, details TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL, is_system INTEGER NOT NULL DEFAULT 0, is_idle INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`INSERT INTO categories (id, name, color_hex, details, sort_order, is_system, is_idle, created_at, updated_at) VALUES
			('00000000-0000-4000-8000-000000000001', 'System', '#8E8E93', '', 0, 1, 0, 0, 0),
			('00000000-0000-4000-8000-000000000002', 'Idle', '#C7C7CC', '', 0, 1, 1, 0, 0)`,
		`CREATE TABLE pending_captures (id INTEGER PRIMARY KEY, relative_path TEXT NOT NULL UNIQUE, captured_at INTEGER NOT NULL, idle_seconds INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER NOT NULL DEFAULT 0, state TEXT NOT NULL, created_at INTEGER NOT NULL)`,
		`CREATE TABLE screenshots (id INTEGER PRIMARY KEY, segment_path TEXT NOT NULL, frame_index INTEGER NOT NULL, captured_at INTEGER NOT NULL, idle_seconds_at_capture INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER, is_deleted INTEGER NOT NULL DEFAULT 0, UNIQUE(segment_path, frame_index))`,
		`CREATE INDEX idx_screenshots_captured_at ON screenshots (captured_at)`,
		`CREATE TABLE providers (id TEXT PRIMARY KEY, display_name TEXT NOT NULL, protocol TEXT NOT NULL, endpoint TEXT NOT NULL, model TEXT NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE chat_conversations (id TEXT PRIMARY KEY, title TEXT, provider_id TEXT, model TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE chat_messages (id INTEGER PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE, role TEXT NOT NULL, content TEXT NOT NULL, status TEXT, created_at INTEGER NOT NULL, tool_name TEXT, tool_arguments TEXT)`,
		`CREATE TABLE journal_entries (day TEXT PRIMARY KEY, intentions TEXT, notes TEXT, goals TEXT, reflections TEXT, summary TEXT, status TEXT NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE day_goals (day TEXT PRIMARY KEY, focus_target_minutes INTEGER NOT NULL, distraction_limit_minutes INTEGER NOT NULL, is_skipped INTEGER NOT NULL DEFAULT 0, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE day_goal_categories (day TEXT NOT NULL REFERENCES day_goals(day) ON DELETE CASCADE, category_id TEXT NOT NULL REFERENCES categories(id), role TEXT NOT NULL, sort_order INTEGER NOT NULL, PRIMARY KEY (day, category_id, role))`,
		`CREATE TABLE llm_calls (id INTEGER PRIMARY KEY, batch_id INTEGER REFERENCES analysis_batches(id), purpose TEXT NOT NULL, attempt_no INTEGER NOT NULL, provider_id TEXT NOT NULL, protocol TEXT NOT NULL, requested_model TEXT NOT NULL, actual_model TEXT, started_at INTEGER NOT NULL, finished_at INTEGER NOT NULL, latency_ms INTEGER NOT NULL, outcome TEXT NOT NULL, error_kind TEXT, http_status INTEGER, input_tokens INTEGER, output_tokens INTEGER, cache_read_tokens INTEGER, cache_write_tokens INTEGER)`,
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
			end_ts    INTEGER NOT NULL,
			observation TEXT    NOT NULL,
			metadata    TEXT,
			created_at  INTEGER NOT NULL
		)`,
		`CREATE INDEX idx_observations_batch ON observations (batch_id, start_ts)`,
		`CREATE INDEX idx_observations_span ON observations (start_ts, end_ts)`,
		// Two anonymous providers: the migration must leave their fields
		// untouched and give max_images the 0 default.
		`INSERT INTO providers (id, display_name, protocol, endpoint, model, created_at, updated_at)
			VALUES ('fixture-provider-a', 'Fixture A', 'openai', 'https://example.invalid/v1', 'fixture-model-a', 1700000000, 1700000000),
			       ('fixture-provider-b', 'Fixture B', 'anthropic', 'https://example.invalid', 'fixture-model-b', 1700000005, 1700000005)`,
		`PRAGMA user_version = 10`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	return nil
}

func writeTruncated(path string) error {
	if err := writeV0WithSettings(path); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	// Keep the header so the file is still recognizably a database, but drop
	// enough that the page tree cannot be read.
	if err := os.Truncate(path, info.Size()/3); err != nil {
		return err
	}
	return nil
}

// writeNotADatabase writes a file with no SQLite header at all.
func writeNotADatabase(path string) error {
	return os.WriteFile(path, []byte("this is not a sqlite database\n"), 0o600)
}

// writeV11 builds a version-11 database (provider image cap exists, no
// starter categories yet). It carries only the two built-in categories, so
// the v12 migration test can prove the starter set is seeded into a
// never-customized database while everything else survives untouched.
func writeV11(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE app_settings (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE analysis_batches (id INTEGER PRIMARY KEY, start_ts INTEGER NOT NULL, end_ts INTEGER NOT NULL, status TEXT NOT NULL, failure_kind TEXT, failure_note TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, is_deleted INTEGER NOT NULL DEFAULT 0)`,
		`CREATE INDEX idx_batches_status ON analysis_batches (status)`,
		`CREATE TABLE timeline_cards (id INTEGER PRIMARY KEY, batch_id INTEGER REFERENCES analysis_batches(id), day TEXT NOT NULL, start TEXT NOT NULL, end TEXT NOT NULL, start_ts INTEGER NOT NULL, end_ts INTEGER NOT NULL, category TEXT NOT NULL, subcategory TEXT, title TEXT NOT NULL, summary TEXT NOT NULL, detailed_summary TEXT, video_summary_path TEXT, metadata TEXT, is_deleted INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE INDEX idx_cards_day  ON timeline_cards (day, start_ts)`,
		`CREATE INDEX idx_cards_span ON timeline_cards (start_ts, end_ts)`,
		`CREATE TABLE categories (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, color_hex TEXT NOT NULL, details TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL, is_system INTEGER NOT NULL DEFAULT 0, is_idle INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`INSERT INTO categories (id, name, color_hex, details, sort_order, is_system, is_idle, created_at, updated_at) VALUES
			('00000000-0000-4000-8000-000000000001', 'System', '#8E8E93', '', 0, 1, 0, 0, 0),
			('00000000-0000-4000-8000-000000000002', 'Idle', '#C7C7CC', '', 0, 1, 1, 0, 0)`,
		`CREATE TABLE pending_captures (id INTEGER PRIMARY KEY, relative_path TEXT NOT NULL UNIQUE, captured_at INTEGER NOT NULL, idle_seconds INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER NOT NULL DEFAULT 0, state TEXT NOT NULL, created_at INTEGER NOT NULL)`,
		`CREATE TABLE screenshots (id INTEGER PRIMARY KEY, segment_path TEXT NOT NULL, frame_index INTEGER NOT NULL, captured_at INTEGER NOT NULL, idle_seconds_at_capture INTEGER, width INTEGER NOT NULL, height INTEGER NOT NULL, redacted INTEGER NOT NULL DEFAULT 0, file_size INTEGER, is_deleted INTEGER NOT NULL DEFAULT 0, UNIQUE(segment_path, frame_index))`,
		`CREATE INDEX idx_screenshots_captured_at ON screenshots (captured_at)`,
		`CREATE TABLE providers (id TEXT PRIMARY KEY, display_name TEXT NOT NULL, protocol TEXT NOT NULL, endpoint TEXT NOT NULL, model TEXT NOT NULL, max_images INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE chat_conversations (id TEXT PRIMARY KEY, title TEXT, provider_id TEXT, model TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE chat_messages (id INTEGER PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE, role TEXT NOT NULL, content TEXT NOT NULL, status TEXT, created_at INTEGER NOT NULL, tool_name TEXT, tool_arguments TEXT)`,
		`CREATE TABLE journal_entries (day TEXT PRIMARY KEY, intentions TEXT, notes TEXT, goals TEXT, reflections TEXT, summary TEXT, status TEXT NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE day_goals (day TEXT PRIMARY KEY, focus_target_minutes INTEGER NOT NULL, distraction_limit_minutes INTEGER NOT NULL, is_skipped INTEGER NOT NULL DEFAULT 0, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE day_goal_categories (day TEXT NOT NULL REFERENCES day_goals(day) ON DELETE CASCADE, category_id TEXT NOT NULL REFERENCES categories(id), role TEXT NOT NULL, sort_order INTEGER NOT NULL, PRIMARY KEY (day, category_id, role))`,
		`CREATE TABLE llm_calls (id INTEGER PRIMARY KEY, batch_id INTEGER REFERENCES analysis_batches(id), purpose TEXT NOT NULL, attempt_no INTEGER NOT NULL, provider_id TEXT NOT NULL, protocol TEXT NOT NULL, requested_model TEXT NOT NULL, actual_model TEXT, started_at INTEGER NOT NULL, finished_at INTEGER NOT NULL, latency_ms INTEGER NOT NULL, outcome TEXT NOT NULL, error_kind TEXT, http_status INTEGER, input_tokens INTEGER, output_tokens INTEGER, cache_read_tokens INTEGER, cache_write_tokens INTEGER)`,
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
			end_ts    INTEGER NOT NULL,
			observation TEXT    NOT NULL,
			metadata    TEXT,
			created_at  INTEGER NOT NULL
		)`,
		`CREATE INDEX idx_observations_batch ON observations (batch_id, start_ts)`,
		`CREATE INDEX idx_observations_span ON observations (start_ts, end_ts)`,
		`PRAGMA user_version = 11`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	return nil
}

// writeV13 builds a version-13 database from the v11 fixture by applying the
// v12 seed and the v13 standup table on top, plus a committed batch and card
// the v14 review table can attach to. This keeps the giant schema statement
// list in exactly one place (writeV11).
func writeV13(outDir string) error {
	v11Path := filepath.Join(outDir, "v11-starter-categories.db")
	v13Path := filepath.Join(outDir, "v13-standup-entries.db")
	data, err := os.ReadFile(v11Path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(v13Path, data, 0o600); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+v13Path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		// v12 seed equivalent: a starter-style user category set (fixture
		// names, same shape).
		`INSERT INTO categories (id, name, color_hex, details, sort_order, is_system, is_idle, created_at, updated_at) VALUES
			('00000000-0000-4000-8000-000000000011', 'Fixture Work', '#6A7EFF', 'fixture focused work', 1, 0, 0, 0, 0),
			('00000000-0000-4000-8000-000000000012', 'Fixture Personal', '#23C4A8', 'fixture personal time', 2, 0, 0, 0, 0)`,
		`INSERT INTO analysis_batches (id, start_ts, end_ts, status, created_at, updated_at) VALUES (1, 1789500000, 1789503600, 'succeeded', 0, 0)`,
		`INSERT INTO timeline_cards (id, batch_id, day, start, end, start_ts, end_ts, category, title, summary, is_deleted, created_at, updated_at)
		 VALUES (1, 1, '2026-09-16', '10:00 AM', '10:30 AM', 1789501200, 1789503000, 'Fixture Work', 'fixture card', 'fixture summary', 0, 0, 0)`,
		// v13: standup entries for AI-generated recaps.
		`CREATE TABLE daily_standup_entries (standup_day TEXT PRIMARY KEY, highlights_title TEXT NOT NULL, highlights TEXT NOT NULL, tasks_title TEXT NOT NULL, tasks TEXT NOT NULL, blockers_title TEXT NOT NULL, blockers_body TEXT NOT NULL, generated_at INTEGER NOT NULL)`,
		`INSERT INTO daily_standup_entries (standup_day, highlights_title, highlights, tasks_title, tasks, blockers_title, blockers_body, generated_at)
		 VALUES ('2026-09-16', 'fixture highlights title', 'fixture highlights', 'fixture tasks title', 'fixture tasks', 'fixture blockers title', 'fixture blockers', 1789510000)`,
		`PRAGMA user_version = 13`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// writeV14 builds a version-14 database from the v13 fixture by applying the
// v14 card_reviews table on top, plus a pending_captures row to test the v15
// migration.
func writeV14(outDir string) error {
	v13Path := filepath.Join(outDir, "v13-standup-entries.db")
	v14Path := filepath.Join(outDir, "v14-card-reviews.db")
	data, err := os.ReadFile(v13Path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(v14Path, data, 0o600); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+v14Path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE card_reviews (
			card_id    INTEGER PRIMARY KEY REFERENCES timeline_cards(id) ON DELETE CASCADE,
			day        TEXT    NOT NULL,
			verdict    TEXT    NOT NULL CHECK (verdict IN ('distraction', 'neutral', 'focus')),
			minutes    INTEGER NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE INDEX idx_card_reviews_day ON card_reviews (day)`,
		`INSERT INTO card_reviews (card_id, day, verdict, minutes, created_at, updated_at) VALUES (1, '2026-09-16', 'focus', 30, 1789510000, 1789510000)`,
		`INSERT INTO pending_captures (id, relative_path, captured_at, idle_seconds, width, height, redacted, file_size, state, created_at)
		 VALUES (1, 'staging/fixture-frame.jpg', 1789501200, NULL, 1920, 1080, 0, 1024, 'pending', 1789501200)`,
		`PRAGMA user_version = 14`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func writeV15(outDir string) error {
	v14Path := filepath.Join(outDir, "v14-card-reviews.db")
	v15Path := filepath.Join(outDir, "v15-pending-frame-index.db")
	data, err := os.ReadFile(v14Path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(v15Path, data, 0o600); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+v15Path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
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
		`INSERT INTO screenshots (id, segment_path, frame_index, captured_at, idle_seconds_at_capture, width, height, redacted, file_size, is_deleted)
		 VALUES (100, 'segments/fixture-segment.mp4', 0, 1789501000, NULL, 1920, 1080, 0, 1000, 0),
		        (101, 'segments/fixture-segment.mp4', 1, 1789501010, NULL, 1920, 1080, 0, 2000, 0)`,
		`PRAGMA user_version = 15`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// writeV16 builds a version-16 database from the v15 fixture by applying the
// v16 screenshot file_size amortization on top, then inserts two anonymous
// providers that still carry the single `model` column. The v17 migration test
// upgrades this file and proves each `model` becomes a one-element `models`
// JSON array while the provider's other fields and max_images survive.
func writeV16(outDir string) error {
	v15Path := filepath.Join(outDir, "v15-pending-frame-index.db")
	v16Path := filepath.Join(outDir, "v16-providers.db")
	data, err := os.ReadFile(v15Path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(v16Path, data, 0o600); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+v16Path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		// v16 amortizes multi-frame segment file_size in place; the fixture's
		// two segment frames get their cumulative sizes divided per frame.
		`UPDATE screenshots
		 SET file_size = MAX(1, (
			SELECT MAX(s2.file_size) / COUNT(*)
			FROM screenshots s2
			WHERE s2.segment_path = screenshots.segment_path AND s2.is_deleted = 0
		 ))
		 WHERE segment_path LIKE '%.mp4'`,
		// Two anonymous providers with the pre-v17 single model column. One
		// carries a non-zero max_images so the migration test can prove that
		// column survives the table rebuild.
		`INSERT INTO providers (id, display_name, protocol, endpoint, model, max_images, created_at, updated_at)
		 VALUES ('fixture-provider-a', 'Fixture A', 'openai', 'https://example.invalid/v1', 'fixture-model-a', 0, 1700000000, 1700000000),
		        ('fixture-provider-b', 'Fixture B', 'anthropic', 'https://example.invalid', 'fixture-model-b', 4, 1700000005, 1700000005)`,
		`PRAGMA user_version = 16`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// writeV17 builds a version-17 database from the v16 fixture by applying the
// v17 providers rebuild on top: the single `model` column becomes `models`, a
// JSON array. The v18 migration test upgrades this file and proves the ratings
// table arrives without disturbing the v17 provider rows, the v14 verdict or
// the v13 card.
func writeV17(outDir string) error {
	v16Path := filepath.Join(outDir, "v16-providers.db")
	v17Path := filepath.Join(outDir, "v17-provider-models.db")
	data, err := os.ReadFile(v16Path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(v17Path, data, 0o600); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+v17Path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
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
		// The two fixture models, wrapped in a one-element array exactly as the
		// v17 migration does.
		`UPDATE providers_v17 SET models = '["fixture-model-a"]' WHERE id = 'fixture-provider-a'`,
		`UPDATE providers_v17 SET models = '["fixture-model-b"]' WHERE id = 'fixture-provider-b'`,
		`DROP TABLE providers`,
		`ALTER TABLE providers_v17 RENAME TO providers`,
		`PRAGMA user_version = 17`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// writeV18 builds a version-18 database from the v17 fixture by applying the
// v18 card_ratings table on top, plus a journal_entries row carrying a summary
// value. The v19 migration test upgrades this file and proves the summary
// column is dropped while the row's other fields survive untouched.
func writeV18(outDir string) error {
	v17Path := filepath.Join(outDir, "v17-provider-models.db")
	v18Path := filepath.Join(outDir, "v18-card-ratings.db")
	data, err := os.ReadFile(v17Path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(v18Path, data, 0o600); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+v18Path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE card_ratings (
			card_id    INTEGER PRIMARY KEY REFERENCES timeline_cards(id) ON DELETE CASCADE,
			rating     TEXT    NOT NULL CHECK (rating IN ('up', 'down')),
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`INSERT INTO card_ratings (card_id, rating, created_at, updated_at) VALUES (1, 'up', 1789510000, 1789510000)`,
		// A journal row with a summary the v19 migration must drop while keeping
		// every other field.
		`INSERT INTO journal_entries (day, intentions, notes, goals, reflections, summary, status, updated_at)
		 VALUES ('2026-09-16', 'fixture intentions', 'fixture notes', NULL, NULL, 'fixture ai summary', 'intentions_set', 1789510000)`,
		`PRAGMA user_version = 18`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
