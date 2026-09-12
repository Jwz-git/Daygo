package app

import (
	"context"
	"strings"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/chat"
	"github.com/Jwz-git/Daygo/internal/platform/secrets"
	"github.com/Jwz-git/Daygo/internal/settings"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// chatTimeout bounds one synchronous chat binding call. SendChatMessage
// returns after the user message is stored; the assistant turn itself runs
// on the service's own context.
const chatTimeout = 10 * time.Second

// ChatConversationDTO is one conversation list row.
type ChatConversationDTO struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	ProviderID string `json:"providerId"` // "" = follow the routing chain
	UpdatedAt  int64  `json:"updatedAt"`
}

// ChatMessageDTO is one transcript row. Status is set for assistant messages
// only (ok | failed | canceled).
type ChatMessageDTO struct {
	ID        int64  `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"createdAt"`
}

// chatService returns the service, wiring it on first use. Wiring here rather
// than at startup keeps headless tests and a store-less second instance
// working without special cases: no store means the error below, not a nil
// dereference.
func (b *Backend) chatService() (*chat.Service, error) {
	store := b.store()
	if store == nil {
		if err := b.storageFailure(); err != nil {
			return nil, mapStorageError("open chat", err)
		}
		return nil, apperr.E(apperr.DatabaseError, "chat requires a database", nil)
	}

	b.chatMu.Lock()
	defer b.chatMu.Unlock()
	if b.chat != nil {
		return b.chat, nil
	}
	if b.secrets == nil {
		// Chat without a keychain cannot read any provider's secret; reporting
		// native_unavailable is more honest than a service that fails every
		// turn with an opaque error.
		return nil, apperr.E(apperr.NativeUnavailable, "keychain is unavailable", nil)
	}
	service := chat.New(storeChatAdapter{repo: store.Chat()}, backendProviders{backend: b}, backendChatSettings{backend: b})
	service.SetNotifier(func(conversationID string) {
		b.emitter.Emit(EventChatUpdated, ChatUpdatedPayload{ConversationID: conversationID})
	})
	b.chat = service
	return service, nil
}

// ListChatConversations returns threads newest-activity first.
func (b *Backend) ListChatConversations() ([]ChatConversationDTO, error) {
	service, err := b.chatService()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), chatTimeout)
	defer cancel()

	conversations, err := service.Conversations(ctx)
	if err != nil {
		return nil, mapStorageError("list conversations", err)
	}
	out := make([]ChatConversationDTO, 0, len(conversations))
	for _, c := range conversations {
		out = append(out, conversationToDTO(c))
	}
	return out, nil
}

// CreateChatConversation starts a new empty thread.
func (b *Backend) CreateChatConversation() (ChatConversationDTO, error) {
	service, err := b.chatService()
	if err != nil {
		return ChatConversationDTO{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), chatTimeout)
	defer cancel()

	conversation, err := service.NewConversation(ctx)
	if err != nil {
		return ChatConversationDTO{}, mapStorageError("create conversation", err)
	}
	return conversationToDTO(conversation), nil
}

// DeleteChatConversation removes a thread; messages cascade.
func (b *Backend) DeleteChatConversation(id string) error {
	service, err := b.chatService()
	if err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return apperr.E(apperr.InvalidArgument, "conversation id is required", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), chatTimeout)
	defer cancel()

	if err := service.DeleteConversation(ctx, id); err != nil {
		return mapStorageError("delete conversation", err)
	}
	b.emitter.Emit(EventChatUpdated, ChatUpdatedPayload{ConversationID: id})
	return nil
}

// SetChatConversationProvider pins a thread to one provider; "" returns it to
// the routing chain.
func (b *Backend) SetChatConversationProvider(id string, providerID string) error {
	service, err := b.chatService()
	if err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return apperr.E(apperr.InvalidArgument, "conversation id is required", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), chatTimeout)
	defer cancel()

	if err := service.SetConversationProvider(ctx, id, strings.TrimSpace(providerID)); err != nil {
		if isChatProviderMissing(err) {
			return apperr.E(apperr.InvalidArgument, "unknown provider", nil)
		}
		return mapStorageError("set conversation provider", err)
	}
	b.emitter.Emit(EventChatUpdated, ChatUpdatedPayload{ConversationID: id})
	return nil
}

// GetChatMessages pages one thread's transcript, oldest first. beforeID > 0
// pages backward; 0 returns the latest page.
func (b *Backend) GetChatMessages(conversationID string, beforeID int64, limit int) ([]ChatMessageDTO, error) {
	service, err := b.chatService()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(conversationID) == "" {
		return nil, apperr.E(apperr.InvalidArgument, "conversation id is required", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), chatTimeout)
	defer cancel()

	messages, err := service.Messages(ctx, conversationID, beforeID, limit)
	if err != nil {
		return nil, mapStorageError("read messages", err)
	}
	out := make([]ChatMessageDTO, 0, len(messages))
	for _, m := range messages {
		out = append(out, ChatMessageDTO{
			ID:        m.ID,
			Role:      m.Role,
			Content:   m.Content,
			Status:    m.Status,
			CreatedAt: m.CreatedAt.Unix(),
		})
	}
	return out, nil
}

// SendChatMessage appends the user message and starts one assistant turn.
// It returns once the user message is persisted; the reply arrives as the
// chat:updated event, after which the frontend re-pulls via GetChatMessages.
func (b *Backend) SendChatMessage(conversationID string, content string) error {
	service, err := b.chatService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), chatTimeout)
	defer cancel()

	err = service.Send(ctx, conversationID, content)
	if err == nil || err == chat.ErrTurnInFlight {
		if err == chat.ErrTurnInFlight {
			return apperr.E(apperr.InvalidArgument, "a turn is already running in this conversation", nil)
		}
		b.emitter.Emit(EventChatUpdated, ChatUpdatedPayload{ConversationID: conversationID})
		return nil
	}
	if strings.Contains(err.Error(), "chat: message") {
		return apperr.E(apperr.InvalidArgument, err.Error(), nil)
	}
	return mapStorageError("send chat message", err)
}

// CancelChatTurn stops the conversation's running turn. Cancelling an idle
// conversation is a no-op.
func (b *Backend) CancelChatTurn(conversationID string) error {
	service, err := b.chatService()
	if err != nil {
		return err
	}
	if strings.TrimSpace(conversationID) == "" {
		return apperr.E(apperr.InvalidArgument, "conversation id is required", nil)
	}
	service.Cancel(conversationID)
	return nil
}

func conversationToDTO(c chat.Conversation) ChatConversationDTO {
	dto := ChatConversationDTO{
		ID:        c.ID,
		Title:     c.Title,
		UpdatedAt: c.UpdatedAt.Unix(),
	}
	if c.ProviderID != nil {
		dto.ProviderID = *c.ProviderID
	}
	return dto
}

func isChatProviderMissing(err error) bool {
	// The chat service returns the adapter's storage not_found for an unknown
	// pinned id (the adapter maps it below); the raw text check covers both
	// shapes without importing a second error taxonomy.
	return strings.Contains(err.Error(), "not_found") || strings.Contains(err.Error(), "not found")
}

// ChatUpdatedPayload is the chat:updated event body. Invalidation-only: the
// frontend re-pulls via GetChatMessages rather than trusting event data
// (docs/05 §5.5.3).
type ChatUpdatedPayload struct {
	ConversationID string `json:"conversationId"`
}

// storeChatAdapter adapts storage.ChatRepo to chat.Store. The types differ
// only by package; one concept stays one concept, and the conversion is
// mechanical field copying with no logic to drift.
type storeChatAdapter struct {
	repo *storage.ChatRepo
}

func (a storeChatAdapter) CreateConversation(ctx context.Context, c chat.Conversation) (chat.Conversation, error) {
	created, err := a.repo.CreateConversation(ctx, storage.Conversation{
		ID: c.ID, Title: c.Title, ProviderID: c.ProviderID,
	})
	return chatConversationFromStorage(created), err
}

func (a storeChatAdapter) GetConversation(ctx context.Context, id string) (chat.Conversation, error) {
	row, err := a.repo.GetConversation(ctx, id)
	return chatConversationFromStorage(row), err
}

func (a storeChatAdapter) DeleteConversation(ctx context.Context, id string) error {
	return a.repo.DeleteConversation(ctx, id)
}

func (a storeChatAdapter) UpdateConversation(ctx context.Context, id string, title string, providerID *string) error {
	return a.repo.UpdateConversation(ctx, id, title, providerID)
}

func (a storeChatAdapter) ListConversations(ctx context.Context) ([]chat.Conversation, error) {
	rows, err := a.repo.ListConversations(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]chat.Conversation, len(rows))
	for i, row := range rows {
		out[i] = chatConversationFromStorage(row)
	}
	return out, nil
}

func (a storeChatAdapter) AppendMessage(ctx context.Context, conversationID string, m chat.Message) (chat.Message, error) {
	saved, err := a.repo.AppendMessage(ctx, conversationID, storage.ChatMessage{
		Role: m.Role, Content: m.Content, Status: m.Status,
	})
	m.ID = saved.ID
	m.ConversationID = saved.ConversationID
	m.CreatedAt = saved.CreatedAt
	return m, err
}

func (a storeChatAdapter) Messages(ctx context.Context, conversationID string, beforeID int64, limit int) ([]chat.Message, error) {
	rows, err := a.repo.Messages(ctx, conversationID, beforeID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]chat.Message, len(rows))
	for i, row := range rows {
		out[i] = chat.Message{
			ID: row.ID, ConversationID: row.ConversationID, Role: row.Role,
			Content: row.Content, Status: row.Status, CreatedAt: row.CreatedAt,
		}
	}
	return out, nil
}

func chatConversationFromStorage(c storage.Conversation) chat.Conversation {
	return chat.Conversation{
		ID: c.ID, Title: c.Title, ProviderID: c.ProviderID,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

// backendProviders adapts the backend's repos and keychain to chat.Providers.
type backendProviders struct {
	backend *Backend
}

func (p backendProviders) Chain(ctx context.Context) ([]chat.ProviderEntry, error) {
	repo := p.backend.store().Providers()
	routing, err := p.backend.loadRouting(ctx, repo)
	if err != nil {
		return nil, err
	}
	entries := make([]chat.ProviderEntry, 0, len(routing.Chain))
	for _, id := range routing.Chain {
		entry, err := p.entryByID(ctx, repo, id)
		if err != nil {
			// A routing slot whose provider vanished between read and here is
			// skipped, not fatal: the rest of the chain still serves the turn.
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (p backendProviders) ByID(ctx context.Context, id string) (chat.ProviderEntry, error) {
	return p.entryByID(ctx, p.backend.store().Providers(), id)
}

func (p backendProviders) entryByID(ctx context.Context, repo *storage.ProviderRepo, id string) (chat.ProviderEntry, error) {
	row, err := repo.Get(ctx, id)
	if err != nil {
		return chat.ProviderEntry{}, err
	}
	secret, err := p.backend.secrets.Get(ctx, id)
	if err != nil && !secrets.IsNotFound(err) {
		return chat.ProviderEntry{}, err
	}
	return chat.ProviderEntry{
		ID:       row.ID,
		Protocol: row.Protocol,
		Endpoint: row.Endpoint,
		Model:    row.Model,
		Secret:   secret,
	}, nil
}

// backendChatSettings adapts the settings layer to chat.Settings.
type backendChatSettings struct {
	backend *Backend
}

func (s backendChatSettings) Memory(ctx context.Context) (string, error) {
	snapshot, err := settings.New(s.backend.store().Settings()).Load(ctx)
	if err != nil {
		return "", err
	}
	return snapshot.ChatMemory, nil
}
