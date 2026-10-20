package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Jwz-git/Daygo/internal/ai/openai"
)

type chatSubscriptionTransport struct {
	t     *testing.T
	calls atomic.Int64
}

func (f *chatSubscriptionTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.String() != openai.SubscriptionEndpoint+"/responses" || r.Header.Get("Authorization") != "Bearer anonymous-token" {
		f.t.Error("wrong subscription target or credential")
	}
	var request struct {
		Text struct {
			Format struct {
				Name string `json:"name"`
			} `json:"format"`
		} `json:"text"`
	}
	if json.NewDecoder(r.Body).Decode(&request) != nil || request.Text.Format.Name != envelopeOutput.Name {
		f.t.Error("chat envelope schema missing")
	}
	r.Body.Close()
	reply := `{"kind":"tool","tool":"categories","arguments":{}}`
	if f.calls.Add(1) > 1 {
		reply = `{"kind":"answer","answer":"Anonymous answer"}`
	}
	event, _ := json.Marshal(map[string]any{"type": "response.output_text.done", "output_index": 0, "content_index": 0, "text": reply})
	body := fmt.Sprintf("data: %s\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[]}}\n\n", event)
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
}

func TestChatGPTSubscriptionToolTurnWithSparseCompletion(t *testing.T) {
	transport := &chatSubscriptionTransport{t: t}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous })
	providers := newFakeProviders(ProviderEntry{ID: "subscription", Protocol: "chatgpt", Endpoint: openai.SubscriptionEndpoint, Model: "fixture", TokenSource: func(context.Context) (string, error) { return "anonymous-token", nil }})
	executor := &fakeExecutor{result: func(ToolCall) ToolResult { return okResult(`{"ok":true,"data":[]}`) }}
	service := New(newFakeStore(t), providers, &fakeSettings{editMode: "readonly"}, executor, nil)
	conversation, err := service.NewConversation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Send(context.Background(), conversation.ID, "Anonymous categories question"); err != nil {
		t.Fatal(err)
	}
	waitTurn(t, service, conversation.ID)
	messages, err := service.Messages(context.Background(), conversation.ID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 4 || messages[1].Role != RoleToolCall || messages[2].Role != RoleToolRes || messages[3].Status != StatusOK || messages[3].Content != "Anonymous answer" {
		t.Fatalf("messages=%+v", messages)
	}
	if transport.calls.Load() != 2 || executor.callCount() != 1 {
		t.Fatalf("requests=%d tools=%d", transport.calls.Load(), executor.callCount())
	}
}
