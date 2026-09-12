package storage

import (
	"context"
	"database/sql"
	"testing"
)

// seedGoalCategory inserts one categories row directly: CategoryRepo.Save
// replaces the whole non-built-in set, which is the wrong tool for seeding
// two rows alongside the built-ins in one transaction with FK-enforcing
// goal refs.
func seedGoalCategory(t *testing.T, store *Store, id, name string) {
	t.Helper()
	err := store.Write(context.Background(), "seed goal category", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO categories (id, name, color_hex, sort_order, is_system, is_idle, created_at, updated_at)
			 VALUES (?, ?, '#123456', 0, 0, 0, 0, 0)`, id, name)
		return err
	})
	if err != nil {
		t.Fatalf("seed goal category %s: %v", name, err)
	}
}

func TestGoalSaveReplacesCategoryRefs(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	ctx := context.Background()

	seedGoalCategory(t, store, "cat-coding", "Coding")
	seedGoalCategory(t, store, "cat-writing", "Writing")

	if _, _, found, err := store.Goals().Get(ctx, "2026-09-12"); err != nil || found {
		t.Fatalf("get before save = found %v, err %v; want not found", found, err)
	}

	if err := store.Goals().Save(ctx, DayGoal{
		Day: "2026-09-12", FocusTargetMinutes: 240, DistractionLimitMinutes: 30,
	}, []GoalCategoryRef{
		{CategoryID: "cat-coding", Name: "Coding", Role: GoalRoleFocus},
		{CategoryID: "cat-writing", Name: "Writing", Role: GoalRoleDistraction},
	}); err != nil {
		t.Fatalf("first save: %v", err)
	}

	goal, refs, found, err := store.Goals().Get(ctx, "2026-09-12")
	if err != nil || !found {
		t.Fatalf("get after save = found %v, err %v", found, err)
	}
	if goal.FocusTargetMinutes != 240 || goal.DistractionLimitMinutes != 30 || goal.IsSkipped {
		t.Fatalf("goal = %+v, want 240/30 not skipped", goal)
	}
	if len(refs) != 2 || refs[0].CategoryID != "cat-coding" || refs[0].Role != GoalRoleFocus ||
		refs[1].Role != GoalRoleDistraction {
		t.Fatalf("refs = %+v, want Coding(focus) then Writing(distraction)", refs)
	}

	// Second save fully replaces the category set.
	if err := store.Goals().Save(ctx, DayGoal{
		Day: "2026-09-12", FocusTargetMinutes: 120, DistractionLimitMinutes: 0, IsSkipped: true,
	}, []GoalCategoryRef{
		{CategoryID: "cat-writing", Name: "Writing", Role: GoalRoleFocus},
	}); err != nil {
		t.Fatalf("second save: %v", err)
	}
	goal, refs, found, err = store.Goals().Get(ctx, "2026-09-12")
	if err != nil || !found {
		t.Fatalf("get after replace = found %v, err %v", found, err)
	}
	if !goal.IsSkipped || goal.FocusTargetMinutes != 120 {
		t.Fatalf("goal after replace = %+v, want 120 minutes skipped", goal)
	}
	if len(refs) != 1 || refs[0].CategoryID != "cat-writing" || refs[0].Role != GoalRoleFocus {
		t.Fatalf("refs after replace = %+v, want only Writing(focus)", refs)
	}
}
