package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Jwz-git/Daygo/internal/ai"
)

func TestSubscriptionRequestAndCompletedStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("missing scoped credential")
		}
		var p map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Error(err)
		}
		if string(p["store"]) != "false" || string(p["stream"]) != "true" {
			t.Error("subscription transport flags missing")
		}
		if _, ok := p["max_output_tokens"]; ok {
			t.Error("unsupported budget field sent")
		}
		if _, ok := p["text"]; !ok {
			t.Error("structured schema lost")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"ignored partial\"}\n\n")
		fmt.Fprint(w, `data: {"type":"response.completed","response":{"status":"completed","model":"fixture","output":[{"type":"message","status":"completed","content":[{"type":"output_text","text":"{\"ok\":true}"}]}],"usage":{"input_tokens":2,"output_tokens":3}}}`+"\n\n")
	}))
	defer server.Close()
	c, _ := NewSubscriptionClient(server.Client(), "fixture", func(context.Context) (string, error) { return "fixture-token", nil })
	c.endpoint = server.URL
	result, err := c.Generate(context.Background(), ai.Request{Parts: []ai.Part{ai.TextPart("anonymous fixture")}, MaxOutputTokens: 128, Output: &ai.OutputSchema{Name: "fixture", Schema: json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"],"additionalProperties":false}`), Strict: true}})
	if err != nil || string(result.JSON) != `{"ok":true}` || result.Usage.OutputTokens == nil || *result.Usage.OutputTokens != 3 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestSubscriptionRejectsIncompleteAndLimitWithoutRetry(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		retry      bool
	}{
		{"eof", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n", true},
		{"limit", `data: {"type":"response.failed","response":{"error":{"code":"subscription_sharing_usage_limit_exceeded","message":"private"}}}` + "\n\n", false},
		{"incomplete", `data: {"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}}}` + "\n\n", false},
		{"missing-status", `data: {"type":"response.completed","response":{"output":[{"type":"message","content":[{"type":"output_text","text":"private"}]}]}}` + "\n\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, tc.body)
			}))
			defer s.Close()
			c, _ := NewSubscriptionClient(s.Client(), "fixture", func(context.Context) (string, error) { return "key", nil })
			c.endpoint = s.URL
			result, err := c.Generate(context.Background(), ai.Request{Parts: []ai.Part{ai.TextPart("fixture")}})
			if err == nil || result.Text != "" || result.Model != "" || strings.Contains(err.Error(), "private") || ai.Retryable(err) != tc.retry {
				t.Fatalf("unsafe terminal result=%+v err=%v retry=%v", result, err, ai.Retryable(err))
			}
		})
	}
}

func TestSubscriptionNeverFollowsCredentialRedirect(t *testing.T) {
	called := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer source.Close()
	c, _ := NewSubscriptionClient(source.Client(), "fixture", func(context.Context) (string, error) { return "fixture-access", nil })
	c.endpoint = source.URL
	if _, err := c.Generate(context.Background(), ai.Request{Parts: []ai.Part{ai.TextPart("fixture")}}); err == nil || called {
		t.Fatal("credential redirect followed")
	}
}
