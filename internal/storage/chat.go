package storage

import (
	"context"
	"database/sql"
	"time"
)

// Chat roles and assistant statuses are the closed sets from docs/05 §5.12:
// this slice stores only user/assistant; tool_call and tool_result arrive with
// the agent slice.
const (
	ChatRoleUser      = "user"
	ChatRoleAssistant = "assistant"

	ChatStatusOK       = "ok"
	ChatStatusFailed   = "failed"
	ChatStatusCanceled = "canceled"
)

// ChatMessageLimit caps one page of Messages, bounding the work a runaway
// caller can request.
const ChatMessageLimit = 200

// Conversation is one row of chat_conversations. ProviderID nil means no
// provider is selected yet; chat never implicitly falls back to the chain
// (decisions/chat-session-model).
type Conversation struct {
	ID         string
	Title      string
	ProviderID *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ChatMessage is one row of chat_messages. Status is set only for assistant
// messages; user messages leave it empty.
type ChatMessage struct {
	ID             int64
	ConversationID string
	Role           string
	Content        string
	Status         string
	CreatedAt      time.Time
}

// ChatRepo is the typed access layer over the chat tables (docs/03 §3.3.4).
type ChatRepo struct {
	store *Store
}

// Chat returns the repository bound to this store.
func (s *Store) Chat() *ChatRepo {
	if s == nil {
		return nil
	}
	return &ChatRepo{store: s}
}

// CreateConversation inserts a conversation and returns it with timestamps.
func (r *ChatRepo) CreateConversation(ctx context.Context, c Conversation) (Conversation, error) {
	at := r.store.now()
	err := r.store.Write(ctx, "chat create conversation", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO chat_conversations (id, title, provider_id, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?)`,
			c.ID, c.Title, c.ProviderID, at.Unix(), at.Unix())
		return err
	})
	if err != nil {
		return Conversation{}, err
	}
	c.CreatedAt = at
	c.UpdatedAt = at
	return c, nil
}

// ListConversations returns conversations newest-activity first: the sidebar
// shows the most recently used conversation at the top.
func (r *ChatRepo) ListConversations(ctx context.Context) ([]Conversation, error) {
	var out []Conversation
	err := r.store.Read(ctx, "chat list conversations", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			`SELECT id, title, provider_id, created_at, updated_at
			 FROM chat_conversations ORDER BY updated_at DESC, id`)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			c, err := scanConversation(rows)
			if err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetConversation returns one conversation by id.
func (r *ChatRepo) GetConversation(ctx context.Context, id string) (Conversation, error) {
	var c Conversation
	err := r.store.Read(ctx, "chat get conversation", func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx,
			`SELECT id, title, provider_id, created_at, updated_at
			 FROM chat_conversations WHERE id = ?`, id)
		var err error
		c, err = scanConversation(row)
		return err
	})
	if err != nil {
		return Conversation{}, err
	}
	return c, nil
}

// DeleteConversation removes one conversation; chat_messages rows follow via
// ON DELETE CASCADE.
func (r *ChatRepo) DeleteConversation(ctx context.Context, id string) error {
	return r.store.Write(ctx, "chat delete conversation", func(ctx context.Context, tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM chat_conversations WHERE id = ?`, id)
		if err != nil {
			return err
		}
		return requireUpdated(res, "chat delete conversation")
	})
}

// UpdateConversation sets the title and provider of one conversation and bumps
// updated_at. Deleting a provider prunes it from conversations by calling this
// with a nil ProviderID.
func (r *ChatRepo) UpdateConversation(ctx context.Context, id string, title string, providerID *string) error {
	at := r.store.now()
	return r.store.Write(ctx, "chat update conversation", func(ctx context.Context, tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE chat_conversations SET title = ?, provider_id = ?, updated_at = ? WHERE id = ?`,
			title, providerID, at.Unix(), id)
		if err != nil {
			return err
		}
		return requireUpdated(res, "chat update conversation")
	})
}

// AppendMessage inserts one message and returns it with its ID and timestamp.
// It also bumps the conversation's updated_at so ListConversations ordering
// tracks activity, in the same transaction: a message landing without the bump
// would strand a conversation mid-list.
func (r *ChatRepo) AppendMessage(ctx context.Context, conversationID string, m ChatMessage) (ChatMessage, error) {
	at := r.store.now()
	err := r.store.Write(ctx, "chat append message", func(ctx context.Context, tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO chat_messages (conversation_id, role, content, status, created_at)
			 VALUES (?, ?, ?, ?, ?)`,
			conversationID, m.Role, m.Content, m.Status, at.Unix())
		if err != nil {
			return err
		}
		if m.ID, err = res.LastInsertId(); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE chat_conversations SET updated_at = ? WHERE id = ?`, at.Unix(), conversationID)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return ChatMessage{}, err
	}
	m.ConversationID = conversationID
	m.CreatedAt = at
	return m, nil
}

// Messages returns one conversation's messages oldest-first. beforeID > 0
// pages backward (rows with id < beforeID); beforeID 0 returns the latest
// page. limit <= 0 or > ChatMessageLimit is clamped to ChatMessageLimit.
func (r *ChatRepo) Messages(ctx context.Context, conversationID string, beforeID int64, limit int) ([]ChatMessage, error) {
	if limit <= 0 || limit > ChatMessageLimit {
		limit = ChatMessageLimit
	}
	query := `SELECT id, conversation_id, role, content, status, created_at
		FROM chat_messages WHERE conversation_id = ?`
	args := []any{conversationID}
	if beforeID > 0 {
		query += ` AND id < ?`
		args = append(args, beforeID)
	}
	// Newest rows first, reversed after the scan, so "latest page" is the
	// highest ids without a second max(id) query.
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)

	var out []ChatMessage
	err := r.store.Read(ctx, "chat messages", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, query, args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			m, err := scanChatMessage(rows)
			if err != nil {
				return err
			}
			out = append(out, m)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	// Reverse to oldest-first for the caller.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func scanConversation(row scanner) (Conversation, error) {
	var c Conversation
	var title, providerID sql.NullString
	var createdAt, updatedAt int64
	if err := row.Scan(&c.ID, &title, &providerID, &createdAt, &updatedAt); err != nil {
		return Conversation{}, err
	}
	c.Title = title.String
	if providerID.Valid {
		id := providerID.String
		c.ProviderID = &id
	}
	c.CreatedAt = time.Unix(createdAt, 0)
	c.UpdatedAt = time.Unix(updatedAt, 0)
	return c, nil
}

func scanChatMessage(row scanner) (ChatMessage, error) {
	var m ChatMessage
	var status sql.NullString
	var createdAt int64
	if err := row.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &status, &createdAt); err != nil {
		return ChatMessage{}, err
	}
	m.Status = status.String
	m.CreatedAt = time.Unix(createdAt, 0)
	return m, nil
}
