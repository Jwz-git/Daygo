package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/Jwz-git/Daygo/internal/domain"
)

func categoriesByName(t *testing.T, s *Store) map[string]domain.Category {
	t.Helper()
	rows, err := s.Categories().List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]domain.Category, len(rows))
	for _, c := range rows {
		out[c.Name] = c
	}
	return out
}

func TestMigrateV23FixtureBackfillsCategoryArchetypes(t *testing.T) {
	dir := newDir(t)
	copyFile(t, filepath.Join("testdata", "v23-category-archetype-source.db"), filepath.Join(dir, DatabaseFileName))
	s := openWriter(t, dir)
	if got := userVersionOf(t, s); got != schemaVersion() {
		t.Fatalf("user_version = %d, want %d", got, schemaVersion())
	}
	got := categoriesByName(t, s)
	for name, want := range map[string]string{
		"System": "order", "Idle": "camp",
		"Fixture Work": "forge", "Fixture Personal": "forge", // unknown names keep the default
		"Coding": "forge", " reading ": "sanctum", "MEETINGS": "nexus",
		"Break": "camp", "Admin": "order", "Side Project": "forge",
	} {
		c, ok := got[name]
		if !ok {
			t.Fatalf("category %q lost in the upgrade", name)
		}
		if c.Archetype != want {
			t.Fatalf("%q archetype = %q, want %q", name, c.Archetype, want)
		}
	}
	// The upgrade touches only the new column.
	if c := got["Coding"]; c.ColorHex != "#111111" || c.Details != "anonymous" || c.SortOrder != 3 {
		t.Fatalf("Coding changed in the upgrade: %+v", c)
	}
	// The schema itself keeps the set closed.
	err := s.Write(context.Background(), "bad archetype", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE categories SET archetype = 'castle' WHERE name = 'Coding'`)
		return err
	})
	if err == nil {
		t.Fatal("an archetype outside the closed set was stored")
	}
}

func TestFreshDatabaseSeedsStarterArchetypes(t *testing.T) {
	s := openWriter(t, newDir(t))
	got := categoriesByName(t, s)
	for name, want := range map[string]string{
		"System": "order", "Idle": "camp", "Focus Work": "forge", "Communication": "nexus",
		"Learning": "sanctum", "Research": "sanctum", "Distraction": "camp", "Personal": "camp",
	} {
		if got[name].Archetype != want {
			t.Fatalf("%q archetype = %q, want %q", name, got[name].Archetype, want)
		}
	}
}

func TestCategorySaveValidatesArchetype(t *testing.T) {
	s := openWriter(t, newDir(t))
	ctx := context.Background()
	cats := userCategories(t, s)
	cats[0].Archetype = ""
	cats[1].Archetype = "nexus"
	if err := s.Categories().Save(ctx, cats); err != nil {
		t.Fatal(err)
	}
	got := categoriesByName(t, s)
	if got[cats[0].Name].Archetype != "forge" || got[cats[1].Name].Archetype != "nexus" {
		t.Fatalf("archetypes after save: %q, %q", got[cats[0].Name].Archetype, got[cats[1].Name].Archetype)
	}
	cats[0].Archetype = "castle"
	assertKind(t, s.Categories().Save(ctx, cats), KindConstraint)
}

func TestApplyPresetReplaceKeepsReferencedCategories(t *testing.T) {
	s := openWriter(t, newDir(t))
	ctx := context.Background()
	before := categoriesByName(t, s)
	// Learning holds a card, Research a plan block, Personal a day goal.
	if err := s.Write(ctx, "seed references", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO timeline_cards
			(day, start, end, start_ts, end_ts, category, title, summary, created_at, updated_at)
			VALUES ('2026-10-05', '10:00 AM', '10:30 AM', 100, 1900, 'Learning', 'anonymous', 'anonymous', 1, 1)`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Plans().Insert(ctx, PlanBlock{Day: "2026-10-05", StartTs: 100, EndTs: 200, Title: "Anonymous plan",
		CategoryID: before["Research"].ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.Goals().Save(ctx, DayGoal{Day: "2026-10-05", FocusTargetMinutes: 60},
		[]GoalCategoryRef{{CategoryID: before["Personal"].ID, Role: GoalRoleFocus}}); err != nil {
		t.Fatal(err)
	}

	preset := []domain.Category{
		{Name: "Coding", ColorHex: "#6366F1", Details: "d1", Archetype: "forge"},
		{Name: "communication", ColorHex: "#06B6D4", Details: "d2", Archetype: "nexus"},
		{Name: "Break", ColorHex: "#EC4899", Details: "d3", Archetype: "camp"},
	}
	if err := s.Categories().ApplyPreset(ctx, preset); err != nil {
		t.Fatal(err)
	}
	got := categoriesByName(t, s)
	want := []string{"Coding", "Communication", "Break", "Learning", "Research", "Personal", "System", "Idle"}
	if len(got) != len(want) {
		t.Fatalf("categories after replace = %v, want %v", got, want)
	}
	for i, name := range want[:6] {
		c, ok := got[name]
		if !ok {
			t.Fatalf("%q missing after replace", name)
		}
		if c.SortOrder != i+1 {
			t.Fatalf("%q sort order = %d, want %d (preset first, kept rows after)", name, c.SortOrder, i+1)
		}
	}
	if got["Communication"].ID != before["Communication"].ID {
		t.Fatal("matched category lost its id")
	}
	if got["Research"].ID != before["Research"].ID || got["Personal"].ID != before["Personal"].ID {
		t.Fatal("referenced categories were recreated")
	}
}

func TestApplyPresetRejectsBadInputWithoutWriting(t *testing.T) {
	s := openWriter(t, newDir(t))
	ctx := context.Background()
	before := categoriesByName(t, s)
	ok := domain.Category{Name: "Fine", ColorHex: "#123456", Archetype: "forge"}
	for name, rows := range map[string][]domain.Category{
		"empty":     nil,
		"blank":     {{Name: "  ", ColorHex: "#123456"}},
		"duplicate": {ok, {Name: "fine", ColorHex: "#123456"}},
		"color":     {ok, {Name: "Other", ColorHex: "red"}},
		"archetype": {ok, {Name: "Other", ColorHex: "#123456", Archetype: "castle"}},
		"built-in":  {ok, {Name: "idle", ColorHex: "#123456"}},
	} {
		assertKind(t, s.Categories().ApplyPreset(ctx, rows), KindConstraint)
		if got := categoriesByName(t, s); len(got) != len(before) {
			t.Fatalf("%s: rejected preset changed the set: %v", name, got)
		}
	}
}

func TestApplyPaletteCyclesUserCategoriesInOrder(t *testing.T) {
	s := openWriter(t, newDir(t))
	ctx := context.Background()
	palette := []string{"#4A7C59", "#8FC0A9", "#C8D5B9", "#D9AE61"}
	if err := s.Categories().ApplyPalette(ctx, palette); err != nil {
		t.Fatal(err)
	}
	got := categoriesByName(t, s)
	// Starter order: Focus Work, Communication, Learning, Research, Distraction, Personal.
	for name, want := range map[string]string{
		"Focus Work": "#4A7C59", "Communication": "#8FC0A9", "Learning": "#C8D5B9",
		"Research": "#D9AE61", "Distraction": "#4A7C59", "Personal": "#8FC0A9",
		"System": "#8E8E93", "Idle": "#C7C7CC",
	} {
		if got[name].ColorHex != want {
			t.Fatalf("%q color = %q, want %q", name, got[name].ColorHex, want)
		}
	}
	assertKind(t, s.Categories().ApplyPalette(ctx, nil), KindConstraint)
	assertKind(t, s.Categories().ApplyPalette(ctx, []string{"#4A7C59", "#zzzzzz"}), KindConstraint)
	if got := categoriesByName(t, s); got["Focus Work"].ColorHex != "#4A7C59" {
		t.Fatal("rejected palette changed a color")
	}
}
