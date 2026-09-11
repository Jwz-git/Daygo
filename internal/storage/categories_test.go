package storage

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Jwz-git/Daygo/internal/domain"
)

func listCategoryNames(t *testing.T, store *Store) []string {
	t.Helper()
	cats, err := store.Categories().List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	names := make([]string, 0, len(cats))
	for _, c := range cats {
		names = append(names, c.Name)
	}
	return names
}

func TestCategoriesSeededAfterFreshOpen(t *testing.T) {
	store := openWriter(t, newDir(t))

	cats, err := store.Categories().List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(cats) != 2 {
		t.Fatalf("categories = %v, want System and Idle only", listCategoryNames(t, store))
	}
	byName := make(map[string]domain.Category, len(cats))
	for _, c := range cats {
		byName[c.Name] = c
	}
	if !byName["System"].IsSystem || byName["System"].IsIdle {
		t.Fatalf("System flags wrong: %+v", byName["System"])
	}
	if !byName["Idle"].IsSystem || !byName["Idle"].IsIdle {
		t.Fatalf("Idle flags wrong: %+v", byName["Idle"])
	}
}

func TestCategorySaveOverwritesUserSet(t *testing.T) {
	store := openWriter(t, newDir(t))
	ctx := context.Background()

	err := store.Categories().Save(ctx, []domain.Category{
		{Name: "Coding", ColorHex: "#FF0000", SortOrder: 1},
		{Name: "Writing", ColorHex: "#00FF00", SortOrder: 2},
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Whole-set overwrite: dropping Writing removes it.
	err = store.Categories().Save(ctx, []domain.Category{
		{Name: "Coding", ColorHex: "#FF0000", SortOrder: 1},
	})
	if err != nil {
		t.Fatalf("Save second: %v", err)
	}
	// Order is sort_order (0 for built-ins) then name: Idle < System < Coding.
	names := listCategoryNames(t, store)
	if len(names) != 3 || names[0] != "Idle" || names[1] != "System" || names[2] != "Coding" {
		t.Fatalf("names = %v, want [Idle System Coding]", names)
	}

	// A save with no user rows leaves the built-ins alone.
	if err := store.Categories().Save(ctx, nil); err != nil {
		t.Fatalf("Save empty: %v", err)
	}
	if got := listCategoryNames(t, store); len(got) != 2 {
		t.Fatalf("names = %v, want only built-ins after empty save", got)
	}
}

func TestCategorySaveRejectsDuplicateNames(t *testing.T) {
	store := openWriter(t, newDir(t))
	ctx := context.Background()

	err := store.Categories().Save(ctx, []domain.Category{
		{Name: "Coding", ColorHex: "#FF0000"},
		{Name: "Coding", ColorHex: "#00FF00"},
	})
	assertKind(t, err, KindConstraint)

	// The rejected save must have changed nothing.
	if got := listCategoryNames(t, store); len(got) != 2 {
		t.Fatalf("names = %v after rejected save; transaction leaked rows", got)
	}
}

func TestCategorySaveRejectsEmptyNameAndCallerClaimedSystem(t *testing.T) {
	store := openWriter(t, newDir(t))
	ctx := context.Background()

	if err := store.Categories().Save(ctx, []domain.Category{{ColorHex: "#FF0000"}}); err == nil {
		t.Fatal("Save accepted an empty name")
	}
	if err := store.Categories().Save(ctx, []domain.Category{
		{Name: "FakeSystem", IsSystem: true},
	}); err == nil {
		t.Fatal("Save accepted a caller-claimed is_system row")
	}
}

func TestCategorySaveRenamesRewriteCardsInSameTransaction(t *testing.T) {
	store := openWriter(t, newDir(t))
	ctx := context.Background()

	seeded := store.Categories().Save(ctx, []domain.Category{
		{Name: "Coding", ColorHex: "#FF0000"},
	})
	if seeded != nil {
		t.Fatalf("Save: %v", seeded)
	}
	cats, err := store.Categories().List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var coding domain.Category
	for _, c := range cats {
		if c.Name == "Coding" {
			coding = c
		}
	}
	if coding.ID == "" {
		t.Fatal("Coding not found after save")
	}

	// A card stored under the old name. The cards repository arrives in the
	// next slice; seeding the row through the store's write wrapper keeps this
	// test on the storage layer's own surface.
	err = store.Write(ctx, "seed card", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO timeline_cards (batch_id, day, start, end, start_ts, end_ts, category, title, summary, created_at, updated_at)
			VALUES (NULL, '2026-09-11', '10:00 AM', '10:30 AM', 1000, 1800, 'Coding', 't', 's', 0, 0)`)
		return err
	})
	if err != nil {
		t.Fatalf("seed card: %v", err)
	}

	// Rename through Save with the same id.
	err = store.Categories().Save(ctx, []domain.Category{
		{ID: coding.ID, Name: "Engineering", ColorHex: "#FF0000"},
	})
	if err != nil {
		t.Fatalf("Save rename: %v", err)
	}
	if _, ok, _ := store.Categories().ByName(ctx, "Coding"); ok {
		t.Fatal("Coding still exists after rename")
	}
	if _, ok, _ := store.Categories().ByName(ctx, "Engineering"); !ok {
		t.Fatal("Engineering missing after rename")
	}

	// The card's category string must have followed the rename, in the same
	// transaction — this is the assertion the whole method exists for.
	var cardCategory string
	err = store.Read(ctx, "read card", func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			"SELECT category FROM timeline_cards WHERE title = 't'").Scan(&cardCategory)
	})
	if err != nil {
		t.Fatalf("read card: %v", err)
	}
	if cardCategory != "Engineering" {
		t.Fatalf("card category = %q after rename, want Engineering", cardCategory)
	}
}
