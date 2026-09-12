package chat

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// newOpenAIServer runs a minimal Chat Completions endpoint that answers with
// fn(prompt). The prompt is reconstructed from the messages array so tests
// can assert on what the service actually sent.
func newOpenAIServer(t *testing.T, fn func(prompt string) string) *httptest.Server {
	t.Helper()
	var ignored atomic.Int64
	return newOpenAIServerWithCalls(t, fn, &ignored)
}

func newOpenAIServerWithCalls(t *testing.T, fn func(prompt string) string, calls *atomic.Int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var payload struct {
			Messages []struct {
				Role string `json:"role"`
				// The openai client sends content as a parts array
				// ([{"type":"text","text":"..."}]), matching its Generate path.
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var prompt strings.Builder
		for _, message := range payload.Messages {
			for _, part := range message.Content {
				prompt.WriteString(part.Text)
			}
			prompt.WriteString("\n")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "m",
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": fn(prompt.String())}},
			},
		})
	}))
}

func newOpenAIFailingServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))
}
