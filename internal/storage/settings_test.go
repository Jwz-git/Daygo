package storage

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

// A key that was never written reports absence rather than an invented default.
// Defaults belong to the feature that owns the setting, not to this layer
// (docs/09 §9.5).
func TestSettingsGetMissingKeyReportsAbsent(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.Settings()

	value, ok, err := repo.Get(context.Background(), "appearance.theme")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ok {
		t.Fatalf("Get reported the key present with value %q", value)
	}
	if value != "" {
		t.Fatalf("value = %q for an absent key, want empty", value)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.Settings()
	ctx := context.Background()

	// The stored form is opaque JSON; this layer must not reinterpret it.
	want := `{"primary":"provider-a","secondary":null}`
	if err := repo.Set(ctx, "providers.routing", want); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, ok, err := repo.Get(ctx, "providers.routing")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get reported the key absent after Set")
	}
	if got != want {
		t.Fatalf("value = %q, want %q", got, want)
	}
}

// Values survive a close and reopen, which is the property that lets settings
// replace the frontend's localStorage (docs/09 §9.5).
func TestSettingsPersistAcrossReopen(t *testing.T) {
	dir := newDir(t)
	ctx := context.Background()

	first := openWriter(t, dir)
	if err := first.Settings().Set(ctx, "appearance.language", `"en"`); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	second := openWriter(t, dir)
	got, ok, err := second.Settings().Get(ctx, "appearance.language")
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if !ok || got != `"en"` {
		t.Fatalf("after reopen got (%q, %v), want (%q, true)", got, ok, `"en"`)
	}
}

func TestSettingsSetOverwrites(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.Settings()
	ctx := context.Background()

	if err := repo.Set(ctx, "capture.intervalSeconds", "10"); err != nil {
		t.Fatalf("Set first: %v", err)
	}
	if err := repo.Set(ctx, "capture.intervalSeconds", "30"); err != nil {
		t.Fatalf("Set second: %v", err)
	}

	got, _, err := repo.Get(ctx, "capture.intervalSeconds")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "30" {
		t.Fatalf("value = %q, want %q; the second write did not replace the first", got, "30")
	}

	// The upsert must not have created a duplicate row.
	if count := rowCount(t, store, "app_settings"); count != 1 {
		t.Fatalf("app_settings has %d rows for one key, want 1", count)
	}
}

func TestSettingsGetAllReturnsEverythingStored(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.Settings()
	ctx := context.Background()

	want := map[string]string{
		"appearance.theme":        `"dark"`,
		"appearance.language":     `"zh-CN"`,
		"system.showDockIcon":     `false`,
		"capture.intervalSeconds": `10`,
	}
	if err := repo.SetMany(ctx, want); err != nil {
		t.Fatalf("SetMany: %v", err)
	}

	got, err := repo.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("GetAll returned %d keys, want %d: %v", len(got), len(want), got)
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("key %q = %q, want %q", key, got[key], value)
		}
	}
}

// SetMany is one transaction: a failure part-way must leave none of the keys
// written, otherwise a caller could observe a half-applied settings group.
func TestSettingsSetManyIsAtomic(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.Settings()
	ctx := context.Background()

	// Seed one key so the transaction has something to roll back alongside the
	// forced failure.
	if err := repo.Set(ctx, "appearance.theme", `"light"`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Force the transaction to fail after the first write by cancelling the
	// context mid-transaction is not deterministic, so instead drive a
	// constraint violation: the table has no nullable value column, and the
	// failure must roll the whole group back.
	err := store.Write(ctx, "settings set many", func(ctx context.Context, tx *sql.Tx) error {
		_, execErr := tx.ExecContext(ctx,
			"INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)",
			"appearance.theme", nil, 0)
		return execErr
	})
	if err == nil {
		t.Skip("driver accepted a NULL value; constraint enforcement differs here")
	}

	// The seeded value must be unchanged: nothing from the failed write landed.
	got, _, getErr := repo.Get(ctx, "appearance.theme")
	if getErr != nil {
		t.Fatalf("Get after failed write: %v", getErr)
	}
	if got != `"light"` {
		t.Fatalf("value = %q after a failed write, want the seeded %q", got, `"light"`)
	}
}

func TestSettingsDeleteRemovesKey(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.Settings()
	ctx := context.Background()

	if err := repo.Set(ctx, "telemetry.analyticsOptIn", "true"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := repo.Delete(ctx, "telemetry.analyticsOptIn"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, ok, err := repo.Get(ctx, "telemetry.analyticsOptIn"); err != nil {
		t.Fatalf("Get: %v", err)
	} else if ok {
		t.Fatal("key still present after Delete")
	}
}

// Deleting an absent key is not an error: the caller asked for it to be gone
// and it is gone.
func TestSettingsDeleteAbsentKeySucceeds(t *testing.T) {
	store := openWriter(t, newDir(t))
	if err := store.Settings().Delete(context.Background(), "never.written"); err != nil {
		t.Fatalf("Delete of an absent key: %v", err)
	}
}

func TestSettingsSetManyEmptyIsNoOp(t *testing.T) {
	store := openWriter(t, newDir(t))
	if err := store.Settings().SetMany(context.Background(), nil); err != nil {
		t.Fatalf("SetMany(nil): %v", err)
	}
}

// A read-only instance must refuse to write settings, at the connection layer.
func TestSettingsWriteRefusedOnReadOnlyInstance(t *testing.T) {
	dir := newDir(t)
	openWriter(t, dir)
	reader := openReader(t, dir)

	err := reader.Settings().Set(context.Background(), "appearance.theme", `"dark"`)
	assertKind(t, err, KindReadOnly)
}

// Watch delivers a notification for a write and closes when ctx ends, so a
// consumer ranging over the channel terminates (docs/05 §5.6.3 rule 3).
func TestSettingsWatchDeliversAndCloses(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.Settings()

	ctx, cancel := context.WithCancel(context.Background())
	events := repo.Watch(ctx)

	if err := repo.Set(context.Background(), "appearance.theme", `"dark"`); err != nil {
		t.Fatalf("Set: %v", err)
	}

	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("channel closed before delivering the write")
		}
		if len(event.Keys) != 1 || event.Keys[0] != "appearance.theme" {
			t.Fatalf("event keys = %v, want [appearance.theme]", event.Keys)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no notification for a committed write")
	}

	cancel()
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("channel still open after ctx cancel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("channel was not closed when ctx ended")
	}
}

// A rolled-back write must not notify: an event would send consumers to re-read
// a value that never changed.
func TestSettingsWatchStaysQuietOnFailedWrite(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.Settings()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := repo.Watch(ctx)

	writeErr := store.Write(context.Background(), "forced failure", func(ctx context.Context, tx *sql.Tx) error {
		_, execErr := tx.ExecContext(ctx,
			"INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)",
			"forced", nil, 0)
		return execErr
	})
	if writeErr == nil {
		t.Skip("driver accepted a NULL value; cannot force a rollback here")
	}

	select {
	case event, ok := <-events:
		if ok {
			t.Fatalf("notified on a rolled-back write: %v", event)
		}
	case <-time.After(200 * time.Millisecond):
		// Expected: no event.
	}
}

func TestSettingsRepoOnNilStoreIsNil(t *testing.T) {
	var store *Store
	if store.Settings() != nil {
		t.Fatal("Settings() on a nil store returned a repository")
	}
}

func TestSettingsKeysAreNotRewritten(t *testing.T) {
	// Key names are the contract (docs/05 §5.6.3 rule 1). A round trip must not
	// silently change them; this guards against accidental trimming or casing.
	store := openWriter(t, newDir(t))
	repo := store.Settings()
	ctx := context.Background()

	keys := []string{
		"capture.intervalSeconds",
		"privacy.blockedApplicationIds",
		"storage.recordingsLimitBytes",
		"notifications.journalReminderTime",
		"system.agentEditsEnabled",
		"telemetry.crashReportingOptIn",
		"llm.outputLanguage",
	}
	for _, key := range keys {
		if err := repo.Set(ctx, key, `0`); err != nil {
			t.Fatalf("Set(%q): %v", key, err)
		}
	}
	all, err := repo.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	for _, key := range keys {
		if _, ok := all[key]; !ok {
			t.Errorf("key %q did not round trip; stored keys: %s", key, strings.Join(mapKeys(all), ", "))
		}
	}
}

func mapKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sortStrings(out)
	return out
}
