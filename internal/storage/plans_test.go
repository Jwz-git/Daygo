package storage

import (
	"context"
	"database/sql"
	"testing"
)

func strPtr(value string) *string { return &value }

func TestPlanBlocksForDayAreOrderedAndJoinTheirCategory(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	ctx := context.Background()
	seedGoalCategory(t, store, "cat-coding", "Coding")

	later, err := store.Plans().Insert(ctx, PlanBlock{
		Day: "2026-09-12", StartTs: 1789560000, EndTs: 1789563600, Title: "Review", Remind: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	earlier, err := store.Plans().Insert(ctx, PlanBlock{
		Day: "2026-09-12", StartTs: 1789552800, EndTs: 1789556400, Title: "Write API",
		Notes: strPtr("- endpoints\n- tests"), CategoryID: "cat-coding",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Plans().Insert(ctx, PlanBlock{
		Day: "2026-09-13", StartTs: 1789639200, EndTs: 1789642800, Title: "Other day",
	}); err != nil {
		t.Fatal(err)
	}

	blocks, err := store.Plans().ForDay(ctx, "2026-09-12")
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 || blocks[0].ID != earlier || blocks[1].ID != later {
		t.Fatalf("blocks = %+v, want [%d %d] by start time", blocks, earlier, later)
	}
	first := blocks[0]
	if first.CategoryName != "Coding" || first.CategoryColor != "#123456" || first.Notes == nil || *first.Notes != "- endpoints\n- tests" {
		t.Fatalf("first block = %+v, want its category joined and notes kept", first)
	}
	if first.Status != PlanStatusPlanned || first.CompletedAt != nil || first.Remind {
		t.Fatalf("first block status = %q completed %v remind %v, want planned / nil / false", first.Status, first.CompletedAt, first.Remind)
	}
	if !blocks[1].Remind || blocks[1].CategoryID != "" {
		t.Fatalf("second block = %+v, want remind on and no category", blocks[1])
	}

	empty, err := store.Plans().ForDay(ctx, "2026-09-20")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty day = %v, %v; want a non-nil empty slice", empty, err)
	}
}

func TestPlanBlockUpdateStatusAndDelete(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	ctx := context.Background()
	seedGoalCategory(t, store, "cat-coding", "Coding")

	id, err := store.Plans().Insert(ctx, PlanBlock{Day: "2026-09-12", StartTs: 100, EndTs: 200, Title: "Draft"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Plans().Update(ctx, PlanBlock{
		ID: id, Day: "2026-09-12", StartTs: 150, EndTs: 300, Title: "Draft v2",
		Notes: strPtr("details"), CategoryID: "cat-coding", Remind: true,
	}); err != nil {
		t.Fatal(err)
	}
	block, err := store.Plans().Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if block.StartTs != 150 || block.EndTs != 300 || block.Title != "Draft v2" || block.CategoryID != "cat-coding" || !block.Remind {
		t.Fatalf("updated block = %+v", block)
	}

	if err := store.Plans().SetStatus(ctx, id, PlanStatusDone); err != nil {
		t.Fatal(err)
	}
	block, _ = store.Plans().Get(ctx, id)
	if block.Status != PlanStatusDone || block.CompletedAt == nil {
		t.Fatalf("done block = %+v, want completed_at stamped", block)
	}
	if err := store.Plans().SetStatus(ctx, id, PlanStatusPlanned); err != nil {
		t.Fatal(err)
	}
	block, _ = store.Plans().Get(ctx, id)
	if block.Status != PlanStatusPlanned || block.CompletedAt != nil {
		t.Fatalf("reopened block = %+v, want completed_at cleared", block)
	}

	if err := store.Plans().Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	for name, err := range map[string]error{
		"get":    func() error { _, err := store.Plans().Get(ctx, id); return err }(),
		"update": store.Plans().Update(ctx, PlanBlock{ID: id, Day: "2026-09-12", StartTs: 1, EndTs: 2, Title: "x"}),
		"status": store.Plans().SetStatus(ctx, id, PlanStatusDone),
		"delete": store.Plans().Delete(ctx, id),
	} {
		if !IsKind(err, KindNotFound) {
			t.Fatalf("%s on a deleted block = %v, want not_found", name, err)
		}
	}
}

func TestPlanBlockRejectsInvalidRowsAndKeepsBlockWhenCategoryGoes(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	ctx := context.Background()
	seedGoalCategory(t, store, "cat-gone", "Gone")

	if _, err := store.Plans().Insert(ctx, PlanBlock{Day: "2026-09-12", StartTs: 200, EndTs: 200, Title: "Zero"}); err == nil {
		t.Fatal("a zero-length block was accepted; want the end > start check to refuse it")
	}
	if err := store.Plans().SetStatus(ctx, 1, "finished"); err == nil {
		t.Fatal("an unknown status was accepted")
	}

	id, err := store.Plans().Insert(ctx, PlanBlock{Day: "2026-09-12", StartTs: 100, EndTs: 200, Title: "Keep", CategoryID: "cat-gone"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Write(ctx, "drop category", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM categories WHERE id = 'cat-gone'`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	block, err := store.Plans().Get(ctx, id)
	if err != nil {
		t.Fatalf("block lost with its category: %v", err)
	}
	if block.CategoryID != "" || block.CategoryName != "" {
		t.Fatalf("block = %+v, want its category cleared, not the block deleted", block)
	}
}
