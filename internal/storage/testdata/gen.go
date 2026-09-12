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
