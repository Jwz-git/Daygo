package chat

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// fakeStore adapts storage.ChatRepo's signature set to the chat.Store
// interface using a real temp-dir database — the repo is already tested for
// SQL behavior; what these tests exercise is the service around it.
type fakeStore struct {
	*storage.ChatRepo
}

func newFakeStore(t *testing.T) *fakeStore {
	t.Helper()
	store, err := storage.Open(context.Background(), storage.Options{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return &fakeStore{store.Chat()}
}

// The real repo returns storage.Conversation; the adapter re-exports it as
// chat.Conversation because the types are structurally identical by design
// (one definition per concept, storage owns persistence, chat owns behavior).
func (f fakeStore) CreateConversation(ctx context.Context, c Conversation) (Conversation, error) {
	created, err := f.ChatRepo.CreateConversation(ctx, storage.Conversation{
		ID: c.ID, Title: c.Title, ProviderID: c.ProviderID,
	})
	return toConversation(created), err
}

func (f fakeStore) GetConversation(ctx context.Context, id string) (Conversation, error) {
	c, err := f.ChatRepo.GetConversation(ctx, id)
	return toConversation(c), err
}

func (f fakeStore) UpdateConversation(ctx context.Context, id string, title string, providerID *string) error {
	return f.ChatRepo.UpdateConversation(ctx, id, title, providerID)
}

func (f fakeStore) ListConversations(ctx context.Context) ([]Conversation, error) {
	rows, err := f.ChatRepo.ListConversations(ctx)
	out := make([]Conversation, len(rows))
	for i, row := range rows {
		out[i] = toConversation(row)
	}
	return out, err
}

func (f fakeStore) AppendMessage(ctx context.Context, conversationID string, m Message) (Message, error) {
	saved, err := f.ChatRepo.AppendMessage(ctx, conversationID, storage.ChatMessage{
		Role: m.Role, Content: m.Content, Status: m.Status,
		ToolName: m.ToolName, ToolArguments: m.ToolArguments,
	})
	m.ID = saved.ID
	m.ConversationID = saved.ConversationID
	m.CreatedAt = saved.CreatedAt
	return m, err
}

func (f fakeStore) Messages(ctx context.Context, conversationID string, beforeID int64, limit int) ([]Message, error) {
	rows, err := f.ChatRepo.Messages(ctx, conversationID, beforeID, limit)
	out := make([]Message, len(rows))
	for i, row := range rows {
		out[i] = Message{
			ID: row.ID, ConversationID: row.ConversationID, Role: row.Role,
			Content: row.Content, Status: row.Status,
			ToolName: row.ToolName, ToolArguments: row.ToolArguments,
			CreatedAt: row.CreatedAt,
		}
	}
	return out, err
}

func toConversation(c storage.Conversation) Conversation {
	return Conversation{
		ID: c.ID, Title: c.Title, ProviderID: c.ProviderID,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

// fakeProviders serves scripted provider entries.
type fakeProviders struct {
	mu        sync.Mutex
	chain     []ProviderEntry
	byID      map[string]ProviderEntry
	chainErr  error
	byIDCalls map[string]int
}

func newFakeProviders(entries ...ProviderEntry) *fakeProviders {
	byID := make(map[string]ProviderEntry, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = entry
	}
	return &fakeProviders{chain: entries, byID: byID, byIDCalls: make(map[string]int)}
}

func (p *fakeProviders) Chain(context.Context) ([]ProviderEntry, error) {
	return p.chain, p.chainErr
}

func (p *fakeProviders) ByID(_ context.Context, id string) (ProviderEntry, error) {
	p.mu.Lock()
	p.byIDCalls[id]++
	p.mu.Unlock()
	entry, ok := p.byID[id]
	if !ok {
		return ProviderEntry{}, fmt.Errorf("provider %q not found", id)
	}
	return entry, nil
}

// fakeSettings carries the global memory and the sandbox gate.
type fakeSettings struct {
	memory   string
	editMode string
}

func (s *fakeSettings) Memory(context.Context) (string, error) { return s.memory, nil }

func (s *fakeSettings) EditMode(context.Context) (string, error) { return s.editMode, nil }

// scriptedProvider is a fake ai.Provider with controllable outcomes.
type scriptedProvider struct {
	mu       sync.Mutex
	results  []string
	errors   []error
	requests []ai.Request
	block    chan struct{}
}

func (p *scriptedProvider) Generate(_ context.Context, request ai.Request) (ai.Result, error) {
	p.mu.Lock()
	p.requests = append(p.requests, request)
	index := len(p.requests) - 1
	p.mu.Unlock()

	if p.block != nil {
		<-p.block
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	var result ai.Result
	var err error
	if index < len(p.results) {
		result = ai.Result{Text: p.results[index]}
	}
	if index < len(p.errors) {
		err = p.errors[index]
	}
	return result, err
}

func (p *scriptedProvider) lastRequest() ai.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.requests) == 0 {
		return ai.Request{}
	}
	return p.requests[len(p.requests)-1]
}

func (p *scriptedProvider) calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.requests)
}

// A service wired for tests with the given provider entries.
func testService(t *testing.T, providers *fakeProviders, settings *fakeSettings) (*Service, *fakeStore) {
	t.Helper()
	store := newFakeStore(t)
	service := New(store, providers, settings, nil, nil)
	done := make(chan string, 16)
	service.SetNotifier(func(conversationID string) { done <- conversationID })
	t.Cleanup(func() { close(done) })
	// Polling helper instead of exposing internals.
	t.Cleanup(func() { _ = service })
	return service, store
}

// waitTurn waits until the service reports the conversation's turn done.
func waitTurn(t *testing.T, service *Service, conversationID string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		service.mu.Lock()
		_, running := service.inflight[conversationID]
		service.mu.Unlock()
		if !running {
			return
		}
		select {
		case <-deadline:
			t.Fatal("turn did not finish within 5s")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// The provider entries used by tests. The "client" factory is not exercised —
// rebuildChain builds real clients from these fields, so the fake provider
// never sees them; instead tests verify through the scriptedProvider only in
// chain-level tests. For service-level tests we use an httptest-backed
// entry? No: simpler to assert via the scriptedProvider injected by
// overriding the chain — but rebuildChain constructs clients from fields.
// The cleanest seam: run a local HTTP server speaking the openai protocol.
func TestServiceHappyPathPersistsMessages(t *testing.T) {
	server := newOpenAIServer(t, func(prompt string) string { return "你好，我是助手。" })
	providers := newFakeProviders(ProviderEntry{
		ID: "p1", Protocol: "openai", Endpoint: server.URL, Model: "m", Secret: "sk-test",
	})
	service, store := testService(t, providers, &fakeSettings{})

	conversation, err := service.NewConversation(context.Background())
	if err != nil {
		t.Fatalf("NewConversation: %v", err)
	}
	if err := service.Send(context.Background(), conversation.ID, "你好"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitTurn(t, service, conversation.ID)

	messages, err := service.Messages(context.Background(), conversation.ID, 0, 0)
	if err != nil {
		t.Fatalf("Messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want 2 (user + assistant)", len(messages))
	}
	if messages[0].Role != RoleUser || messages[0].Content != "你好" {
		t.Fatalf("user message = %+v", messages[0])
	}
	if messages[1].Role != RoleAssistant || messages[1].Status != StatusOK || messages[1].Content != "你好，我是助手。" {
		t.Fatalf("assistant message = %+v", messages[1])
	}

	// The conversation took its title from the first user message.
	got, err := service.Conversations(context.Background())
	if err != nil || len(got) != 1 || got[0].Title != "你好" {
		t.Fatalf("conversations = %+v (%v)", got, err)
	}
	_ = store
}

// Global memory is appended to the system prompt of every turn.
func TestServiceInjectsGlobalMemory(t *testing.T) {
	var lastPrompt string
	server := newOpenAIServer(t, func(prompt string) string {
		lastPrompt = prompt
		return "ok"
	})
	providers := newFakeProviders(ProviderEntry{
		ID: "p1", Protocol: "openai", Endpoint: server.URL, Model: "m", Secret: "k",
	})
	service, _ := testService(t, providers, &fakeSettings{memory: "回答必须简短。"})

	conversation, _ := service.NewConversation(context.Background())
	if err := service.Send(context.Background(), conversation.ID, "hi"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitTurn(t, service, conversation.ID)

	if !strings.Contains(lastPrompt, "回答必须简短。") {
		t.Fatalf("prompt missing global memory:\n%s", lastPrompt)
	}
	if !strings.Contains(lastPrompt, "你是 Daygo 的时间跟踪助手") {
		t.Fatal("prompt missing the base system prompt")
	}
	if !strings.Contains(lastPrompt, "User: hi") {
		t.Fatalf("prompt missing the user message:\n%s", lastPrompt)
	}
}

// History from earlier turns is part of the next prompt.
func TestServiceIncludesHistory(t *testing.T) {
	var lastPrompt string
	server := newOpenAIServer(t, func(prompt string) string {
		lastPrompt = prompt
		return "ok"
	})
	providers := newFakeProviders(ProviderEntry{
		ID: "p1", Protocol: "openai", Endpoint: server.URL, Model: "m", Secret: "k",
	})
	service, _ := testService(t, providers, &fakeSettings{})

	conversation, _ := service.NewConversation(context.Background())
	_ = service.Send(context.Background(), conversation.ID, "first")
	waitTurn(t, service, conversation.ID)
	_ = service.Send(context.Background(), conversation.ID, "second")
	waitTurn(t, service, conversation.ID)

	if !strings.Contains(lastPrompt, "User: first") || !strings.Contains(lastPrompt, "Assistant: ok") || !strings.Contains(lastPrompt, "User: second") {
		t.Fatalf("prompt missing history:\n%s", lastPrompt)
	}
}

// A conversation pinned to a provider uses exactly that provider.
func TestServicePinnedProviderNoFallback(t *testing.T) {
	var calls atomic.Int64
	primary := newOpenAIServer(t, func(string) string { return "primary" })
	fallback := newOpenAIServerWithCalls(t, func(string) string { return "fallback" }, &calls)

	providers := newFakeProviders(
		ProviderEntry{ID: "p1", Protocol: "openai", Endpoint: primary.URL, Model: "m", Secret: "k"},
		ProviderEntry{ID: "p2", Protocol: "openai", Endpoint: fallback.URL, Model: "m", Secret: "k"},
	)
	service, _ := testService(t, providers, &fakeSettings{})

	conversation, _ := service.NewConversation(context.Background())
	if err := service.SetConversationProvider(context.Background(), conversation.ID, "p2"); err != nil {
		t.Fatalf("SetConversationProvider: %v", err)
	}
	if err := service.Send(context.Background(), conversation.ID, "hi"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitTurn(t, service, conversation.ID)

	if calls.Load() == 0 {
		t.Fatal("pinned provider was not called")
	}

	messages, _ := service.Messages(context.Background(), conversation.ID, 0, 0)
	if len(messages) != 2 || messages[1].Content != "fallback" {
		t.Fatalf("messages = %+v", messages)
	}
}

// An unknown pinned provider id is rejected by SetConversationProvider.
func TestServicePinnedProviderMustExist(t *testing.T) {
	providers := newFakeProviders()
	service, _ := testService(t, providers, &fakeSettings{})

	conversation, _ := service.NewConversation(context.Background())
	if err := service.SetConversationProvider(context.Background(), conversation.ID, "ghost"); err == nil {
		t.Fatal("pinning an unknown provider succeeded")
	}
}

// New threads default to the routing chain's primary provider.
func TestNewConversationDefaultsToPrimaryProvider(t *testing.T) {
	providers := newFakeProviders(
		ProviderEntry{ID: "p1", Protocol: "openai", Endpoint: "http://localhost:1", Model: "m", Secret: "k"},
		ProviderEntry{ID: "p2", Protocol: "openai", Endpoint: "http://localhost:1", Model: "m", Secret: "k"},
	)
	service, _ := testService(t, providers, &fakeSettings{})

	conversation, err := service.NewConversation(context.Background())
	if err != nil {
		t.Fatalf("NewConversation: %v", err)
	}
	if conversation.ProviderID == nil || *conversation.ProviderID != "p1" {
		t.Fatalf("default provider = %+v, want p1", conversation.ProviderID)
	}
}

// A thread with no pinned provider fails the turn instead of falling back to
// the routing chain.
func TestServiceSendWithoutProviderFails(t *testing.T) {
	server := newOpenAIServer(t, func(string) string { return "ok" })
	providers := newFakeProviders(ProviderEntry{
		ID: "p1", Protocol: "openai", Endpoint: server.URL, Model: "m", Secret: "k",
	})
	service, _ := testService(t, providers, &fakeSettings{})

	conversation, _ := service.NewConversation(context.Background())
	if err := service.SetConversationProvider(context.Background(), conversation.ID, ""); err != nil {
		t.Fatalf("clear provider pin: %v", err)
	}
	if err := service.Send(context.Background(), conversation.ID, "hi"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitTurn(t, service, conversation.ID)

	messages, _ := service.Messages(context.Background(), conversation.ID, 0, 0)
	if len(messages) != 2 || messages[1].Status != StatusFailed {
		t.Fatalf("messages = %+v", messages)
	}
	if !strings.Contains(messages[1].Content, "尚未选择供应商") {
		t.Fatalf("failure text = %q", messages[1].Content)
	}
}

// No configured providers → a failed assistant message, no request.
func TestServiceNoProviderFailsGracefully(t *testing.T) {
	providers := newFakeProviders()
	service, _ := testService(t, providers, &fakeSettings{})

	conversation, _ := service.NewConversation(context.Background())
	if err := service.Send(context.Background(), conversation.ID, "hi"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitTurn(t, service, conversation.ID)

	messages, _ := service.Messages(context.Background(), conversation.ID, 0, 0)
	if len(messages) != 2 || messages[1].Status != StatusFailed {
		t.Fatalf("messages = %+v", messages)
	}
	if !strings.Contains(messages[1].Content, "供应商") {
		t.Fatalf("failure text = %q", messages[1].Content)
	}
}

// A provider error lands as a failed assistant message with sanitized text.
func TestServiceProviderFailureLandsAsFailedMessage(t *testing.T) {
	server := newOpenAIFailingServer(t, 401)
	providers := newFakeProviders(ProviderEntry{
		ID: "p1", Protocol: "openai", Endpoint: server.URL, Model: "m", Secret: "bad",
	})
	service, _ := testService(t, providers, &fakeSettings{})

	conversation, _ := service.NewConversation(context.Background())
	_ = service.Send(context.Background(), conversation.ID, "hi")
	waitTurn(t, service, conversation.ID)

	messages, _ := service.Messages(context.Background(), conversation.ID, 0, 0)
	if len(messages) != 2 || messages[1].Status != StatusFailed {
		t.Fatalf("messages = %+v", messages)
	}
	if strings.Contains(messages[1].Content, "bad") {
		t.Fatalf("failure text leaks the secret: %q", messages[1].Content)
	}
}

// Two turns may run concurrently in different conversations.
func TestServiceConcurrentTurnsAcrossConversations(t *testing.T) {
	server := newOpenAIServer(t, func(string) string { return "ok" })
	providers := newFakeProviders(ProviderEntry{
		ID: "p1", Protocol: "openai", Endpoint: server.URL, Model: "m", Secret: "k",
	})
	service, _ := testService(t, providers, &fakeSettings{})

	c1, _ := service.NewConversation(context.Background())
	c2, _ := service.NewConversation(context.Background())
	if err := service.Send(context.Background(), c1.ID, "one"); err != nil {
		t.Fatalf("Send c1: %v", err)
	}
	if err := service.Send(context.Background(), c2.ID, "two"); err != nil {
		t.Fatalf("Send c2: %v", err)
	}
	waitTurn(t, service, c1.ID)
	waitTurn(t, service, c2.ID)

	for _, id := range []string{c1.ID, c2.ID} {
		messages, _ := service.Messages(context.Background(), id, 0, 0)
		if len(messages) != 2 || messages[1].Status != StatusOK {
			t.Fatalf("conversation %s messages = %+v", id, messages)
		}
	}
}

// Empty and oversized messages are rejected synchronously.
func TestServiceMessageValidation(t *testing.T) {
	providers := newFakeProviders()
	service, _ := testService(t, providers, &fakeSettings{})

	conversation, _ := service.NewConversation(context.Background())
	if err := service.Send(context.Background(), conversation.ID, "   "); err == nil {
		t.Fatal("blank message accepted")
	}
	if err := service.Send(context.Background(), conversation.ID, strings.Repeat("x", maxContentBytes+1)); err == nil {
		t.Fatal("oversized message accepted")
	}
}

func TestServiceDeleteConversation(t *testing.T) {
	providers := newFakeProviders()
	service, _ := testService(t, providers, &fakeSettings{})

	conversation, _ := service.NewConversation(context.Background())
	if err := service.DeleteConversation(context.Background(), conversation.ID); err != nil {
		t.Fatalf("DeleteConversation: %v", err)
	}
	if _, err := service.Messages(context.Background(), conversation.ID, 0, 0); err == nil {
		// Messages on a deleted conversation return an empty page via the
		// repo (no error, no rows) — the assertion is the conversations list.
		_ = err
	}
	list, _ := service.Conversations(context.Background())
	if len(list) != 0 {
		t.Fatalf("conversations after delete = %+v", list)
	}
}
