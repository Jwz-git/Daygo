package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/chat"
	"github.com/Jwz-git/Daygo/internal/platform/fake"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// planBackend is a read-write backend on a fresh store, a fake system and a
// clock fixed at 2026-09-12 09:00 local, with a recording emitter.
func planBackend(t *testing.T) (*Backend, *fake.System, *recordingEmitter) {
	t.Helper()
	system := fake.NewSystem()
	store := openTestStore(t, t.TempDir(), false)
	backend := newBackend(fixedClock{now: localAt(2026, 9, 12, 9, 0)}, system, store, true, true)
	emitter := &recordingEmitter{}
	backend.emitter = emitter
	return backend, system, emitter
}

func localAt(year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, time.Local)
}

// seedPlanCategory inserts a category directly (see seedCodingCard).
func seedPlanCategory(t *testing.T, b *Backend, id, name string) {
	t.Helper()
	err := b.store().Write(context.Background(), "seed plan category", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO categories (id, name, color_hex, sort_order, is_system, is_idle, created_at, updated_at)
			 VALUES (?, ?, '#4F8CFF', 0, 0, 0, 0, 0)`, id, name)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// seedCard inserts one live card of category between two local instants.
func seedCard(t *testing.T, b *Backend, category string, from, to time.Time) {
	t.Helper()
	err := b.store().Write(context.Background(), "seed plan card", func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO analysis_batches (id, start_ts, end_ts, status, created_at, updated_at)
			 VALUES (1, 0, 0, 'succeeded', 0, 0)`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO timeline_cards (batch_id, day, start, end, start_ts, end_ts, category, title, summary, created_at, updated_at)
			 VALUES (1, '2026-09-12', '', '', ?, ?, ?, 't', 's', 0, 0)`,
			from.Unix(), to.Unix(), category)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func notes(value string) *string { return &value }

func TestPlanBindingsRoundTrip(t *testing.T) {
	backend, _, emitter := planBackend(t)
	seedPlanCategory(t, backend, "cat-coding", "Coding")

	id, err := backend.SavePlanBlock(PlanBlockInputDTO{
		Day: "2026-09-12", Start: "9:30", End: "11:00", Title: "  Write API  ",
		Notes: notes("- endpoints\n- tests"), CategoryID: "cat-coding", Remind: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	late, err := backend.SavePlanBlock(PlanBlockInputDTO{Day: "2026-09-12", Start: "23:30", End: "04:00", Title: "Night review"})
	if err != nil {
		t.Fatal(err)
	}

	day, err := backend.GetPlanDay("2026-09-12")
	if err != nil {
		t.Fatal(err)
	}
	if len(day.Blocks) != 2 || day.Blocks[0].ID != id || day.Blocks[1].ID != late {
		t.Fatalf("blocks = %+v, want [%d %d]", day.Blocks, id, late)
	}
	first := day.Blocks[0]
	if first.Start != "09:30" || first.End != "11:00" || first.Title != "Write API" || first.CategoryName != "Coding" ||
		first.ColorHex != "#4F8CFF" || first.Status != storage.PlanStatusPlanned || !first.Remind {
		t.Fatalf("first block = %+v", first)
	}
	night := day.Blocks[1]
	if night.End != "04:00" || night.EndTs-night.StartTs != int64(4*3600+30*60) {
		t.Fatalf("night block = %s–%s (%d s), want 23:30 to the day's end", night.Start, night.End, night.EndTs-night.StartTs)
	}

	// Full replacement edit: move it, clear the category and the notes.
	if _, err := backend.SavePlanBlock(PlanBlockInputDTO{
		ID: id, Day: "2026-09-12", Start: "10:00", End: "12:00", Title: "Write API v2", Notes: notes("  "),
	}); err != nil {
		t.Fatal(err)
	}
	if err := backend.SetPlanBlockStatus(id, storage.PlanStatusDone); err != nil {
		t.Fatal(err)
	}
	day, _ = backend.GetPlanDay("2026-09-12")
	edited := day.Blocks[0]
	if edited.Start != "10:00" || edited.CategoryID != "" || edited.Notes != nil || edited.Status != storage.PlanStatusDone || edited.CompletedAtTs == nil {
		t.Fatalf("edited block = %+v", edited)
	}

	if err := backend.DeletePlanBlock(late); err != nil {
		t.Fatal(err)
	}
	day, _ = backend.GetPlanDay("2026-09-12")
	if len(day.Blocks) != 1 {
		t.Fatalf("blocks after delete = %d, want 1", len(day.Blocks))
	}
	if got := emitter.count(EventPlanUpdated); got != 5 {
		t.Fatalf("plan:updated emitted %d times, want one per write (5)", got)
	}
}

func TestPlanBindingsRejectBadInput(t *testing.T) {
	backend, _, _ := planBackend(t)
	for name, input := range map[string]PlanBlockInputDTO{
		"bad day":          {Day: "2026-9-12", Start: "09:00", End: "10:00", Title: "x"},
		"bad clock":        {Day: "2026-09-12", Start: "9am", End: "10:00", Title: "x"},
		"end before start": {Day: "2026-09-12", Start: "10:00", End: "09:00", Title: "x"},
		"blank title":      {Day: "2026-09-12", Start: "09:00", End: "10:00", Title: "   "},
		"unknown category": {Day: "2026-09-12", Start: "09:00", End: "10:00", Title: "x", CategoryID: "nope"},
	} {
		if _, err := backend.SavePlanBlock(input); !isCode(err, apperr.InvalidArgument) {
			t.Errorf("%s: err = %v, want invalid_argument", name, err)
		}
	}
	if err := backend.SetPlanBlockStatus(1, "finished"); !isCode(err, apperr.InvalidArgument) {
		t.Errorf("unknown status: err = %v, want invalid_argument", err)
	}
	if err := backend.SetPlanBlockStatus(99, storage.PlanStatusDone); !isCode(err, apperr.NotFound) {
		t.Errorf("missing block: err = %v, want not_found", err)
	}
	if _, err := backend.SavePlanBlock(PlanBlockInputDTO{ID: 99, Day: "2026-09-12", Start: "09:00", End: "10:00", Title: "x"}); !isCode(err, apperr.NotFound) {
		t.Errorf("edit missing block: err = %v, want not_found", err)
	}

	// A second instance on the same directory cannot take the write lock, so it
	// opens read-only at the connection layer (ownership comes from the lock,
	// not from the constructor flags).
	second := openTestStore(t, filepath.Dir(backend.store().Path()), false)
	readOnly := newBackend(fixedClock{now: localAt(2026, 9, 12, 9, 0)}, nil, second, false, false)
	if _, err := readOnly.SavePlanBlock(PlanBlockInputDTO{Day: "2026-09-12", Start: "09:00", End: "10:00", Title: "x"}); !isCode(err, apperr.NotCaptureOwner) {
		t.Errorf("read-only save: err = %v, want not_capture_owner", err)
	}
}

func isCode(err error, code apperr.Code) bool {
	var ae *apperr.Error
	return errors.As(err, &ae) && ae.Code == code
}

func TestPlanCoverageFromCards(t *testing.T) {
	backend, _, _ := planBackend(t)
	seedPlanCategory(t, backend, "cat-coding", "Coding")
	backend.clock = fixedClock{now: localAt(2026, 9, 12, 12, 0)}
	seedCard(t, backend, "Coding", localAt(2026, 9, 12, 9, 30), localAt(2026, 9, 12, 10, 30))
	seedCard(t, backend, "Distraction", localAt(2026, 9, 12, 10, 30), localAt(2026, 9, 12, 10, 50))
	if _, err := backend.SavePlanBlock(PlanBlockInputDTO{Day: "2026-09-12", Start: "10:00", End: "11:00", Title: "Code", CategoryID: "cat-coding"}); err != nil {
		t.Fatal(err)
	}
	day, err := backend.GetPlanDay("2026-09-12")
	if err != nil {
		t.Fatal(err)
	}
	if got := day.Blocks[0]; got.MatchedMinutes != 30 || got.DistractionMinutes != 20 {
		t.Fatalf("coverage = %v matched / %v distraction, want 30 / 20", got.MatchedMinutes, got.DistractionMinutes)
	}
}

func TestPlanToolsShareTheWritePath(t *testing.T) {
	backend, _, _ := planBackend(t)
	executor := chatToolExecutor{backend: backend}
	call := func(tool, args string) json.RawMessage {
		t.Helper()
		result := executor.Execute(context.Background(), chat.ToolCall{Tool: tool, Arguments: json.RawMessage(args)})
		if result.Err {
			t.Fatalf("%s %s = %s", tool, args, result.Data)
		}
		return result.Data
	}

	var added struct {
		OK      bool  `json:"ok"`
		BlockID int64 `json:"blockId"`
	}
	if err := json.Unmarshal(call(chat.ToolPlanAdd, `{"day":"2026-09-12","start":"14:00","end":"15:30","title":"Review PR","notes":"check tests"}`), &added); err != nil || !added.OK || added.BlockID == 0 {
		t.Fatalf("plan_add = %+v, %v", added, err)
	}
	call(chat.ToolPlanUpdate, `{"blockId":`+itoa(added.BlockID)+`,"title":"Review PR #12"}`)
	call(chat.ToolPlanComplete, `{"blockId":`+itoa(added.BlockID)+`}`)

	var day PlanDayDTO
	if err := json.Unmarshal(call(chat.ToolPlan, `{"day":"2026-09-12"}`), &day); err != nil {
		t.Fatal(err)
	}
	block := day.Blocks[0]
	if block.Title != "Review PR #12" || block.Start != "14:00" || block.End != "15:30" ||
		block.Notes == nil || *block.Notes != "check tests" || !block.Remind || block.Status != storage.PlanStatusDone {
		t.Fatalf("block after tools = %+v, want the partial update to keep times, notes and remind", block)
	}

	empty := executor.Execute(context.Background(), chat.ToolCall{Tool: chat.ToolPlanUpdate, Arguments: json.RawMessage(`{"blockId":` + itoa(added.BlockID) + `}`)})
	if !empty.Err {
		t.Fatal("plan_update with no fields succeeded; want invalid_argument")
	}

	// The agent socket handler validates against the same schema first.
	handler := agentWriteHandler{backend: backend}
	if _, err := handler.Execute(context.Background(), "plan_complete", json.RawMessage(`{"blockId":`+itoa(added.BlockID)+`,"status":"finished"}`)); err == nil {
		t.Fatal("socket plan_complete with an unknown status succeeded")
	}
	if _, err := handler.Execute(context.Background(), "plan_delete", json.RawMessage(`{"blockId":`+itoa(added.BlockID)+`}`)); err != nil {
		t.Fatalf("socket plan_delete: %v", err)
	}
	if day, _ := backend.GetPlanDay("2026-09-12"); len(day.Blocks) != 0 {
		t.Fatalf("blocks after socket delete = %d, want 0", len(day.Blocks))
	}
}

func itoa(value int64) string {
	data, _ := json.Marshal(value)
	return string(data)
}
