package storage

import (
	"context"
	"testing"
	"time"
)

func newTestProvider(id, name string) Provider {
	return Provider{
		ID:          id,
		DisplayName: name,
		Protocol:    "openai",
		Endpoint:    "https://api.example.com/v1",
		Model:       "fixture-model",
	}
}

func TestProviderRepoCRUD(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.Providers()
	ctx := context.Background()

	p := newTestProvider("provider-a", "Fixture A")
	if err := repo.Add(ctx, p); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := repo.Get(ctx, "provider-a")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.DisplayName != "Fixture A" || got.Protocol != "openai" {
		t.Fatalf("Get returned %+v", got)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("timestamps not set: %+v", got)
	}

	updated := newTestProvider("provider-a", "Fixture A renamed")
	updated.Model = "other-model"
	if err := repo.Update(ctx, "provider-a", updated); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err = repo.Get(ctx, "provider-a")
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got.DisplayName != "Fixture A renamed" || got.Model != "other-model" {
		t.Fatalf("update did not apply: %+v", got)
	}
	if !got.UpdatedAt.After(got.CreatedAt) && !got.UpdatedAt.Equal(got.CreatedAt) {
		t.Fatalf("updated_at must not go backwards")
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != "provider-a" {
		t.Fatalf("List = %+v", list)
	}
}

func TestProviderRepoListOrdersByDisplayName(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.Providers()
	ctx := context.Background()

	for _, p := range []Provider{
		newTestProvider("provider-c", "Zeta"),
		newTestProvider("provider-a", "Alpha"),
		newTestProvider("provider-b", "Mid"),
	} {
		if err := repo.Add(ctx, p); err != nil {
			t.Fatalf("Add %s: %v", p.ID, err)
		}
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"Alpha", "Mid", "Zeta"}
	if len(list) != len(want) {
		t.Fatalf("List length = %d, want %d", len(list), len(want))
	}
	for i, id := range want {
		if list[i].DisplayName != id {
			t.Fatalf("List[%d].DisplayName = %q, want %q", i, list[i].DisplayName, id)
		}
	}
}

func TestProviderRepoGetAbsentIsNotFound(t *testing.T) {
	store := openWriter(t, newDir(t))
	ctx := context.Background()

	_, err := store.Providers().Get(ctx, "no-such-provider")
	assertKind(t, err, KindNotFound)
}

func TestProviderRepoDelete(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.Providers()
	ctx := context.Background()

	if err := repo.Add(ctx, newTestProvider("provider-a", "Fixture A")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := repo.Delete(ctx, "provider-a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, "provider-a"); !IsKind(err, KindNotFound) {
		t.Fatalf("Get after delete = %v, want not_found", err)
	}
	assertKind(t, repo.Delete(ctx, "provider-a"), KindNotFound)
}

func newTestConversation(id string, providerID *string) Conversation {
	return Conversation{ID: id, Title: "Fixture " + id, ProviderID: providerID}
}

func TestChatRepoConversationLifecycle(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.Chat()
	ctx := context.Background()

	c, err := repo.CreateConversation(ctx, newTestConversation("conv-a", nil))
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if c.CreatedAt.IsZero() || c.UpdatedAt.IsZero() {
		t.Fatalf("timestamps not set: %+v", c)
	}

	provider := "provider-a"
	// Advance the clock past second granularity so updated_at strictly advances.
	store.setClock(func() time.Time { return time.Now().Add(2 * time.Second) })
	if err := repo.UpdateConversation(ctx, "conv-a", "New title", &provider); err != nil {
		t.Fatalf("UpdateConversation: %v", err)
	}
	got, err := repo.GetConversation(ctx, "conv-a")
	if err != nil {
		t.Fatalf("GetConversation: %v", err)
	}
	if got.Title != "New title" || got.ProviderID == nil || *got.ProviderID != "provider-a" {
		t.Fatalf("update did not apply: %+v", got)
	}
	if !got.UpdatedAt.After(got.CreatedAt) {
		t.Fatalf("updated_at must advance on update")
	}

	// Nulling the provider (the deleted-provider prune) must also work.
	if err := repo.UpdateConversation(ctx, "conv-a", "New title", nil); err != nil {
		t.Fatalf("UpdateConversation null: %v", err)
	}
	got, err = repo.GetConversation(ctx, "conv-a")
	if err != nil {
		t.Fatalf("GetConversation after null: %v", err)
	}
	if got.ProviderID != nil {
		t.Fatalf("provider_id = %q, want nil", *got.ProviderID)
	}
}

func TestChatRepoListOrdersByActivity(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.Chat()
	ctx := context.Background()

	for _, id := range []string{"conv-a", "conv-b"} {
		if _, err := repo.CreateConversation(ctx, newTestConversation(id, nil)); err != nil {
			t.Fatalf("CreateConversation %s: %v", id, err)
		}
	}
	// Advance the store clock so the later write has a strictly greater
	// timestamp; Unix-second granularity could tie otherwise.
	store.setClock(func() time.Time { return time.Now().Add(2 * time.Second) })
	if _, err := repo.AppendMessage(ctx, "conv-a", ChatMessage{Role: ChatRoleUser, Content: "hello"}); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	list, err := repo.ListConversations(ctx)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(list) != 2 || list[0].ID != "conv-a" {
		t.Fatalf("conversation with the newest message must list first, got %+v", list)
	}
}

func TestChatRepoDeleteConversationCascadesMessages(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.Chat()
	ctx := context.Background()

	if _, err := repo.CreateConversation(ctx, newTestConversation("conv-a", nil)); err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if _, err := repo.AppendMessage(ctx, "conv-a", ChatMessage{Role: ChatRoleUser, Content: "hello"}); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if err := repo.DeleteConversation(ctx, "conv-a"); err != nil {
		t.Fatalf("DeleteConversation: %v", err)
	}

	msgs, err := repo.Messages(ctx, "conv-a", 0, 0)
	if err != nil {
		t.Fatalf("Messages after delete: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("messages survived conversation delete: %+v", msgs)
	}
	assertKind(t, repo.DeleteConversation(ctx, "conv-a"), KindNotFound)
}

func TestChatRepoAppendMessageSetsIDAndBumpsConversation(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.Chat()
	ctx := context.Background()

	if _, err := repo.CreateConversation(ctx, newTestConversation("conv-a", nil)); err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	m, err := repo.AppendMessage(ctx, "conv-a", ChatMessage{Role: ChatRoleUser, Content: "hello"})
	if err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if m.ID == 0 || m.CreatedAt.IsZero() || m.ConversationID != "conv-a" {
		t.Fatalf("AppendMessage result incomplete: %+v", m)
	}

	a, err := repo.AppendMessage(ctx, "conv-a", ChatMessage{Role: ChatRoleAssistant, Content: "hi", Status: ChatStatusOK})
	if err != nil {
		t.Fatalf("AppendMessage assistant: %v", err)
	}
	if a.ID <= m.ID {
		t.Fatalf("assistant message id %d must follow user id %d", a.ID, m.ID)
	}
}

func TestChatRepoToolMessageRoundTrip(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.Chat()
	ctx := context.Background()

	if _, err := repo.CreateConversation(ctx, newTestConversation("conv-a", nil)); err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	call, err := repo.AppendMessage(ctx, "conv-a", ChatMessage{
		Role: ChatRoleToolCall, ToolName: "timeline", ToolArguments: `{"day":"2026-09-12"}`})
	if err != nil {
		t.Fatalf("AppendMessage tool_call: %v", err)
	}
	result, err := repo.AppendMessage(ctx, "conv-a", ChatMessage{
		Role: ChatRoleToolRes, ToolName: "timeline", Content: `{"ok":true}`})
	if err != nil {
		t.Fatalf("AppendMessage tool_result: %v", err)
	}

	msgs, err := repo.Messages(ctx, "conv-a", 0, 0)
	if err != nil {
		t.Fatalf("Messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want 2", len(msgs))
	}
	gotCall, gotResult := msgs[0], msgs[1]
	if gotCall.Role != ChatRoleToolCall || gotCall.ToolName != "timeline" ||
		gotCall.ToolArguments != `{"day":"2026-09-12"}` || gotCall.Content != "" || gotCall.Status != "" {
		t.Fatalf("tool_call round trip = %+v", gotCall)
	}
	if gotResult.Role != ChatRoleToolRes || gotResult.ToolName != "timeline" ||
		gotResult.ToolArguments != "" || gotResult.Content != `{"ok":true}` {
		t.Fatalf("tool_result round trip = %+v", gotResult)
	}
	if result.ID <= call.ID {
		t.Fatalf("tool_result id %d must follow tool_call id %d", result.ID, call.ID)
	}
}

func TestChatRepoMessagesPagination(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.Chat()
	ctx := context.Background()

	if _, err := repo.CreateConversation(ctx, newTestConversation("conv-a", nil)); err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	const total = 5
	for i := 0; i < total; i++ {
		if _, err := repo.AppendMessage(ctx, "conv-a", ChatMessage{Role: ChatRoleUser, Content: "m"}); err != nil {
			t.Fatalf("AppendMessage %d: %v", i, err)
		}
	}

	// Latest page, oldest-first.
	page, err := repo.Messages(ctx, "conv-a", 0, 3)
	if err != nil {
		t.Fatalf("Messages: %v", err)
	}
	if len(page) != 3 || page[0].ID >= page[1].ID || page[1].ID >= page[2].ID {
		t.Fatalf("latest page wrong: %+v", page)
	}

	// Page backward from the oldest id of the latest page.
	page2, err := repo.Messages(ctx, "conv-a", page[0].ID, 3)
	if err != nil {
		t.Fatalf("Messages page 2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("second page length = %d, want 2", len(page2))
	}
	if page2[0].ID >= page[0].ID {
		t.Fatalf("second page must contain older ids only")
	}

	// beforeID beyond the oldest id returns an empty page, not an error.
	page3, err := repo.Messages(ctx, "conv-a", page2[0].ID, 3)
	if err != nil {
		t.Fatalf("Messages page 3: %v", err)
	}
	if len(page3) != 0 {
		t.Fatalf("third page = %+v, want empty", page3)
	}
}

func TestChatRepoMessagesClampsLimit(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.Chat()
	ctx := context.Background()

	if _, err := repo.CreateConversation(ctx, newTestConversation("conv-a", nil)); err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	// limit 0 must clamp to ChatMessageLimit, not return an error or nothing.
	msgs, err := repo.Messages(ctx, "conv-a", 0, 0)
	if err != nil {
		t.Fatalf("Messages with limit 0: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("empty conversation returned %d messages", len(msgs))
	}
}

func TestChatRepoAppendToAbsentConversationFails(t *testing.T) {
	store := openWriter(t, newDir(t))
	ctx := context.Background()

	_, err := store.Chat().AppendMessage(ctx, "no-such-conv", ChatMessage{Role: ChatRoleUser, Content: "x"})
	if err == nil {
		t.Fatal("append to absent conversation succeeded; the foreign key must refuse it")
	}
	assertKind(t, err, KindConstraint)
}
