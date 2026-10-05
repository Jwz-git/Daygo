package storage

import (
	"context"
	"testing"

	"github.com/Jwz-git/Daygo/internal/domain"
)

func userCategories(t *testing.T, store *Store) []domain.Category {
	t.Helper()
	rows, err := store.Categories().List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var cats []domain.Category
	for _, c := range rows {
		if !c.IsSystem {
			cats = append(cats, c)
		}
	}
	if len(cats) == 0 {
		t.Fatal("no user categories")
	}
	return cats
}

func TestCategorySavePreservesPlanReference(t *testing.T) {
	s := openWriter(t, newDir(t))
	ctx := context.Background()
	cats := userCategories(t, s)
	id, err := s.Plans().Insert(ctx, PlanBlock{Day: "2026-10-05", StartTs: 100, EndTs: 200, Title: "Anonymous plan", CategoryID: cats[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, rename := range []bool{false, true} {
		if rename {
			cats[0].Name = "Anonymous renamed category"
		}
		if err := s.Categories().Save(ctx, cats); err != nil {
			t.Fatal(err)
		}
		block, err := s.Plans().Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if block.CategoryID != cats[0].ID || block.CategoryName != cats[0].Name {
			t.Fatalf("category reference changed on save: %+v", block)
		}
	}
	if err := s.Categories().Save(ctx, cats[1:]); err != nil {
		t.Fatal(err)
	}
	block, err := s.Plans().Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if block.CategoryID != "" {
		t.Fatal("actual deletion did not clear plan category")
	}
}

func TestCategorySavePreservesGoalReferenceDuringSwap(t *testing.T) {
	s := openWriter(t, newDir(t))
	ctx := context.Background()
	cats := userCategories(t, s)
	err := s.Goals().Save(ctx, DayGoal{Day: "2026-10-05", FocusTargetMinutes: 60}, []GoalCategoryRef{{CategoryID: cats[0].ID, Role: GoalRoleFocus}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Categories().Save(ctx, cats); err != nil {
		t.Fatal(err)
	}
	cats[0].Name, cats[1].Name = cats[1].Name, cats[0].Name
	if err := s.Categories().Save(ctx, cats); err != nil {
		t.Fatal(err)
	}
	_, refs, found, err := s.Goals().Get(ctx, "2026-10-05")
	if err != nil || !found || len(refs) != 1 {
		t.Fatalf("goal refs=%+v, found=%v, err=%v", refs, found, err)
	}
	if refs[0].CategoryID != cats[0].ID || refs[0].Name != cats[0].Name {
		t.Fatalf("goal lost identity: %+v", refs)
	}
	// Existing policy: a category still referenced by a goal cannot be deleted.
	assertKind(t, s.Categories().Save(ctx, cats[1:]), KindConstraint)
	if got := listCategoryNames(t, s); len(got) != 8 {
		t.Fatalf("rejected deletion changed categories: %v", got)
	}
}

func TestCategorySaveRejectsDuplicateAndBuiltInIDs(t *testing.T) {
	s := openWriter(t, newDir(t))
	ctx := context.Background()
	cats := userCategories(t, s)
	cats[1].ID = cats[0].ID
	assertKind(t, s.Categories().Save(ctx, cats), KindConstraint)
	assertKind(t, s.Categories().Save(ctx, []domain.Category{{ID: "00000000-0000-4000-8000-000000000001", Name: "Forged", ColorHex: "#111111"}}), KindConstraint)
	if got := listCategoryNames(t, s); len(got) != 8 {
		t.Fatalf("invalid IDs changed categories: %v", got)
	}
}
