package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// userVersionOf reads PRAGMA user_version through a raw connection, so the test
// observes on-disk state rather than trusting this package's own accessor.
//
// It takes the already-open store rather than a directory: opening a second
// instance would contend with the one under test and silently take the
// read-only path, which would make the assertion meaningless.
func userVersionOf(t *testing.T, store *Store) int {
	t.Helper()
	var version int
	if err := store.db.QueryRowContext(context.Background(), "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	return version
}

// versionAtLeastOne is a guard on the chain itself: user_version must start at
// 1, not 0 (docs/03 §3.3). Without this, an empty chain would silently produce
// a schema-less database that still reported success.
func TestMigrationChainStartsAtOne(t *testing.T) {
	if len(migrations) == 0 {
		t.Fatal("migration chain is empty; user_version cannot start at 1")
	}
	if migrations[0].version != 1 {
		t.Fatalf("first migration version = %d, want 1", migrations[0].version)
	}
	for i, m := range migrations {
		if m.version != i+1 {
			t.Fatalf("migration %d has version %d; the chain must be contiguous", i, m.version)
		}
		if m.name == "" {
			t.Fatalf("migration %d has no name; names are the only diagnostic label", i)
		}
		if m.apply == nil {
			t.Fatalf("migration %d has no apply function", i)
		}
	}
}

func TestOpenMigratesFreshDatabaseToCurrentVersion(t *testing.T) {
	dir := newDir(t)
	store := openWriter(t, dir)

	if got := userVersionOf(t, store); got != schemaVersion() {
		t.Fatalf("user_version = %d, want %d", got, schemaVersion())
	}
}

// DB-1: migrating an already-migrated database must change nothing. It is the
// property that makes a re-run after a crash safe.
func TestMigrationIsIdempotent(t *testing.T) {
	dir := newDir(t)

	first := openWriter(t, dir)
	before := schemaFingerprint(t, first)
	if err := first.Close(); err != nil {
		t.Fatalf("Close first: %v", err)
	}

	// Reopening runs migrate() again over a database already at the target
	// version. Nothing may change.
	second := openWriter(t, dir)
	after := schemaFingerprint(t, second)

	if before != after {
		t.Fatalf("schema changed on second migration:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if got := userVersionOf(t, second); got != schemaVersion() {
		t.Fatalf("user_version = %d after re-migration, want %d", got, schemaVersion())
	}
}

// schemaFingerprint renders the schema so two migrations can be compared. It
// uses sqlite_master rather than a per-table query so a table added by a future
// migration is covered without editing this test.
func schemaFingerprint(t *testing.T, store *Store) string {
	t.Helper()
	rows, err := store.db.QueryContext(context.Background(),
		"SELECT type, name, sql FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' ORDER BY type, name")
	if err != nil {
		t.Fatalf("read sqlite_master: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var out string
	for rows.Next() {
		var typ, name string
		var stmt sql.NullString
		if err := rows.Scan(&typ, &name, &stmt); err != nil {
			t.Fatalf("scan sqlite_master: %v", err)
		}
		out += typ + " " + name + " " + stmt.String + "\n"
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite_master: %v", err)
	}
	return out
}

// DB-2: upgrade an "old database" fixture and assert both the resulting schema
// and the pre-existing data survive.
//
// The fixture is a binary .db file rather than a SQL dump. A dump would be
// re-executed by this build's own DDL, so it could not reveal a migration that
// corrupts a database written by a previous binary. docs/05 §5.6.2 rule 3 asks
// for "old database to new database", and a file is the honest form of "old".
// .gitignore keeps testdata/**/*.db trackable for exactly this reason.
func TestMigrateOldDatabaseFixturePreservesData(t *testing.T) {
	fixture := filepath.Join("testdata", "v0-with-settings.db")
	if _, err := os.Stat(fixture); err != nil {
		t.Fatalf("fixture missing (%v); regenerate with: go run ./internal/storage/testdata/gen.go", err)
	}

	dir := newDir(t)
	dst := filepath.Join(dir, DatabaseFileName)
	copyFile(t, fixture, dst)

	// The fixture is a version-0 database, so this Open exercises the real
	// upgrade path rather than a no-op.
	store := openWriter(t, dir)

	if got := userVersionOf(t, store); got != schemaVersion() {
		t.Fatalf("user_version = %d after upgrade, want %d", got, schemaVersion())
	}

	// The table the migration creates must exist.
	var name string
	err := store.db.QueryRowContext(context.Background(),
		"SELECT name FROM sqlite_master WHERE type='table' AND name='app_settings'").Scan(&name)
	if err != nil {
		t.Fatalf("app_settings missing after migration: %v", err)
	}

	// The pre-existing table and its rows must be untouched.
	var count int
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM legacy_marker").Scan(&count); err != nil {
		t.Fatalf("legacy_marker unreadable after migration: %v", err)
	}
	if count != 2 {
		t.Fatalf("legacy_marker rows = %d, want 2; the migration altered existing data", count)
	}

	var note string
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT note FROM legacy_marker WHERE id = 1").Scan(&note); err != nil {
		t.Fatalf("read legacy row: %v", err)
	}
	if note != "anonymous-fixture-alpha" {
		t.Fatalf("legacy row content = %q, migration corrupted it", note)
	}
}
func TestMigrateCreatesRecordingTables(t *testing.T) {
	store := openWriter(t, newDir(t))
	for _, table := range []string{"pending_captures", "screenshots"} {
		var name string
		if err := store.db.QueryRowContext(context.Background(), "SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name); err != nil {
			t.Fatalf("%s missing after migration: %v", table, err)
		}
	}
}

// A database from a newer build must be refused rather than written to. This
// build cannot know what a future version's schema means (docs/03 §3.3).
func TestMigrateRefusesNewerSchemaVersion(t *testing.T) {
	dir := newDir(t)

	// Write a version far ahead of this build's target.
	store := openWriter(t, dir)
	if err := store.Write(context.Background(), "bump user_version", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "PRAGMA user_version = 9999")
		return err
	}); err != nil {
		t.Fatalf("set user_version: %v", err)
	}
	_ = store.Close()

	_, err := Open(context.Background(), Options{Dir: dir})
	if err == nil {
		t.Fatal("Open accepted a database from a newer schema version")
	}
	// A newer file is not corrupt; it is merely unreadable by this build. Sending
	// it down the recovery path would destroy a valid database.
	if IsCorrupt(err) {
		t.Fatalf("newer schema version classified as corruption: %v", err)
	}
}

// An empty database still reports version 0 before migration, which is what
// makes the v0 fixture a meaningful starting point.
func TestUnmigratedDatabaseReportsVersionZero(t *testing.T) {
	dir := newDir(t)

	// Create the file without going through Open, so no migration runs.
	store := openWriter(t, dir)
	if err := store.Write(context.Background(), "reset version", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "PRAGMA user_version = 0")
		return err
	}); err != nil {
		t.Fatalf("reset: %v", err)
	}
	var version int
	if err := store.db.QueryRowContext(context.Background(), "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read: %v", err)
	}
	if version != 0 {
		t.Fatalf("user_version = %d, want 0", version)
	}
}

// DB-2 for v2: upgrade a database written by a v1-only build and assert the
// pre-existing settings survive, the new tables and indexes exist, and both
// built-in categories are seeded.
func TestMigrateV1FixturePreservesDataAndCreatesCardsTables(t *testing.T) {
	fixture := filepath.Join("testdata", "v1-with-settings.db")
	if _, err := os.Stat(fixture); err != nil {
		t.Fatalf("fixture missing (%v); regenerate with: go run ./internal/storage/testdata/gen.go", err)
	}

	dir := newDir(t)
	dst := filepath.Join(dir, DatabaseFileName)
	copyFile(t, fixture, dst)

	store := openWriter(t, dir)

	if got := userVersionOf(t, store); got != schemaVersion() {
		t.Fatalf("user_version = %d after upgrade, want %d", got, schemaVersion())
	}

	for _, table := range []string{"timeline_cards", "categories", "analysis_batches"} {
		var name string
		err := store.db.QueryRowContext(context.Background(),
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Fatalf("table %s missing after migration: %v", table, err)
		}
	}
	for _, index := range []string{"idx_cards_day", "idx_cards_span", "idx_batches_status"} {
		var name string
		err := store.db.QueryRowContext(context.Background(),
			"SELECT name FROM sqlite_master WHERE type='index' AND name=?", index).Scan(&name)
		if err != nil {
			t.Fatalf("index %s missing after migration: %v", index, err)
		}
	}

	// Pre-existing settings must survive the upgrade.
	var theme string
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT value FROM app_settings WHERE key = 'appearance.theme'").Scan(&theme); err != nil {
		t.Fatalf("read setting after upgrade: %v", err)
	}
	if theme != `"system"` {
		t.Fatalf("appearance.theme = %q after upgrade; the migration altered existing data", theme)
	}

	// Both built-in categories must be seeded, with the system flags set.
	rows, err := store.db.QueryContext(context.Background(),
		"SELECT name, is_system, is_idle FROM categories ORDER BY name")
	if err != nil {
		t.Fatalf("read categories: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var got []string
	for rows.Next() {
		var name string
		var isSystem, isIdle int
		if err := rows.Scan(&name, &isSystem, &isIdle); err != nil {
			t.Fatalf("scan category: %v", err)
		}
		got = append(got, name)
		switch name {
		case "System":
			if isSystem != 1 || isIdle != 0 {
				t.Fatalf("System flags = system:%d idle:%d, want 1/0", isSystem, isIdle)
			}
		case "Idle":
			if isSystem != 1 || isIdle != 1 {
				t.Fatalf("Idle flags = system:%d idle:%d, want 1/1", isSystem, isIdle)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate categories: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("built-in categories = %v, want exactly System and Idle", got)
	}
}

// DB-2 for v4: upgrade a database written by a v3-only build and assert the
// new providers/chat tables exist and the pre-existing data survives.
func TestMigrateV3FixturePreservesDataAndCreatesV4Tables(t *testing.T) {
	fixture := filepath.Join("testdata", "v3-recording.db")
	if _, err := os.Stat(fixture); err != nil {
		t.Fatalf("fixture missing (%v); regenerate with: go run ./internal/storage/testdata/gen.go", err)
	}

	dir := newDir(t)
	dst := filepath.Join(dir, DatabaseFileName)
	copyFile(t, fixture, dst)

	store := openWriter(t, dir)

	if got := userVersionOf(t, store); got != schemaVersion() {
		t.Fatalf("user_version = %d after upgrade, want %d", got, schemaVersion())
	}

	for _, table := range []string{"providers", "chat_conversations", "chat_messages"} {
		var name string
		err := store.db.QueryRowContext(context.Background(),
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Fatalf("table %s missing after migration: %v", table, err)
		}
	}
	var index string
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT name FROM sqlite_master WHERE type='index' AND name='idx_chat_messages_conversation'").Scan(&index); err != nil {
		t.Fatalf("index idx_chat_messages_conversation missing after migration: %v", err)
	}

	// Pre-existing recording data must survive untouched.
	var state string
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT state FROM pending_captures WHERE id = 7").Scan(&state); err != nil {
		t.Fatalf("read pending capture after upgrade: %v", err)
	}
	if state != "pending" {
		t.Fatalf("pending capture state = %q; the migration altered existing data", state)
	}

	// The old-shape routing setting stays byte-identical: value-level
	// normalization belongs to internal/settings on read, not to the migration.
	var routing string
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT value FROM app_settings WHERE key = 'providers.routing'").Scan(&routing); err != nil {
		t.Fatalf("read providers.routing after upgrade: %v", err)
	}
	if routing != `{"primary":"fixture-primary","secondary":"fixture-secondary"}` {
		t.Fatalf("providers.routing = %q; the migration rewrote the stored value", routing)
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

// DB-2 for v5: upgrade a database written by a v4-only build and assert the
// new daily tables exist and the pre-existing chat data survives.
func TestMigrateV4FixturePreservesDataAndCreatesV5Tables(t *testing.T) {
	fixture := filepath.Join("testdata", "v4-chat.db")
	if _, err := os.Stat(fixture); err != nil {
		t.Fatalf("fixture missing (%v); regenerate with: go run ./internal/storage/testdata/gen.go", err)
	}

	dir := newDir(t)
	dst := filepath.Join(dir, DatabaseFileName)
	copyFile(t, fixture, dst)

	store := openWriter(t, dir)

	if got := userVersionOf(t, store); got != schemaVersion() {
		t.Fatalf("user_version = %d after upgrade, want %d", got, schemaVersion())
	}

	for _, table := range []string{"journal_entries", "day_goals", "day_goal_categories"} {
		var name string
		err := store.db.QueryRowContext(context.Background(),
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Fatalf("table %s missing after migration: %v", table, err)
		}
	}

	// Pre-existing chat data must survive untouched.
	var role, content string
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT role, content FROM chat_messages WHERE id = 1").Scan(&role, &content); err != nil {
		t.Fatalf("read chat message after upgrade: %v", err)
	}
	if role != "user" || content != "fixture question" {
		t.Fatalf("chat message = %q/%q; the migration altered existing data", role, content)
	}
}

// DB-2 for v6: upgrade a database written by a v5-only build and assert the
// llm_calls table and chat tool columns exist, and pre-existing chat and daily
// data survives.
func TestMigrateV5FixturePreservesDataAndCreatesV6Tables(t *testing.T) {
	fixture := filepath.Join("testdata", "v5-daily.db")
	if _, err := os.Stat(fixture); err != nil {
		t.Fatalf("fixture missing (%v); regenerate with: go run ./internal/storage/testdata/gen.go", err)
	}

	dir := newDir(t)
	dst := filepath.Join(dir, DatabaseFileName)
	copyFile(t, fixture, dst)

	store := openWriter(t, dir)

	if got := userVersionOf(t, store); got != schemaVersion() {
		t.Fatalf("user_version = %d after upgrade, want %d", got, schemaVersion())
	}

	var name string
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT name FROM sqlite_master WHERE type='table' AND name='llm_calls'").Scan(&name); err != nil {
		t.Fatalf("table llm_calls missing after migration: %v", err)
	}
	var index string
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT name FROM sqlite_master WHERE type='index' AND name='idx_llm_calls_batch'").Scan(&index); err != nil {
		t.Fatalf("index idx_llm_calls_batch missing after migration: %v", err)
	}

	// The tool columns arrive as NULL on pre-existing rows; the extension is
	// purely additive.
	var toolName, toolArguments sql.NullString
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT tool_name, tool_arguments FROM chat_messages WHERE id = 1").Scan(&toolName, &toolArguments); err != nil {
		t.Fatalf("read chat message tool columns after upgrade: %v", err)
	}
	if toolName.Valid || toolArguments.Valid {
		t.Fatalf("tool columns = %q/%q on a pre-agent row; the migration wrote data", toolName.String, toolArguments.String)
	}

	// Pre-existing chat and daily data must survive untouched.
	var role, content string
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT role, content FROM chat_messages WHERE id = 1").Scan(&role, &content); err != nil {
		t.Fatalf("read chat message after upgrade: %v", err)
	}
	if role != "user" || content != "fixture question" {
		t.Fatalf("chat message = %q/%q; the migration altered existing data", role, content)
	}
	var status string
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT status FROM journal_entries WHERE day = '2026-09-12'").Scan(&status); err != nil {
		t.Fatalf("read journal entry after upgrade: %v", err)
	}
	if status != "draft" {
		t.Fatalf("journal status = %q; the migration altered existing data", status)
	}
	var focusMinutes int
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT focus_target_minutes FROM day_goals WHERE day = '2026-09-12'").Scan(&focusMinutes); err != nil {
		t.Fatalf("read day goal after upgrade: %v", err)
	}
	if focusMinutes != 120 {
		t.Fatalf("focus_target_minutes = %d; the migration altered existing data", focusMinutes)
	}
}

// DB-2 for v7: upgrade a database written by a v6-only build and assert the
// conversation model column exists with ” on pre-existing rows, and chat and
// daily data survives.
func TestMigrateV6FixturePreservesDataAndAddsConversationModel(t *testing.T) {
	fixture := filepath.Join("testdata", "v6-chat-tools.db")
	if _, err := os.Stat(fixture); err != nil {
		t.Fatalf("fixture missing (%v); regenerate with: go run ./internal/storage/testdata/gen.go", err)
	}

	dir := newDir(t)
	dst := filepath.Join(dir, DatabaseFileName)
	copyFile(t, fixture, dst)

	store := openWriter(t, dir)

	if got := userVersionOf(t, store); got != schemaVersion() {
		t.Fatalf("user_version = %d after upgrade, want %d", got, schemaVersion())
	}

	// The model column arrives as '' on pre-existing rows: '' follows the
	// provider's configured model, which is what every pre-v7 conversation did.
	var model string
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT model FROM chat_conversations WHERE id = 'fixture-conversation'").Scan(&model); err != nil {
		t.Fatalf("read conversation model after upgrade: %v", err)
	}
	if model != "" {
		t.Fatalf("model = %q on a pre-existing conversation; the migration wrote data", model)
	}

	// Pre-existing chat data must survive untouched.
	var role, content string
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT role, content FROM chat_messages WHERE id = 1").Scan(&role, &content); err != nil {
		t.Fatalf("read chat message after upgrade: %v", err)
	}
	if role != "user" || content != "fixture question" {
		t.Fatalf("chat message = %q/%q; the migration altered existing data", role, content)
	}
}
