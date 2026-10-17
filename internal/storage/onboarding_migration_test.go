package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func onboardingValue(t *testing.T, s *Store) (string, bool) {
	t.Helper()
	var value string
	err := s.Read(context.Background(), "onboarding setting", func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = 'system.onboardingCompleted'`).Scan(&value)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return value, true
}

func TestMigrateV24FixtureMarksOnboardingDone(t *testing.T) {
	dir := newDir(t)
	copyFile(t, filepath.Join("testdata", "v24-onboarding-source.db"), filepath.Join(dir, DatabaseFileName))
	s := openWriter(t, dir)
	if got := userVersionOf(t, s); got != schemaVersion() {
		t.Fatalf("user_version = %d, want %d", got, schemaVersion())
	}
	// The fixture holds providers and cards but no setting: data alone marks it.
	if value, ok := onboardingValue(t, s); !ok || value != "true" {
		t.Fatalf("onboarding setting = %q (present %v), want true", value, ok)
	}
	if got := rowCount(t, s, "app_settings"); got != 1 {
		t.Fatalf("app_settings rows = %d, want only the onboarding mark", got)
	}
	// The archetypes the fixture carried survive.
	if got := categoriesByName(t, s); got["MEETINGS"].Archetype != "nexus" || got["System"].Archetype != "order" {
		t.Fatalf("archetypes changed in the upgrade: %+v", got)
	}
}

func TestFreshDatabaseLeavesOnboardingUnset(t *testing.T) {
	dir := newDir(t)
	s := openWriter(t, dir)
	if value, ok := onboardingValue(t, s); ok {
		t.Fatalf("fresh database has onboarding = %q, want unset so the guide shows", value)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	// Reopening re-runs nothing, so a fresh install stays unmarked until the
	// guide writes the setting.
	s = openWriter(t, dir)
	if _, ok := onboardingValue(t, s); ok {
		t.Fatal("reopen marked onboarding done")
	}
}
