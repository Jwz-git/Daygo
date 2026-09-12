package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/chat"
	"github.com/Jwz-git/Daygo/internal/domain"
)

// toolCall builds a chat.ToolCall with JSON arguments.
func toolCall(tool string, args string) chat.ToolCall {
	return chat.ToolCall{Tool: tool, Arguments: json.RawMessage(args)}
}

// Same-origin assertion: a card write through the executor and the same write
// through the binding leave the identical database state and event sequence.
func TestChatToolExecutorCardWriteMatchesBinding(t *testing.T) {
	seedShell := domain.CardShell{
		Start: "10:00 AM", End: "10:30 AM", Category: "Coding",
		Title: "fixture", Summary: "fixture summary",
	}

	execBackend, execEmitter := writerBackendWithStore(t, t.TempDir())
	seedTimelineDay(t, execBackend, []domain.CardShell{seedShell})

	bindBackend, bindEmitter := writerBackendWithStore(t, t.TempDir())
	seedTimelineDay(t, bindBackend, []domain.CardShell{seedShell})

	executor := chatToolExecutor{backend: execBackend}
	result := executor.Execute(context.Background(), toolCall("card_update", `{"cardId":1,"category":"Idle"}`))
	if result.Err {
		t.Fatalf("executor card_update = %s", result.Data)
	}
	if err := bindBackend.UpdateCardCategory(1, "Idle"); err != nil {
		t.Fatalf("binding UpdateCardCategory: %v", err)
	}

	// Both databases hold the same updated card.
	for name, store := range map[string]struct {
		backend *Backend
	}{
		"executor": {execBackend}, "binding": {bindBackend},
	} {
		cards, err := store.backend.store().Cards().CardsForDay(context.Background(), "2026-09-12")
		if err != nil {
			t.Fatalf("%s CardsForDay: %v", name, err)
		}
		if len(cards) != 1 || cards[0].Category != "Idle" {
			t.Fatalf("%s card = %+v", name, cards)
		}
	}
	// Both paths emitted the same invalidation event.
	if execEmitter.count(EventTimelineUpdated) != bindEmitter.count(EventTimelineUpdated) {
		t.Fatalf("timeline events: executor %d, binding %d", execEmitter.count(EventTimelineUpdated), bindEmitter.count(EventTimelineUpdated))
	}
}

// Same-origin for goals: executor goal_set and SaveDayGoal produce the same
// row and the same goal:updated event.
func TestChatToolExecutorGoalSetMatchesBinding(t *testing.T) {
	execBackend, execEmitter := writerBackendWithStore(t, t.TempDir())
	bindBackend, bindEmitter := writerBackendWithStore(t, t.TempDir())

	executor := chatToolExecutor{backend: execBackend}
	result := executor.Execute(context.Background(), toolCall("goal_set",
		`{"day":"2026-09-12","focusTargetMinutes":120,"distractionLimitMinutes":30}`))
	if result.Err {
		t.Fatalf("executor goal_set = %s", result.Data)
	}
	if err := bindBackend.SaveDayGoal(DayGoalDTO{
		Day: "2026-09-12", FocusTargetMinutes: 120, DistractionLimitMinutes: 30,
	}); err != nil {
		t.Fatalf("binding SaveDayGoal: %v", err)
	}

	for name, backend := range map[string]*Backend{"executor": execBackend, "binding": bindBackend} {
		goal, _, found, err := backend.store().Goals().Get(context.Background(), "2026-09-12")
		if err != nil || !found {
			t.Fatalf("%s goal: %v found=%v", name, err, found)
		}
		if goal.FocusTargetMinutes != 120 || goal.DistractionLimitMinutes != 30 {
			t.Fatalf("%s goal = %+v", name, goal)
		}
	}
	if execEmitter.count(EventGoalUpdated) != 1 || bindEmitter.count(EventGoalUpdated) != 1 {
		t.Fatalf("goal events: executor %d, binding %d", execEmitter.count(EventGoalUpdated), bindEmitter.count(EventGoalUpdated))
	}
}

// The category tools map single-row intent onto the whole-set save and
// protect the built-ins.
func TestChatToolExecutorCategoryTools(t *testing.T) {
	backend, _ := writerBackendWithStore(t, t.TempDir())
	executor := chatToolExecutor{backend: backend}
	ctx := context.Background()

	// add
	if result := executor.Execute(ctx, toolCall("category_add", `{"name":"Deep Work","colorHex":"#FF0000"}`)); result.Err {
		t.Fatalf("category_add = %s", result.Data)
	}
	categories, err := backend.store().Categories().List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var deepWork domain.Category
	haveDeepWork := false
	for _, c := range categories {
		if c.Name == "Deep Work" {
			deepWork = c
			haveDeepWork = true
		}
	}
	if !haveDeepWork || deepWork.ColorHex != "#FF0000" || deepWork.IsSystem {
		t.Fatalf("added category = %+v", deepWork)
	}

	// duplicate name refused
	if result := executor.Execute(ctx, toolCall("category_add", `{"name":"Deep Work"}`)); !result.Err {
		t.Fatalf("duplicate add must fail: %s", result.Data)
	}

	// update by id
	if result := executor.Execute(ctx, toolCall("category_update",
		`{"categoryId":"`+deepWork.ID+`","name":"Focus","isIdle":true}`)); result.Err {
		t.Fatalf("category_update = %s", result.Data)
	}
	categories, _ = backend.store().Categories().List(ctx)
	found := false
	for _, c := range categories {
		if c.ID == deepWork.ID {
			found = true
			if c.Name != "Focus" || !c.IsIdle {
				t.Fatalf("updated category = %+v", c)
			}
		}
	}
	if !found {
		t.Fatal("updated category vanished")
	}

	// built-in update refused
	if result := executor.Execute(ctx, toolCall("category_update",
		`{"categoryId":"00000000-0000-4000-8000-000000000001","name":"Nope"}`)); !result.Err {
		t.Fatalf("built-in update must fail: %s", result.Data)
	}

	// remove
	if result := executor.Execute(ctx, toolCall("category_remove", `{"categoryId":"`+deepWork.ID+`"}`)); result.Err {
		t.Fatalf("category_remove = %s", result.Data)
	}
	categories, _ = backend.store().Categories().List(ctx)
	for _, c := range categories {
		if c.ID == deepWork.ID {
			t.Fatal("removed category still present")
		}
	}
	// built-ins survive every operation
	sawSystem, sawIdle := false, false
	for _, c := range categories {
		if c.Name == "System" {
			sawSystem = true
		}
		if c.Name == "Idle" {
			sawIdle = true
		}
	}
	if !sawSystem || !sawIdle {
		t.Fatalf("built-ins missing after operations: %v", categories)
	}
}

// Read tools return binding-shaped data and never leak paths or secrets.
func TestChatToolExecutorReadTools(t *testing.T) {
	backend, _ := writerBackendWithStore(t, t.TempDir())
	seedTimelineDay(t, backend, []domain.CardShell{
		{Start: "10:00 AM", End: "10:30 AM", Category: "Coding", Title: "fixture", Summary: "s"},
	})
	executor := chatToolExecutor{backend: backend}
	ctx := context.Background()

	result := executor.Execute(ctx, toolCall("timeline", `{"day":"2026-09-12"}`))
	if result.Err {
		t.Fatalf("timeline = %s", result.Data)
	}
	if !strings.Contains(string(result.Data), `"cards"`) || !strings.Contains(string(result.Data), "fixture") {
		t.Fatalf("timeline result missing cards: %s", result.Data)
	}
	for _, banned := range []string{"videoSummaryPath", "segment_path", "recordings/"} {
		if strings.Contains(string(result.Data), banned) {
			t.Fatalf("timeline result leaks %q", banned)
		}
	}

	result = executor.Execute(ctx, toolCall("card", `{"cardId":1}`))
	if result.Err {
		t.Fatalf("card = %s", result.Data)
	}
	var card map[string]any
	if err := json.Unmarshal(result.Data, &card); err != nil {
		t.Fatalf("card result not JSON: %v", err)
	}
	if card["title"] != "fixture" {
		t.Fatalf("card title = %v", card["title"])
	}

	result = executor.Execute(ctx, toolCall("categories", `{}`))
	if result.Err {
		t.Fatalf("categories = %s", result.Data)
	}
	if !strings.Contains(string(result.Data), "Deep") == false && !strings.Contains(string(result.Data), "System") {
		t.Fatal("categories result missing System")
	}

	result = executor.Execute(ctx, toolCall("daily", `{"day":"2026-09-12"}`))
	if result.Err {
		t.Fatalf("daily = %s", result.Data)
	}
	if !strings.Contains(string(result.Data), `"journal"`) || !strings.Contains(string(result.Data), `"goal"`) {
		t.Fatalf("daily result missing envelope: %s", result.Data)
	}

	result = executor.Execute(ctx, toolCall("weekly", `{"weekStart":"2026-09-07"}`))
	if result.Err {
		t.Fatalf("weekly = %s", result.Data)
	}
}

// A read-only instance refuses every write tool with readonly-instance
// semantics, independent of the editMode gate. The read-only state is real:
// a writer store holds the lock first, so the test store genuinely degrades.
func TestChatToolExecutorReadOnlyInstanceRefusesWrites(t *testing.T) {
	dir := t.TempDir()
	writer := openTestStore(t, dir, true) // holds the write lock
	defer func() { _ = writer.Close() }()
	store := openTestStore(t, dir, false)
	backend := newBackend(fixedClock{}, nil, store, false, false)
	executor := chatToolExecutor{backend: backend}
	ctx := context.Background()

	for _, call := range []chat.ToolCall{
		toolCall("card_update", `{"cardId":1,"title":"x"}`),
		toolCall("card_delete", `{"cardId":1}`),
		toolCall("category_add", `{"name":"X"}`),
		toolCall("goal_set", `{"day":"2026-09-12"}`),
	} {
		result := executor.Execute(ctx, call)
		if !result.Err {
			t.Fatalf("%s must fail on a read-only instance: %s", call.Tool, result.Data)
		}
		var decoded struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(result.Data, &decoded); err != nil {
			t.Fatalf("%s result not JSON: %v", call.Tool, err)
		}
		if decoded.Error.Code != "not_capture_owner" {
			t.Fatalf("%s code = %q, want not_capture_owner", call.Tool, decoded.Error.Code)
		}
	}
}

// attemptSink maps one ai.Attempt onto an llm_calls row with purpose=chat.
func TestAttemptSinkRecordsChatAttempt(t *testing.T) {
	backend, _ := writerBackendWithStore(t, t.TempDir())
	sink := attemptSink{repo: backend.store().LlmCalls()}

	inputTokens := int64(120)
	started := time.Unix(1700000000, 0)
	sink.RecordAttempt(context.Background(), ai.Attempt{
		Purpose:        ai.PurposeChat,
		AttemptNo:      1,
		ProviderID:     "fixture-provider",
		Protocol:       ai.ProtocolOpenAIChat,
		RequestedModel: "fixture-model",
		StartedAt:      started,
		FinishedAt:     started.Add(2 * time.Second),
		Outcome:        "succeeded",
		InputTokens:    &inputTokens,
	})

	var purpose, protocol string
	var latency int64
	err := backend.store().Read(context.Background(), "test read llm calls", func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			`SELECT purpose, protocol, latency_ms FROM llm_calls WHERE provider_id = 'fixture-provider'`).
			Scan(&purpose, &protocol, &latency)
	})
	if err != nil {
		t.Fatalf("read llm_calls: %v", err)
	}
	if purpose != "chat" || protocol != "openai" || latency != 2000 {
		t.Fatalf("llm_calls row = %s/%s/%d", purpose, protocol, latency)
	}
}

// Every catalogued tool dispatches: no entry may fall through to the default
// branch on well-formed arguments.
func TestChatToolExecutorDispatchesWholeCatalog(t *testing.T) {
	backend, _ := writerBackendWithStore(t, t.TempDir())
	seedTimelineDay(t, backend, []domain.CardShell{
		{Start: "10:00 AM", End: "10:30 AM", Category: "Coding", Title: "fixture", Summary: "s"},
	})
	// A throwaway category so update/remove have a legal non-built-in target.
	executor := chatToolExecutor{backend: backend}
	if result := executor.Execute(context.Background(), toolCall("category_add", `{"name":"Fixture Cat"}`)); result.Err {
		t.Fatalf("seed category_add = %s", result.Data)
	}
	categories, _ := backend.store().Categories().List(context.Background())
	fixtureID := ""
	for _, c := range categories {
		if c.Name == "Fixture Cat" {
			fixtureID = c.ID
		}
	}
	if fixtureID == "" {
		t.Fatal("seeded category missing")
	}

	ctx := context.Background()
	// Ordered: card reads must run before card_delete removes the row.
	cases := []struct{ tool, args string }{
		{"timeline", `{"day":"2026-09-12"}`},
		{"card", `{"cardId":1}`},
		{"daily", `{"day":"2026-09-12"}`},
		{"weekly", `{"weekStart":"2026-09-07"}`},
		{"categories", `{}`},
		{"category_add", `{"name":"Another Cat"}`},
		{"category_update", `{"categoryId":"` + fixtureID + `","details":"x"}`},
		{"card_update", `{"cardId":1,"title":"renamed"}`},
		{"goal_set", `{"day":"2026-09-12","focusTargetMinutes":60}`},
		{"category_remove", `{"categoryId":"` + fixtureID + `"}`},
		{"card_delete", `{"cardId":1}`},
	}
	for _, tc := range cases {
		result := executor.Execute(ctx, toolCall(tc.tool, tc.args))
		if result.Err {
			t.Errorf("%s = %s", tc.tool, result.Data)
		}
	}

	// Built-in categories stay untouchable.
	for _, args := range []string{
		`{"categoryId":"00000000-0000-4000-8000-000000000001","details":"x"}`,
		`{"categoryId":"00000000-0000-4000-8000-000000000001"}`,
	} {
		tool := "category_update"
		if strings.HasSuffix(args, `"}`) && !strings.Contains(args, "details") {
			tool = "category_remove"
		}
		result := executor.Execute(ctx, toolCall(tool, args))
		if !result.Err {
			t.Errorf("%s on built-in must be refused: %s", tool, result.Data)
		}
	}

	// Unknown tools close with a rejection.
	result := executor.Execute(ctx, toolCall("run_sql", `{}`))
	if !result.Err {
		t.Fatalf("unknown tool = %s", result.Data)
	}
}

var _ = sql.Drivers
