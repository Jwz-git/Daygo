package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/domain"

	_ "modernc.org/sqlite"
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

	// Both built-in categories must be seeded with the system flags set, and
	// the v12 starter set must be seeded alongside them: this fixture's
	// database has no user-defined categories, so it counts as first run.
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
	want := []string{
		"Communication", "Distraction", "Focus Work", "Idle",
		"Learning", "Personal", "Research", "System",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("categories = %v, want %v (built-ins + v12 starter set)", got, want)
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

// DB-2 for v8: upgrading a v7 database creates the analysis tables empty and
// leaves the conversation model override and the screenshot row untouched.
// Nothing may be auto-enrolled into a batch.
func TestMigrateV7FixturePreservesDataAndCreatesAnalysisTables(t *testing.T) {
	fixture := filepath.Join("testdata", "v7-chat-model.db")
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

	for _, table := range []string{"batch_screenshots", "observations"} {
		var count int
		if err := store.db.QueryRowContext(context.Background(),
			"SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatalf("query %s after upgrade: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s has %d rows after upgrade; the migration must not enroll anything", table, count)
		}
	}

	// The three indexes that make the scheduler's per-tick queries cheap must
	// exist (the screenshot-id index is the one the composite PK cannot serve).
	for _, index := range []string{
		"idx_batch_screenshots_screenshot", "idx_observations_batch", "idx_observations_span",
	} {
		var name string
		if err := store.db.QueryRowContext(context.Background(),
			"SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?", index).Scan(&name); err != nil {
			t.Fatalf("index %s missing after upgrade: %v", index, err)
		}
	}

	var model string
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT model FROM chat_conversations WHERE id = 'fixture-conversation'").Scan(&model); err != nil {
		t.Fatalf("read conversation model after upgrade: %v", err)
	}
	if model != "fixture-override-model" {
		t.Fatalf("model = %q; the migration altered the override", model)
	}

	var segmentPath string
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT segment_path FROM screenshots WHERE id = 11").Scan(&segmentPath); err != nil {
		t.Fatalf("read screenshot after upgrade: %v", err)
	}
	if segmentPath != "staging/frame-0011.jpg" {
		t.Fatalf("segment_path = %q; the migration altered the screenshot", segmentPath)
	}
}

func TestMigrateV8FixturePreservesDataAndAddsAttempts(t *testing.T) {
	fixture := filepath.Join("testdata", "v8-analysis-tables.db")
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

	// The pre-existing failed batch survives with its failure info and starts
	// at attempts = 0: the counter only counts failures from this build on.
	var status, kind, note string
	var attempts int
	if err := store.db.QueryRowContext(context.Background(),
		`SELECT status, failure_kind, failure_note, attempts FROM analysis_batches WHERE id = 7`).
		Scan(&status, &kind, &note, &attempts); err != nil {
		t.Fatalf("read batch after upgrade: %v", err)
	}
	if status != "failed" || kind != "network" || note != "fixture note" || attempts != 0 {
		t.Fatalf("batch = (%q, %q, %q, %d), want (failed, network, fixture note, 0)", status, kind, note, attempts)
	}

	// Membership and observations survive untouched.
	var members int
	if err := store.db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM batch_screenshots WHERE batch_id = 7`).Scan(&members); err != nil {
		t.Fatalf("count batch_screenshots: %v", err)
	}
	if members != 2 {
		t.Fatalf("batch members = %d, want 2", members)
	}
	var observation string
	if err := store.db.QueryRowContext(context.Background(),
		`SELECT observation FROM observations WHERE id = 3`).Scan(&observation); err != nil {
		t.Fatalf("read observation: %v", err)
	}
	if observation != "fixture observation" {
		t.Fatalf("observation = %q, the migration altered it", observation)
	}

	// A new failure through this build increments the counter.
	now := time.Unix(1700000400, 0)
	if err := store.Analysis().SetBatchStatus(context.Background(), 7, BatchPending, "", "", now); err != nil {
		t.Fatalf("requeue batch: %v", err)
	}
	if err := store.Analysis().SetBatchStatus(context.Background(), 7, BatchProcessing, "", "", now); err != nil {
		t.Fatalf("process batch: %v", err)
	}
	if err := store.Analysis().SetBatchStatus(context.Background(), 7, BatchFailed, "network", "", now); err != nil {
		t.Fatalf("fail batch: %v", err)
	}
	if err := store.db.QueryRowContext(context.Background(),
		`SELECT attempts FROM analysis_batches WHERE id = 7`).Scan(&attempts); err != nil {
		t.Fatalf("read attempts after failure: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts after one failure = %d, want 1", attempts)
	}
}

// DB-2 for v10: upgrade a database written by a v9-only build and assert the
// is_deleted column arrives as 0 on the pre-existing failed batch while its
// attempts, failure info, membership, and observations survive untouched.
func TestMigrateV9FixturePreservesDataAndAddsSoftDelete(t *testing.T) {
	fixture := filepath.Join("testdata", "v9-batch-attempts.db")
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

	var isDeleted, attempts int
	var kind, note string
	if err := store.db.QueryRowContext(context.Background(),
		`SELECT is_deleted, attempts, failure_kind, failure_note FROM analysis_batches WHERE id = 9`).
		Scan(&isDeleted, &attempts, &kind, &note); err != nil {
		t.Fatalf("read batch after upgrade: %v", err)
	}
	if isDeleted != 0 || attempts != 2 || kind != "llm_error" || note != "fixture note" {
		t.Fatalf("batch = (deleted:%d, attempts:%d, %q, %q), want (0, 2, llm_error, fixture note)", isDeleted, attempts, kind, note)
	}

	// Membership and observations survive untouched.
	var members int
	if err := store.db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM batch_screenshots WHERE batch_id = 9`).Scan(&members); err != nil {
		t.Fatalf("count batch_screenshots: %v", err)
	}
	if members != 2 {
		t.Fatalf("batch members = %d, want 2", members)
	}
	var observation string
	if err := store.db.QueryRowContext(context.Background(),
		`SELECT observation FROM observations WHERE id = 5`).Scan(&observation); err != nil {
		t.Fatalf("read observation: %v", err)
	}
	if observation != "fixture observation" {
		t.Fatalf("observation = %q, the migration altered it", observation)
	}
}

// DB-2 for v11: upgrade a database written by a v10-only build and assert
// max_images arrives as 0 (the built-in default) on the pre-existing
// providers while their other fields survive untouched.
func TestMigrateV10FixturePreservesDataAndAddsMaxImages(t *testing.T) {
	fixture := filepath.Join("testdata", "v10-batch-soft-delete.db")
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

	rows, err := store.db.QueryContext(context.Background(),
		`SELECT id, display_name, protocol, endpoint, model, max_images FROM providers ORDER BY id`)
	if err != nil {
		t.Fatalf("query providers: %v", err)
	}
	defer func() { _ = rows.Close() }()

	type providerRow struct {
		id, name, protocol, endpoint, model string
		maxImages                           int
	}
	var got []providerRow
	for rows.Next() {
		var r providerRow
		if err := rows.Scan(&r.id, &r.name, &r.protocol, &r.endpoint, &r.model, &r.maxImages); err != nil {
			t.Fatalf("scan provider: %v", err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate providers: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("providers = %d, want 2", len(got))
	}
	if got[0].id != "fixture-provider-a" || got[0].model != "fixture-model-a" || got[0].maxImages != 0 {
		t.Fatalf("provider a = %+v, want untouched fields and max_images 0", got[0])
	}
	if got[1].id != "fixture-provider-b" || got[1].protocol != "anthropic" || got[1].maxImages != 0 {
		t.Fatalf("provider b = %+v, want untouched fields and max_images 0", got[1])
	}
}

// DB-2 for v12: a v11 database with only the built-in categories is a
// never-customized one, so the upgrade must seed the starter set (six rows,
// non-system, with details) while the built-ins survive untouched.
func TestMigrateV11FixtureSeedsStarterCategories(t *testing.T) {
	fixture := filepath.Join("testdata", "v11-starter-categories.db")
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

	cats, err := store.Categories().List(context.Background())
	if err != nil {
		t.Fatalf("List categories: %v", err)
	}
	if len(cats) != 8 {
		t.Fatalf("categories = %d after upgrade, want 8 (2 built-ins + 6 starter)", len(cats))
	}
	byName := make(map[string]domain.Category, len(cats))
	for _, c := range cats {
		byName[c.Name] = c
	}
	if !byName["System"].IsSystem || byName["System"].IsIdle {
		t.Fatalf("System flags wrong after upgrade: %+v", byName["System"])
	}
	if !byName["Idle"].IsSystem || !byName["Idle"].IsIdle {
		t.Fatalf("Idle flags wrong after upgrade: %+v", byName["Idle"])
	}
	for _, name := range []string{"Focus Work", "Communication", "Learning", "Research", "Distraction", "Personal"} {
		c, ok := byName[name]
		if !ok {
			t.Fatalf("starter category %q missing after upgrade: %v", name, cats)
		}
		if c.IsSystem || c.IsIdle || c.Details == "" || c.ColorHex == "" {
			t.Fatalf("starter category %q wrong: %+v", name, c)
		}
	}
}

// The v12 seed must not touch a database that already has user-defined
// categories: an existing customization is authoritative, whatever it is.
func TestMigrateV12SkipsCustomizedCategorySet(t *testing.T) {
	fixture := filepath.Join("testdata", "v11-starter-categories.db")
	if _, err := os.Stat(fixture); err != nil {
		t.Fatalf("fixture missing (%v); regenerate with: go run ./internal/storage/testdata/gen.go", err)
	}

	dir := newDir(t)
	dst := filepath.Join(dir, DatabaseFileName)

	// Prepare a customized v11 database with a raw connection first: opening
	// it through Open would already run the v12 migration and seed the
	// starter set, defeating the point of this test. One user category marks
	// the database as customized while it is still at version 11.
	src, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(dst, src, 0o600); err != nil {
		t.Fatalf("write fixture copy: %v", err)
	}
	prep, err := sql.Open("sqlite", "file:"+dst)
	if err != nil {
		t.Fatalf("open prep connection: %v", err)
	}
	if _, err := prep.Exec(`INSERT INTO categories (id, name, color_hex, details, sort_order, is_system, is_idle, created_at, updated_at)
		 VALUES ('11111111-2222-4333-8444-555555555555', 'Fixture Custom', '#123456', 'fixture details', 5, 0, 0, 0, 0)`); err != nil {
		t.Fatalf("insert custom category: %v", err)
	}
	if err := prep.Close(); err != nil {
		t.Fatalf("close prep connection: %v", err)
	}

	store := openWriter(t, dir)
	if got := userVersionOf(t, store); got != schemaVersion() {
		t.Fatalf("user_version = %d after upgrade, want %d", got, schemaVersion())
	}

	cats, err := store.Categories().List(context.Background())
	if err != nil {
		t.Fatalf("List categories: %v", err)
	}
	if len(cats) != 3 {
		t.Fatalf("categories = %d after upgrade, want 3 (built-ins + the custom row only)", len(cats))
	}
	var sawCustom bool
	for _, c := range cats {
		if c.Name == "Fixture Custom" {
			sawCustom = true
		} else if c.Name != "System" && c.Name != "Idle" {
			t.Fatalf("unexpected category %q seeded into a customized database", c.Name)
		}
	}
	if !sawCustom {
		t.Fatal("the pre-existing custom category was lost by the upgrade")
	}
}
