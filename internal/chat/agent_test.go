package chat

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
)

// fakeExecutor records calls and answers with scripted envelopes.
type fakeExecutor struct {
	mu     sync.Mutex
	calls  []ToolCall
	result func(ToolCall) ToolResult
}

func (e *fakeExecutor) Execute(_ context.Context, call ToolCall) ToolResult {
	e.mu.Lock()
	e.calls = append(e.calls, call)
	result := e.result(call)
	e.mu.Unlock()
	return result
}

func (e *fakeExecutor) callCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.calls)
}

func okResult(data string) ToolResult {
	return ToolResult{Data: json.RawMessage(data)}
}

// agentService wires a service with a tool executor and an openai-protocol
// test server whose reply sequence is scripted (repeating the last reply).
func agentService(t *testing.T, editMode string, executor *fakeExecutor, replies ...string) (*Service, *fakeStore, *fakeExecutor) {
	t.Helper()
	var mu sync.Mutex
	index := 0
	server := newOpenAIServer(t, func(string) string {
		mu.Lock()
		defer mu.Unlock()
		if index >= len(replies) {
			index = len(replies) - 1
		}
		reply := replies[index]
		index++
		return reply
	})
	providers := newFakeProviders(ProviderEntry{
		ID: "p1", Protocol: "openai", Endpoint: server.URL, Model: "m", Secret: "sk-agent-test",
	})
	if executor == nil {
		executor = &fakeExecutor{result: func(ToolCall) ToolResult { return okResult(`{"ok":true,"data":{}}`) }}
	}
	store := newFakeStore(t)
	service := New(store, providers, &fakeSettings{editMode: editMode}, executor, nil)
	service.SetNotifier(func(string) {})
	return service, store, executor
}

// The happy path: one tool call, one result, then an answer. The transcript
// must read user / tool_call / tool_result / assistant(ok), with the tool
// rows paired by name.
func TestAgentLoopHappyPath(t *testing.T) {
	service, _, executor := agentService(t, "edits", nil,
		`{"kind":"tool","tool":"timeline","arguments":{"day":"2026-09-12"}}`,
		`{"kind":"answer","answer":"今天你主要在写代码。"}`)

	conversation, _ := service.NewConversation(context.Background())
	if err := service.Send(context.Background(), conversation.ID, "我今天做了什么？"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitTurn(t, service, conversation.ID)

	messages, err := service.Messages(context.Background(), conversation.ID, 0, 0)
	if err != nil {
		t.Fatalf("Messages: %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("messages = %d, want 4 (user, tool_call, tool_result, assistant): %+v", len(messages), messages)
	}
	call, result, answer := messages[1], messages[2], messages[3]
	if call.Role != RoleToolCall || call.ToolName != "timeline" ||
		call.ToolArguments != `{"day":"2026-09-12"}` || call.Content != "" {
		t.Fatalf("tool_call row = %+v", call)
	}
	if result.Role != RoleToolRes || result.ToolName != "timeline" || result.ToolArguments != "" {
		t.Fatalf("tool_result row = %+v", result)
	}
	if answer.Role != RoleAssistant || answer.Status != StatusOK || answer.Content != "今天你主要在写代码。" {
		t.Fatalf("answer row = %+v", answer)
	}
	if executor.callCount() != 1 {
		t.Fatalf("executor calls = %d, want 1", executor.callCount())
	}
}

// The sandbox gate: in readonly mode a write tool is refused as a tool result
// and the turn continues — the model answers after seeing the refusal.
func TestAgentLoopReadonlyGateRefusesWriteTool(t *testing.T) {
	service, _, executor := agentService(t, "readonly", nil,
		`{"kind":"tool","tool":"card_update","arguments":{"cardId":7,"category":"Coding"}}`,
		`{"kind":"answer","answer":"当前是只读模式，无法修改卡片。"}`)

	conversation, _ := service.NewConversation(context.Background())
	if err := service.Send(context.Background(), conversation.ID, "把卡片改成 Coding"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitTurn(t, service, conversation.ID)

	messages, _ := service.Messages(context.Background(), conversation.ID, 0, 0)
	if len(messages) != 4 {
		t.Fatalf("messages = %d, want 4: %+v", len(messages), messages)
	}
	result := messages[2]
	var decoded struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.Content), &decoded); err != nil {
		t.Fatalf("tool_result content is not JSON: %v (%s)", err, result.Content)
	}
	if decoded.OK || decoded.Error.Code != "edits_disabled" {
		t.Fatalf("gate refusal = %+v", decoded)
	}
	if executor.callCount() != 0 {
		t.Fatalf("executor calls = %d, want 0 (gate must refuse before execution)", executor.callCount())
	}
	if messages[3].Status != StatusOK {
		t.Fatalf("turn must continue to an answer, got %+v", messages[3])
	}
}

// Unknown tools and malformed arguments are closed errors fed back to the
// model; the turn survives both.
func TestAgentLoopUnknownToolAndBadArguments(t *testing.T) {
	service, _, _ := agentService(t, "edits", nil,
		`{"kind":"tool","tool":"run_sql","arguments":{"q":"drop table"}}`,
		`{"kind":"tool","tool":"timeline","arguments":{"day":"not-a-date"}}`,
		`{"kind":"answer","answer":"好的。"}`)

	conversation, _ := service.NewConversation(context.Background())
	_ = service.Send(context.Background(), conversation.ID, "查一下")
	waitTurn(t, service, conversation.ID)

	messages, _ := service.Messages(context.Background(), conversation.ID, 0, 0)
	if len(messages) != 6 {
		t.Fatalf("messages = %d, want 6 (user, 2×(call,result), answer): %+v", len(messages), messages)
	}
	for _, i := range []int{2, 4} {
		var decoded struct {
			OK    bool `json:"ok"`
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(messages[i].Content), &decoded); err != nil {
			t.Fatalf("result %d not JSON: %v", i, err)
		}
		if decoded.OK {
			t.Fatalf("result %d must be an error envelope", i)
		}
	}
	wantCodes := []string{"unknown_tool", "invalid_argument"}
	for i, code := range wantCodes {
		var decoded struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal([]byte(messages[2+i*2].Content), &decoded)
		if decoded.Error.Code != code {
			t.Fatalf("result %d code = %q, want %q", i, decoded.Error.Code, code)
		}
	}
}

// The ninth tool call is recorded but not executed; the turn ends failed.
func TestAgentLoopBudgetExceeded(t *testing.T) {
	replies := make([]string, 0, maxToolCallsPerTurn+2)
	for range maxToolCallsPerTurn {
		replies = append(replies, `{"kind":"tool","tool":"categories","arguments":{}}`)
	}
	// The 9th request also asks for a tool; it must not execute.
	replies = append(replies, `{"kind":"tool","tool":"categories","arguments":{}}`)
	service, _, executor := agentService(t, "edits", nil, replies...)

	conversation, _ := service.NewConversation(context.Background())
	_ = service.Send(context.Background(), conversation.ID, "列一下分类")
	waitTurn(t, service, conversation.ID)

	if executor.callCount() != maxToolCallsPerTurn {
		t.Fatalf("executor calls = %d, want %d", executor.callCount(), maxToolCallsPerTurn)
	}
	messages, _ := service.Messages(context.Background(), conversation.ID, 0, 0)
	// user + 8×(call,result) + 9th call + budget result + failed assistant
	if len(messages) != 1+2*maxToolCallsPerTurn+3 {
		t.Fatalf("messages = %d, want %d: %+v", len(messages), 1+2*maxToolCallsPerTurn+3, messages)
	}
	last := messages[len(messages)-1]
	if last.Role != RoleAssistant || last.Status != StatusFailed ||
		!strings.Contains(last.Content, "上限") {
		t.Fatalf("final assistant row = %+v", last)
	}
	budgetResult := messages[len(messages)-2]
	if !strings.Contains(budgetResult.Content, "budget_exceeded") {
		t.Fatalf("budget result = %s", budgetResult.Content)
	}
}

// A tool result over 64 KiB is re-packed as a truncated but parseable JSON
// envelope.
func TestAgentLoopClipsOversizedToolResult(t *testing.T) {
	big := strings.Repeat("x", 100*1024)
	executor := &fakeExecutor{result: func(ToolCall) ToolResult {
		return ToolResult{Data: json.RawMessage(`{"ok":true,"data":"` + big + `"}`)}
	}}
	service, _, _ := agentService(t, "edits", executor,
		`{"kind":"tool","tool":"timeline","arguments":{"day":"2026-09-12"}}`,
		`{"kind":"answer","answer":"数据太大了。"}`)

	conversation, _ := service.NewConversation(context.Background())
	_ = service.Send(context.Background(), conversation.ID, "看看")
	waitTurn(t, service, conversation.ID)

	messages, _ := service.Messages(context.Background(), conversation.ID, 0, 0)
	if len(messages) != 4 {
		t.Fatalf("messages = %d, want 4", len(messages))
	}
	result := messages[2]
	if len(result.Content) > maxToolResultBytes {
		t.Fatalf("tool_result = %d bytes, want <= %d", len(result.Content), maxToolResultBytes)
	}
	var decoded struct {
		OK        bool   `json:"ok"`
		Truncated bool   `json:"truncated"`
		Data      string `json:"data"`
	}
	if err := json.Unmarshal([]byte(result.Content), &decoded); err != nil {
		t.Fatalf("clipped result is not valid JSON: %v", err)
	}
	if !decoded.OK || !decoded.Truncated || decoded.Data == "" {
		t.Fatalf("clipped result = %+v", decoded)
	}
}

// Cancelling mid-tool lands a canceled assistant message and stops the loop.
// The blocked tool's result does not land: after cancellation nothing more
// is written to the transcript.
func TestAgentLoopCancelDuringTool(t *testing.T) {
	unblock := make(chan struct{})
	executor := &fakeExecutor{result: func(ToolCall) ToolResult {
		<-unblock
		return okResult(`{"ok":true,"data":{}}`)
	}}
	service, _, _ := agentService(t, "edits", executor,
		`{"kind":"tool","tool":"categories","arguments":{}}`,
		`{"kind":"answer","answer":"never reached"}`)

	conversation, _ := service.NewConversation(context.Background())
	_ = service.Send(context.Background(), conversation.ID, "hi")
	// Wait for the tool to block, then cancel.
	time.Sleep(100 * time.Millisecond)
	service.Cancel(conversation.ID)
	close(unblock)
	waitTurn(t, service, conversation.ID)

	messages, _ := service.Messages(context.Background(), conversation.ID, 0, 0)
	if len(messages) != 3 {
		t.Fatalf("messages = %d, want 3 (user, call, canceled): %+v", len(messages), messages)
	}
	if last := messages[len(messages)-1]; last.Status != StatusCanceled {
		t.Fatalf("final status = %q, want canceled", last.Status)
	}
}

// A model that never produces a valid envelope burns the budget in
// corrections and the turn fails instead of looping forever.
func TestAgentLoopMalformedRepliesCountAgainstBudget(t *testing.T) {
	service, _, _ := agentService(t, "edits", nil,
		"not json at all",
		"still not json",
		`{"kind":"answer","answer":"终于格式正确了。"}`)

	conversation, _ := service.NewConversation(context.Background())
	_ = service.Send(context.Background(), conversation.ID, "hi")
	waitTurn(t, service, conversation.ID)

	messages, _ := service.Messages(context.Background(), conversation.ID, 0, 0)
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want 2: %+v", len(messages), messages)
	}
	if messages[1].Status != StatusOK || messages[1].Content != "终于格式正确了。" {
		t.Fatalf("answer = %+v", messages[1])
	}
}

// All-malformed: the correction loop must terminate within the budget.
func TestAgentLoopNeverValidEnvelopeTerminates(t *testing.T) {
	service, _, _ := agentService(t, "edits", nil, "garbage")

	conversation, _ := service.NewConversation(context.Background())
	_ = service.Send(context.Background(), conversation.ID, "hi")
	waitTurn(t, service, conversation.ID)

	messages, _ := service.Messages(context.Background(), conversation.ID, 0, 0)
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want 2: %+v", len(messages), messages)
	}
	if messages[1].Status != StatusFailed {
		t.Fatalf("final status = %q, want failed", messages[1].Status)
	}
}

// The second generation of a tool turn must include the tool result that
// landed between the two calls: the model answers from the result, and the
// user message is not duplicated. Regression guard for the cursor bug where
// buildRequest paged history by the user message's id and hid every row of
// the running turn, making the model re-request the same tool forever.
func TestAgentLoopSecondGenerationSeesToolResult(t *testing.T) {
	var generation atomic.Int64
	server := newOpenAIServer(t, func(prompt string) string {
		if generation.Add(1) == 1 {
			return `{"kind":"tool","tool":"timeline","arguments":{"day":"2026-09-12"}}`
		}
		if !strings.Contains(prompt, "Tool result (timeline): ") ||
			!strings.Contains(prompt, `"trackedMinutes":245`) {
			return `{"kind":"answer","answer":"PROMPT_MISSING_TOOL_RESULT"}`
		}
		return `{"kind":"answer","answer":"今天你工作了 245 分钟。"}`
	})
	providers := newFakeProviders(ProviderEntry{
		ID: "p1", Protocol: "openai", Endpoint: server.URL, Model: "m", Secret: "k",
	})
	executor := &fakeExecutor{result: func(ToolCall) ToolResult {
		return okResult(`{"ok":true,"data":{"trackedMinutes":245}}`)
	}}
	store := newFakeStore(t)
	service := New(store, providers, &fakeSettings{editMode: "edits"}, executor, nil)
	service.SetNotifier(func(string) {})

	conversation, _ := service.NewConversation(context.Background())
	if err := service.Send(context.Background(), conversation.ID, "我今天干了什么"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitTurn(t, service, conversation.ID)

	messages, _ := service.Messages(context.Background(), conversation.ID, 0, 0)
	if len(messages) != 4 {
		t.Fatalf("messages = %d, want 4 (user, tool_call, tool_result, assistant): %+v", len(messages), messages)
	}
	if messages[3].Content == "PROMPT_MISSING_TOOL_RESULT" {
		t.Fatal("second generation prompt did not contain the tool result")
	}
	if messages[3].Status != StatusOK || messages[3].Content != "今天你工作了 245 分钟。" {
		t.Fatalf("answer = %+v", messages[3])
	}
	if executor.callCount() != 1 {
		t.Fatalf("executor calls = %d, want 1 (no retry loop)", executor.callCount())
	}
}

// The prompt's convenience dates come from timeutil: today is the logical day
// (4 AM boundary), monday its week's Monday. Regression guard for the bug
// where today was computed as the Monday itself, sending the model to the
// wrong day for every "今天" question.
func TestAgentLoopPromptTodayIsLogicalDay(t *testing.T) {
	var lastPrompt string
	server := newOpenAIServer(t, func(prompt string) string {
		lastPrompt = prompt
		return `{"kind":"answer","answer":"ok"}`
	})
	providers := newFakeProviders(ProviderEntry{
		ID: "p1", Protocol: "openai", Endpoint: server.URL, Model: "m", Secret: "k",
	})
	store := newFakeStore(t)
	service := New(store, providers, &fakeSettings{editMode: "edits"}, nil, nil)
	service.SetNotifier(func(string) {})

	conversation, _ := service.NewConversation(context.Background())
	_ = service.Send(context.Background(), conversation.ID, "hi")
	waitTurn(t, service, conversation.ID)

	wantToday, wantMonday := todayAndMonday(time.Now())
	if !strings.Contains(lastPrompt, "今天的逻辑日是 "+wantToday) {
		t.Fatalf("prompt missing logical today %q:\n%s", wantToday, lastPrompt)
	}
	if !strings.Contains(lastPrompt, "本周一是 "+wantMonday) {
		t.Fatalf("prompt missing monday %q:\n%s", wantMonday, lastPrompt)
	}
}

// Past-turn tool results are clipped when replayed into the prompt.
func TestAgentLoopHistoryToolResultsClipped(t *testing.T) {
	big := strings.Repeat("y", 32*1024)
	executor := &fakeExecutor{result: func(ToolCall) ToolResult {
		return ToolResult{Data: json.RawMessage(`{"ok":true,"data":"` + big + `"}`)}
	}}
	service, _, _ := agentService(t, "edits", executor,
		`{"kind":"tool","tool":"timeline","arguments":{"day":"2026-09-12"}}`,
		`{"kind":"answer","answer":"第一回合完成。"}`,
		`{"kind":"answer","answer":"第二回合完成。"}`)

	conversation, _ := service.NewConversation(context.Background())
	_ = service.Send(context.Background(), conversation.ID, "第一回合")
	waitTurn(t, service, conversation.ID)
	_ = service.Send(context.Background(), conversation.ID, "第二回合")
	waitTurn(t, service, conversation.ID)

	// The second turn's prompt must not carry the full 32 KiB result twice.
	// Indirect proof: the first turn's result row is intact in the store...
	messages, _ := service.Messages(context.Background(), conversation.ID, 0, 0)
	found := false
	for _, m := range messages {
		if m.Role == RoleToolRes && len(m.Content) > 16*1024 {
			found = true
		}
	}
	if !found {
		t.Fatal("store must keep the full tool result")
	}
	// ...and clipHistory itself is exercised directly below.
	clipped := clipHistory(big)
	if len(clipped) > historyToolResultLimit+64 {
		t.Fatalf("clipHistory length = %d", len(clipped))
	}
}

// The executor's own error envelope (readonly instance, apperr codes) flows
// through unchanged and the turn continues.
func TestAgentLoopExecutorErrorEnvelope(t *testing.T) {
	executor := &fakeExecutor{result: func(ToolCall) ToolResult {
		return ToolResult{Data: toolResultEnvelope(false, "readonly_instance", "此实例为只读。"), Err: true}
	}}
	service, _, _ := agentService(t, "edits", executor,
		`{"kind":"tool","tool":"card_delete","arguments":{"cardId":3}}`,
		`{"kind":"answer","answer":"写入被系统拒绝。"}`)

	conversation, _ := service.NewConversation(context.Background())
	_ = service.Send(context.Background(), conversation.ID, "删掉它")
	waitTurn(t, service, conversation.ID)

	messages, _ := service.Messages(context.Background(), conversation.ID, 0, 0)
	if len(messages) != 4 {
		t.Fatalf("messages = %d, want 4: %+v", len(messages), messages)
	}
	if !strings.Contains(messages[2].Content, "readonly_instance") {
		t.Fatalf("result = %s", messages[2].Content)
	}
	if messages[3].Status != StatusOK {
		t.Fatalf("turn must continue after an executor error envelope")
	}
}

// Secrets never reach the transcript: the fixed sanitized messages and
// envelope errors are the only failure text stored.
func TestAgentLoopDoesNotLeakSecret(t *testing.T) {
	service, _, _ := agentService(t, "edits", nil,
		`{"kind":"tool","tool":"nope","arguments":{}}`,
		`{"kind":"answer","answer":"done"}`)

	conversation, _ := service.NewConversation(context.Background())
	_ = service.Send(context.Background(), conversation.ID, "hi")
	waitTurn(t, service, conversation.ID)

	messages, _ := service.Messages(context.Background(), conversation.ID, 0, 0)
	for _, m := range messages {
		if strings.Contains(m.Content, "sk-agent-test") {
			t.Fatalf("secret leaked into message: %+v", m)
		}
		if strings.Contains(m.ToolArguments, "sk-agent-test") {
			t.Fatalf("secret leaked into tool arguments: %+v", m)
		}
	}
}

// Envelope parsing failure on Generate error path still classifies
// cancellation correctly (regression guard for the loop's error branch).
func TestAgentLoopProviderErrorLandsFailed(t *testing.T) {
	_, store, _ := agentService(t, "edits", nil, "")
	failing := newOpenAIFailingServer(t, 500)
	providers := newFakeProviders(ProviderEntry{
		ID: "p1", Protocol: "openai", Endpoint: failing.URL, Model: "m", Secret: "sk-agent-test",
	})
	failingService := New(store, providers, &fakeSettings{editMode: "edits"}, nil, nil)
	failingService.SetNotifier(func(string) {})

	conversation, _ := failingService.NewConversation(context.Background())
	_ = failingService.Send(context.Background(), conversation.ID, "hi")
	waitTurn(t, failingService, conversation.ID)

	messages, _ := failingService.Messages(context.Background(), conversation.ID, 0, 0)
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(messages))
	}
	if messages[1].Status != StatusFailed {
		t.Fatalf("status = %q, want failed", messages[1].Status)
	}
	if strings.Contains(messages[1].Content, "sk-agent-test") {
		t.Fatalf("secret leaked into failure message: %q", messages[1].Content)
	}
}

// Compile-time guard: the interfaces the app layer must implement.
var (
	_ ToolExecutor = (*fakeExecutor)(nil)
	_ AttemptSink  = attemptSinkFunc(nil)
)

type attemptSinkFunc func(ctx context.Context, attempt ai.Attempt)

func (f attemptSinkFunc) RecordAttempt(ctx context.Context, attempt ai.Attempt) {
	f(ctx, attempt)
}
