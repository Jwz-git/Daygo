package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrateV22FixturePreservesPotentialHumanCards(t *testing.T) {
	dir := t.TempDir()
	copyFile(t, filepath.Join("testdata", "v22-human-card-source.db"), filepath.Join(dir, DatabaseFileName))
	s := openWriter(t, dir)
	ctx := context.Background()
	if version := userVersionOf(t, s); version != schemaVersion() {
		t.Fatalf("schema version=%d, want %d", version, schemaVersion())
	}
	for _, want := range []struct {
		id     int64
		edited int
	}{{900, 1}, {901, 1}, {902, 0}} {
		var edited int
		if err := s.Read(ctx, "fixture protection", func(ctx context.Context, tx *sql.Tx) error {
			return tx.QueryRowContext(ctx, "SELECT is_user_edited FROM timeline_cards WHERE id=?", want.id).Scan(&edited)
		}); err != nil {
			t.Fatal(err)
		}
		if edited != want.edited {
			t.Fatalf("card %d edited=%d, want %d", want.id, edited, want.edited)
		}
	}
	card, err := s.Cards().CardByID(ctx, 900)
	if err != nil || card.Title != "anonymous live" || card.StartTs != 1789178400 || card.EndTs != 1789179300 {
		t.Fatalf("old card changed: %+v err=%v", card, err)
	}
	totals, err := s.Reviews().TotalsByDay(ctx, "2026-09-12")
	if err != nil || totals.FocusMinutes != 15 {
		t.Fatalf("review totals=%+v err=%v", totals, err)
	}
	rating, err := s.Reviews().Rating(ctx, 901)
	if err != nil || rating != RatingUp {
		t.Fatalf("rating=%q err=%v", rating, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openWriter(t, dir)
	var edited int
	if err := s.Read(ctx, "fixture restart", func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, "SELECT is_user_edited FROM timeline_cards WHERE id=900").Scan(&edited)
	}); err != nil || edited != 1 {
		t.Fatalf("restart protection=%d err=%v", edited, err)
	}
}
