// Package chat is the in-app conversational agent's service layer
// (docs/modules/chat.md). A turn is a structured-output tool loop: the model
// replies with an envelope asking for a tool or giving an answer, gated by
// the editMode sandbox and bounded by the docs/05 §5.12 budgets.
//
// The package knows Wails nothing. It depends on consumer-side interfaces
// (Store, Providers, Settings, ToolExecutor, AttemptSink) so the service is
// testable headless and CGO_ENABLED=0 (docs/02 §2.1).
package chat

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/ai/factory"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// maxContentBytes bounds one message. 32 KiB is far beyond a normal chat
// message and keeps a runaway paste from ballooning the request.
const maxContentBytes = 32 << 10

// maxOutputTokens is the reply ceiling for one model call inside a turn.
const maxOutputTokens = 4096

// maxModelBytes bounds one conversation model override.
const maxModelBytes = 256

// Agent budgets from docs/05 §5.12: at most 8 tool calls per user message,
// one tool result clipped to 64 KiB, and a whole-turn time limit of 120 s
// shared with user cancellation.
const (
	maxToolCallsPerTurn = 8
	maxToolResultBytes  = 64 << 10
	turnTimeout         = 120 * time.Second
)

// Message roles and assistant statuses mirror the closed sets in storage
// (docs/03 §3.3.4).
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleToolCall  = "tool_call"
	RoleToolRes   = "tool_result"

	StatusOK       = "ok"
	StatusFailed   = "failed"
	StatusCanceled = "canceled"
)

// Conversation is one chat thread. Model is the per-thread override: the
// empty string follows the provider's configured model.
type Conversation struct {
	ID         string
	Title      string
	ProviderID *string
	Model      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Message is one atomic chat message. ToolName/ToolArguments are set only on
// agent rows: tool_call fills both, tool_result pairs by ToolName with empty
// ToolArguments and carries its result envelope in Content.
type Message struct {
	ID             int64
	ConversationID string
	Role           string
	Content        string
	Status         string
	ToolName       string
	ToolArguments  string
	CreatedAt      time.Time
}

// Store is the persistence this package needs, defined at the consumer
// (docs/02 §2.1 rule 2). storage.ChatRepo satisfies it.
type Store interface {
	CreateConversation(ctx context.Context, c Conversation) (Conversation, error)
	GetConversation(ctx context.Context, id string) (Conversation, error)
	DeleteConversation(ctx context.Context, id string) error
	UpdateConversation(ctx context.Context, id string, title string, providerID *string, model string) error
	ListConversations(ctx context.Context) ([]Conversation, error)
	AppendMessage(ctx context.Context, conversationID string, m Message) (Message, error)
	Messages(ctx context.Context, conversationID string, beforeID int64, limit int) ([]Message, error)
}

// Providers resolves routing-chain entries. The app layer implements it over
// ProviderRepo + keychain + settings.Routing; the interface keeps the chat
// package free of storage and platform imports.
type ProviderEntry struct {
	ID       string
	Protocol string
	Endpoint string
	Model    string
	Secret   string
}

type Providers interface {
	// Chain returns the ordered routing chain entries. Empty means no
	// provider is configured.
	Chain(ctx context.Context) ([]ProviderEntry, error)
	// ByID returns one provider by id, with its secret.
	ByID(ctx context.Context, id string) (ProviderEntry, error)
}

// Settings supplies the global chat memory and the agent sandbox gate.
// Memory returns the user-authored text injected into every conversation's
// system prompt; EditMode returns the chat.editMode value ("readonly" or
// "edits"), which the service re-reads each turn.
type Settings interface {
	Memory(ctx context.Context) (string, error)
	EditMode(ctx context.Context) (string, error)
}

// ToolCall is one requested tool invocation from the model.
type ToolCall struct {
	Tool      string
	Arguments json.RawMessage
}

// ToolResult is the envelope JSON handed back to the model. Data is always a
// complete result envelope ({"ok":true,...} or {"ok":false,"error":{...}});
// Err records which of the two it is.
type ToolResult struct {
	Data json.RawMessage
	Err  bool
}

// ToolExecutor runs one tool call. It is defined here, at the consumer, and
// implemented by the app layer over the same shared write paths the bindings
// use; the chat package never touches storage or Wails.
type ToolExecutor interface {
	Execute(ctx context.Context, call ToolCall) ToolResult
}

// AttemptSink records one sanitized provider attempt (llm_calls audit,
// purpose=chat). Defined here so the service can wrap its chain with
// ai.WithAttemptObserver without depending on storage.
type AttemptSink interface {
	RecordAttempt(ctx context.Context, attempt ai.Attempt)
}

// Clock is the title/id source, injectable for tests.
type Clock func() time.Time

// Service drives chat conversations.
type Service struct {
	store     Store
	providers Providers
	settings  Settings
	tools     ToolExecutor
	sink      AttemptSink
	notifier  func(conversationID string)

	mu       sync.Mutex
	inflight map[string]context.CancelFunc
	chain    *ai.Chain
}

// New wires the service. tools may be nil (plain-conversation mode: the
// envelope stays in force and the model can only answer); sink may be nil
// (no llm_calls audit).
func New(store Store, providers Providers, settings Settings, tools ToolExecutor, sink AttemptSink) *Service {
	return &Service{
		store:     store,
		providers: providers,
		settings:  settings,
		tools:     tools,
		sink:      sink,
		inflight:  make(map[string]context.CancelFunc),
		chain:     ai.NewChain(nil, 0),
	}
}

// ErrTurnInFlight reports that the conversation already has a running turn.
// It is a value, not an apperr: the binding layer maps it to invalid_argument.
var ErrTurnInFlight = fmt.Errorf("chat: a turn is already running in this conversation")

// errNoProviderSelected reports a thread with no pinned provider. Selecting
// one is explicit; there is no implicit chain fallback in chat.
var errNoProviderSelected = fmt.Errorf("chat: conversation has no provider selected")

// Send appends the user message and runs one assistant turn asynchronously.
// It returns once the user message is persisted; the assistant message lands
// later and is announced through the emitter the caller installed
// (SetNotifier). Content must be non-empty and within maxContentBytes.
func (s *Service) Send(ctx context.Context, conversationID, content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("chat: message is empty")
	}
	if len(content) > maxContentBytes {
		return fmt.Errorf("chat: message exceeds %d bytes", maxContentBytes)
	}

	if _, err := s.store.GetConversation(ctx, conversationID); err != nil {
		return err
	}

	s.mu.Lock()
	if _, running := s.inflight[conversationID]; running {
		s.mu.Unlock()
		return ErrTurnInFlight
	}
	// The turn's lifetime is the service's, not the caller's: the binding
	// method returns as soon as the user message is stored. The 120 s budget
	// shares this context, so cancellation and expiry are one path.
	turnCtx, cancel := context.WithTimeout(context.Background(), turnTimeout)
	s.inflight[conversationID] = cancel
	s.mu.Unlock()

	// Persist the user message before starting the turn: a reply to a message
	// that never landed would be orphaned in the transcript.
	userMsg, err := s.store.AppendMessage(ctx, conversationID, Message{Role: RoleUser, Content: content})
	if err != nil {
		s.finishTurn(conversationID, cancel)
		return err
	}

	// First user message titles the conversation.
	conversation, err := s.store.GetConversation(ctx, conversationID)
	if err == nil && conversation.Title == "" {
		_ = s.store.UpdateConversation(ctx, conversationID, titleFrom(content), conversation.ProviderID, conversation.Model)
	}

	go s.runTurn(turnCtx, conversationID, userMsg)
	return nil
}

// runTurn executes one agent turn: resolve the provider, then loop —
// generate an envelope, execute the tool it asks for (or none), feed the
// result back — until the model answers, the budgets run out, or the turn is
// canceled. Every intermediate step lands in the transcript as tool_call /
// tool_result rows and notifies, so the UI tracks progress.
func (s *Service) runTurn(ctx context.Context, conversationID string, userMsg Message) {
	conversation, err := s.store.GetConversation(ctx, conversationID)
	if err != nil {
		s.finishTurnID(conversationID)
		return
	}

	entries, err := s.resolveEntries(ctx, conversation)
	if err != nil {
		s.complete(conversationID, Message{Role: RoleAssistant, Status: StatusFailed, Content: failureText(err)})
		return
	}
	if len(entries) == 0 {
		s.complete(conversationID, Message{Role: RoleAssistant, Status: StatusFailed, Content: failureText(ai.ErrNoProvider)})
		return
	}

	s.mu.Lock()
	s.rebuildChain(entries)
	s.mu.Unlock()

	toolCalls := 0
	correction := ""
	for {
		request, err := s.buildRequest(ctx, conversationID, userMsg, correction)
		if err != nil {
			s.complete(conversationID, Message{Role: RoleAssistant, Status: StatusFailed, Content: failureText(err)})
			return
		}
		request.Output = &envelopeOutput

		result, err := s.chain.Generate(ctx, request)
		correction = ""
		if err != nil {
			status := StatusFailed
			if ctx.Err() != nil || ai.ErrorKindOf(err) == ai.ErrorCanceled {
				status = StatusCanceled
			} else if toolCalls >= maxToolCallsPerTurn {
				status = StatusFailed
			}
			s.complete(conversationID, Message{Role: RoleAssistant, Status: status, Content: failureText(err)})
			return
		}

		reply, err := decodeEnvelope(result.Text)
		if err != nil {
			// A malformed reply counts against the tool budget: without that,
			// a model looping on garbage would never terminate.
			toolCalls++
			if toolCalls >= maxToolCallsPerTurn {
				s.complete(conversationID, Message{Role: RoleAssistant, Status: StatusFailed,
					Content: "已达到工具调用次数上限（8 次），回合终止。"})
				return
			}
			correction = envelopeCorrection
			continue
		}

		if reply.Kind == envelopeKindAnswer {
			s.complete(conversationID, Message{Role: RoleAssistant, Status: StatusOK, Content: reply.Answer})
			return
		}

		// The model asked for a tool. Record the call before doing anything:
		// the transcript keeps the audit trail even when the turn dies here.
		compactArgs, _ := json.Marshal(reply.Arguments)
		if _, err := s.store.AppendMessage(ctx, conversationID, Message{
			Role: RoleToolCall, ToolName: reply.Tool, ToolArguments: string(compactArgs),
		}); err != nil {
			s.complete(conversationID, Message{Role: RoleAssistant, Status: StatusFailed, Content: failureText(err)})
			return
		}
		s.notify(conversationID)

		if toolCalls >= maxToolCallsPerTurn {
			s.landToolResult(ctx, conversationID, reply.Tool, toolResultEnvelope(false, "budget_exceeded",
				"已达到工具调用次数上限（8 次），本次调用不执行。"))
			s.complete(conversationID, Message{Role: RoleAssistant, Status: StatusFailed,
				Content: "已达到工具调用次数上限（8 次），回合终止。"})
			return
		}
		toolCalls++

		outcome := s.executeTool(ctx, conversationID, reply)
		s.landToolResult(ctx, conversationID, reply.Tool, outcome)
		if ctx.Err() != nil {
			s.complete(conversationID, Message{Role: RoleAssistant, Status: StatusCanceled, Content: failureText(ctx.Err())})
			return
		}
	}
}

// executeTool runs one validated tool call. Gate and validation failures are
// tool results, not turn failures (docs/05 §5.12): the model sees the closed
// error and continues, typically by answering with what it has.
func (s *Service) executeTool(ctx context.Context, conversationID string, reply envelope) json.RawMessage {
	if _, known := toolByName(reply.Tool); !known {
		return toolResultEnvelope(false, "unknown_tool", "未知工具 "+reply.Tool+"。")
	}
	if err := validateToolArguments(reply.Tool, reply.Arguments); err != nil {
		return toolResultEnvelope(false, "invalid_argument", "工具参数不符合要求："+err.Error())
	}
	if isWriteTool(reply.Tool) && !s.writesAllowed(ctx) {
		return toolResultEnvelope(false, "edits_disabled",
			"当前为只读模式，写操作被拒绝。请用现有数据回答，并提示用户在设置中开启「应用内对话编辑」。")
	}
	if s.tools == nil {
		return toolResultEnvelope(false, "internal_error", "工具执行器未接入。")
	}
	outcome := s.tools.Execute(ctx, ToolCall{Tool: reply.Tool, Arguments: reply.Arguments})
	return clipToolResult(outcome.Data)
}

// writesAllowed reads the sandbox gate for this turn; it fails closed.
func (s *Service) writesAllowed(ctx context.Context) bool {
	if s.settings == nil {
		return false
	}
	mode, err := s.settings.EditMode(ctx)
	return err == nil && normalizeEditMode(mode) == editModeEdits
}

// landToolResult persists one tool_result row (paired by tool name) and
// notifies. The transcript is the progress channel: each row triggers the
// invalidation event the frontend re-pulls on.
func (s *Service) landToolResult(ctx context.Context, conversationID, tool string, result json.RawMessage) {
	if _, err := s.store.AppendMessage(ctx, conversationID, Message{
		Role: RoleToolRes, ToolName: tool, Content: string(result),
	}); err != nil {
		return
	}
	s.notify(conversationID)
}

// notify fires the invalidation callback for one conversation.
func (s *Service) notify(conversationID string) {
	if s.notifier != nil {
		s.notifier(conversationID)
	}
}

// toolResultEnvelope builds one result envelope. Success carries the data
// JSON as-is; failure carries the closed error code and a fixed message.
func toolResultEnvelope(ok bool, code, message string) json.RawMessage {
	if ok {
		return json.RawMessage(`{"ok":true,"data":null}`)
	}
	envelope, _ := json.Marshal(map[string]any{
		"ok": false,
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
	return envelope
}

// clipToolResult applies the 64 KiB budget. An oversized success payload is
// re-packed as a truncated JSON string so the row stays parseable.
func clipToolResult(data json.RawMessage) json.RawMessage {
	if len(data) <= maxToolResultBytes {
		return data
	}
	budget := maxToolResultBytes - 128 // room for the envelope keys
	if budget < 0 {
		budget = 0
	}
	clipped := string(data[:budget])
	envelope, _ := json.Marshal(map[string]any{
		"ok":        true,
		"truncated": true,
		"data":      clipped,
	})
	return envelope
}

// resolveEntries picks the provider for a conversation: the pinned provider
// as a single entry, no fallback. A thread without a pin is a hard error —
// chat never implicitly follows the routing chain. A non-empty model override
// replaces the entry's configured model.
func (s *Service) resolveEntries(ctx context.Context, conversation Conversation) ([]ProviderEntry, error) {
	if conversation.ProviderID == nil || *conversation.ProviderID == "" {
		return nil, errNoProviderSelected
	}
	entry, err := s.providers.ByID(ctx, *conversation.ProviderID)
	if err != nil {
		return nil, err
	}
	if conversation.Model != "" {
		entry.Model = conversation.Model
	}
	return []ProviderEntry{entry}, nil
}

// rebuildChain swaps the chain's entries, preserving failure counters by id
// (ai.Chain.Rebuild).
func (s *Service) rebuildChain(entries []ProviderEntry) {
	chainEntries := make([]ai.ChainEntry, 0, len(entries))
	for _, entry := range entries {
		provider, err := factory.NewClient(nil, factory.Config{
			Protocol: ai.Protocol(entry.Protocol),
			Endpoint: entry.Endpoint,
			Model:    entry.Model,
			Secret:   entry.Secret,
		})
		if err != nil {
			// A broken entry would fail every Generate anyway; the honest move
			// is a chain without it, which the remaining entries still serve.
			continue
		}
		if s.sink != nil {
			sink := s.sink
			provider = ai.WithAttemptObserver(provider, entry.ID, ai.Protocol(entry.Protocol), entry.Model,
				ai.AttemptObserverFunc(func(_ context.Context, attempt ai.Attempt) {
					// The attempt's own ctx may already be canceled; audit
					// landing must not be, or a canceled turn loses its final
					// failure row.
					dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					sink.RecordAttempt(dbCtx, attempt)
				}))
		}
		provider = ai.WithRetry(provider, ai.DefaultRetryPolicy())
		chainEntries = append(chainEntries, ai.ChainEntry{ID: entry.ID, Provider: provider})
	}
	s.chain.Rebuild(chainEntries)
}

// buildRequest assembles the prompt: agent system text (catalog + gate +
// today/monday), global memory, the conversation history as labelled text,
// and the new user message. History tool rows render as labelled exchanges
// so the model sees its own past calls. The page is the latest one (no
// beforeID): this turn's own tool_call / tool_result rows are newer than the
// user message, so paging by userMsg.ID would hide them and the model would
// re-request the same tool forever.
func (s *Service) buildRequest(ctx context.Context, conversationID string, userMsg Message, extra string) (ai.Request, error) {
	history, err := s.store.Messages(ctx, conversationID, 0, 0)
	if err != nil {
		return ai.Request{}, err
	}

	prompt := s.basePrompt(ctx)
	sawUser := false
	for _, message := range history {
		if message.ID == userMsg.ID {
			sawUser = true
		}
		switch message.Role {
		case RoleUser:
			prompt.WriteString("\n\nUser: ")
			prompt.WriteString(message.Content)
		case RoleAssistant:
			prompt.WriteString("\n\nAssistant: ")
			prompt.WriteString(message.Content)
		case RoleToolCall:
			prompt.WriteString("\n\nAssistant: ")
			prompt.WriteString(`{"kind":"tool","tool":` + jsonString(message.ToolName) +
				`,"arguments":` + orEmptyJSON(message.ToolArguments) + "}")
		case RoleToolRes:
			prompt.WriteString("\n\nTool result (" + message.ToolName + "): ")
			// Current-turn results stay whole (the 64 KiB budget already
			// clipped them at landing); older ones are clipped so eight past
			// tools cannot refill the context.
			if message.ID > userMsg.ID {
				prompt.WriteString(message.Content)
			} else {
				prompt.WriteString(clipHistory(message.Content))
			}
		}
	}
	// The user row is always inside the latest page (a turn adds at most 17
	// rows after it, the page holds 200), but a prompt that silently lost the
	// question is worse than one duplicate check.
	if !sawUser {
		prompt.WriteString("\n\nUser: ")
		prompt.WriteString(userMsg.Content)
	}
	if extra != "" {
		prompt.WriteString(extra)
	}

	return ai.Request{
		Purpose:         ai.PurposeChat,
		Parts:           []ai.Part{ai.TextPart(prompt.String())},
		MaxOutputTokens: maxOutputTokens,
	}, nil
}

// basePrompt renders the agent system prompt plus the global memory.
func (s *Service) basePrompt(ctx context.Context) *strings.Builder {
	now := time.Now()
	today, monday := todayAndMonday(now)
	// The prompt dates are a convenience for the model, derived once per
	// request; exact logical-day arithmetic stays in timeutil at the
	// executor, where day arguments are validated.
	editMode := ""
	if s.settings != nil {
		if mode, err := s.settings.EditMode(ctx); err == nil {
			editMode = mode
		}
	}
	prompt := &strings.Builder{}
	prompt.WriteString(agentSystemPrompt(normalizeEditMode(editMode), today, monday))
	if s.settings != nil {
		if memory, err := s.settings.Memory(ctx); err == nil && strings.TrimSpace(memory) != "" {
			prompt.WriteString("\n\n用户的全局指令：\n")
			prompt.WriteString(memory)
		}
	}
	return prompt
}

// todayAndMonday derives the prompt's convenience dates from timeutil: today
// is the logical day (4 AM boundary) containing t, monday the Monday of that
// logical day's week.
func todayAndMonday(t time.Time) (today, monday string) {
	today = timeutil.LogicalDay(t, t.Location())
	monday, err := timeutil.WeekStart(today, t.Location())
	if err != nil {
		monday = today
	}
	return today, monday
}

// jsonString quotes a value for inline JSON in the prompt.
func jsonString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// orEmptyJSON substitutes {} for a blank arguments string so the rendered
// envelope stays parseable.
func orEmptyJSON(value string) string {
	if strings.TrimSpace(value) == "" {
		return "{}"
	}
	return value
}

// clipHistory bounds a past tool result replayed into the prompt.
func clipHistory(content string) string {
	if len(content) <= historyToolResultLimit {
		return content
	}
	return content[:historyToolResultLimit] + "…（已截断）"
}

// complete persists the assistant message and clears the turn. Notifications
// fire after the write commits.
func (s *Service) complete(conversationID string, message Message) {
	_, err := s.store.AppendMessage(context.Background(), conversationID, message)
	s.finishTurnID(conversationID)
	if err == nil && s.notifier != nil {
		s.notifier(conversationID)
	}
}

// SetNotifier installs the completion callback. Unexported: the emitter is
// the app layer's concern.
func (s *Service) SetNotifier(fn func(conversationID string)) {
	s.notifier = fn
}

// Cancel stops the conversation's running turn, if any. Cancelling an idle
// conversation is a no-op.
func (s *Service) Cancel(conversationID string) {
	s.mu.Lock()
	cancel := s.inflight[conversationID]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Conversations lists threads newest-activity first.
func (s *Service) Conversations(ctx context.Context) ([]Conversation, error) {
	return s.store.ListConversations(ctx)
}

// NewConversation creates a thread with a generated id. The thread is pinned
// to the routing chain's primary provider when one exists; provider selection
// is always explicit (decisions/chat-session-model).
func (s *Service) NewConversation(ctx context.Context) (Conversation, error) {
	id, err := newConversationID()
	if err != nil {
		return Conversation{}, err
	}
	c := Conversation{ID: id}
	if chain, err := s.providers.Chain(ctx); err == nil && len(chain) > 0 {
		primary := chain[0].ID
		c.ProviderID = &primary
	}
	return s.store.CreateConversation(ctx, c)
}

// DeleteConversation removes a thread; the store cascades messages.
func (s *Service) DeleteConversation(ctx context.Context, id string) error {
	return s.store.DeleteConversation(ctx, id)
}

// SetConversationProvider pins a thread to one provider. providerID "" clears
// the pin; the thread then has no provider until one is picked again. Changing
// the pin resets the model override: a model chosen under the previous
// provider has no meaning under the new one.
func (s *Service) SetConversationProvider(ctx context.Context, id string, providerID string) error {
	var pinned *string
	if providerID != "" {
		if _, err := s.providers.ByID(ctx, providerID); err != nil {
			return err
		}
		pinned = &providerID
	}
	conversation, err := s.store.GetConversation(ctx, id)
	if err != nil {
		return err
	}
	model := conversation.Model
	if providerID != conversation.providerKey() {
		model = ""
	}
	return s.store.UpdateConversation(ctx, id, conversation.Title, pinned, model)
}

// SetConversationModel sets the thread's model override. model "" follows the
// provider's configured model. Setting a model on a thread with no provider is
// rejected: there is nothing for the override to apply to.
func (s *Service) SetConversationModel(ctx context.Context, id string, model string) error {
	model = strings.TrimSpace(model)
	conversation, err := s.store.GetConversation(ctx, id)
	if err != nil {
		return err
	}
	if conversation.ProviderID == nil || *conversation.ProviderID == "" {
		return fmt.Errorf("chat: conversation has no provider selected")
	}
	if model != "" && len(model) > maxModelBytes {
		return fmt.Errorf("chat: model name exceeds %d bytes", maxModelBytes)
	}
	return s.store.UpdateConversation(ctx, id, conversation.Title, conversation.ProviderID, model)
}

// providerKey renders the pinned provider id, "" when none.
func (c Conversation) providerKey() string {
	if c.ProviderID == nil {
		return ""
	}
	return *c.ProviderID
}

// Messages pages one conversation's transcript, oldest first.
func (s *Service) Messages(ctx context.Context, conversationID string, beforeID int64, limit int) ([]Message, error) {
	return s.store.Messages(ctx, conversationID, beforeID, limit)
}

func (s *Service) finishTurn(conversationID string, cancel context.CancelFunc) {
	cancel()
	s.mu.Lock()
	delete(s.inflight, conversationID)
	s.mu.Unlock()
}

func (s *Service) finishTurnID(conversationID string) {
	s.mu.Lock()
	if cancel, ok := s.inflight[conversationID]; ok {
		cancel()
		delete(s.inflight, conversationID)
	}
	s.mu.Unlock()
}

// newConversationID generates an opaque thread id.
func newConversationID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("chat: generate conversation id: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// titleFrom derives a thread title from the first user message.
func titleFrom(content string) string {
	title := strings.TrimSpace(content)
	if len(title) > 30 {
		title = title[:30]
	}
	return title
}

// failureText maps an ai error to the stored failure message. The ai layer's
// messages are fixed sanitized strings; raw provider bodies never reach here.
func failureText(err error) string {
	if err == nil {
		return ""
	}
	if err == ai.ErrNoProvider {
		return "没有已配置的供应商；请先在设置中添加。"
	}
	if err == errNoProviderSelected {
		return "该会话尚未选择供应商，请先在右上角选择。"
	}
	switch ai.ErrorKindOf(err) {
	case ai.ErrorCanceled:
		return "已取消。"
	default:
		return strings.TrimPrefix(err.Error(), "ai: ")
	}
}
