package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
		fmt.Fprint(w, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"output_index\":0,\"content_index\":0,\"delta\":\"ignored partial\"}\n\n")
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

// Plan inference can omit Content-Type and omit repeated output in the final
// response. Text still belongs to the stream; only completed commits it.
func TestSubscriptionSparseCompletion(t *testing.T) {
	for _, tc := range []struct {
		name, contentType, events string
	}{
		{"text-done-no-header", "", `data: {"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"partial"}

data: {"type":"response.output_text.done","output_index":0,"content_index":0,"text":"{\"kind\":\"answer\",\"answer\":\"Hello\"}"}

`},
		{"item-done", "text/event-stream; charset=utf-8", `data: {"type":"response.output_item.done","output_index":0,"item":{"type":"message","status":"completed","content":[{"type":"output_text","text":"{\"kind\":\"answer\",\"answer\":\"Hello\"}"}]}}

`},
		{"delta-only", "text/event-stream", `data: {"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"{\"kind\":\"answer\","}

data: {"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"\"answer\":\"Hello\"}"}

`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := subscriptionFixtureTransport{header: tc.contentType, body: tc.events + `data: {"type":"response.completed","response":{"status":"completed","output":[],"model":"fixture","usage":{"input_tokens":2,"output_tokens":3}}}` + "\n\n"}
			c, _ := NewSubscriptionClient(&http.Client{Transport: transport}, "fixture", func(context.Context) (string, error) { return "fixture-token", nil })
			result, err := c.Generate(context.Background(), ai.Request{Parts: []ai.Part{ai.TextPart("anonymous fixture")}, Output: &ai.OutputSchema{Name: "chat_fixture", Schema: json.RawMessage(`{"type":"object","properties":{"kind":{"const":"answer"},"answer":{"type":"string"}},"required":["kind","answer"],"additionalProperties":false}`)}})
			if err != nil || string(result.JSON) != `{"kind":"answer","answer":"Hello"}` || result.Model != "fixture" || result.Usage.OutputTokens == nil || *result.Usage.OutputTokens != 3 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

type subscriptionFixtureTransport struct{ header, body string }

func (f subscriptionFixtureTransport) RoundTrip(*http.Request) (*http.Response, error) {
	h := make(http.Header)
	if f.header != "" {
		h.Set("Content-Type", f.header)
	}
	return &http.Response{StatusCode: http.StatusOK, Header: h, Body: io.NopCloser(strings.NewReader(f.body))}, nil
}

func TestSubscriptionStreamOrdersAndDeduplicatesText(t *testing.T) {
	body := ""
	for _, e := range []string{
		`{"type":"response.output_text.delta","output_index":2,"content_index":1,"delta":"C"}`,
		`{"type":"response.output_text.delta","output_index":2,"content_index":0,"delta":"B"}`,
		`{"type":"response.output_text.done","output_index":0,"content_index":0,"text":"ignored"}`,
		`{"type":"response.output_item.done","output_index":0,"item":{"type":"message","status":"completed","content":[{"type":"output_text","text":"A"}]}}`,
		`{"type":"response.completed","response":{"status":"completed"}}`,
	} {
		body += "data: " + e + "\n\n"
	}
	c, _ := NewSubscriptionClient(&http.Client{Transport: subscriptionFixtureTransport{body: body}}, "fixture", func(context.Context) (string, error) { return "fixture-token", nil })
	result, err := c.Generate(context.Background(), ai.Request{Parts: []ai.Part{ai.TextPart("anonymous fixture")}})
	if err != nil || result.Text != "ABC" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestSubscriptionStreamTextDoesNotBypassFailure(t *testing.T) {
	text := `data: {"type":"response.output_text.done","output_index":0,"content_index":0,"text":"private fixture-token"}` + "\n\n"
	for _, tc := range []struct {
		name, header, end string
		limit             bool
	}{
		{"interrupted", "", "", false},
		{"quota-after-text", "", `data: {"type":"response.failed","response":{"error":{"code":"subscription_sharing_usage_limit_exceeded"}}}`, true},
		{"incomplete-after-text", "", `data: {"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}}}`, false},
		{"missing-status", "", `data: {"type":"response.completed","response":{}}`, false},
		{"refusal", "", "data: {\"type\":\"response.refusal.done\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}", false},
		{"failed-message", "", `data: {"type":"response.output_item.done","output_index":0,"item":{"type":"message","status":"incomplete","content":[{"type":"output_text","text":"private"}]}}` + "\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}", false},
		{"explicit-wrong-header", "application/json", `data: {"type":"response.completed","response":{"status":"completed"}}`, false},
		{"invalid-envelope", "", `data: {"type":"response.completed","response":{"status":"completed"}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := NewSubscriptionClient(&http.Client{Transport: subscriptionFixtureTransport{header: tc.header, body: text + tc.end + "\n\n"}}, "fixture", func(context.Context) (string, error) { return "fixture-token", nil })
			req := ai.Request{Parts: []ai.Part{ai.TextPart("anonymous fixture")}}
			if tc.name == "invalid-envelope" {
				req.Output = &ai.OutputSchema{Name: "fixture", Schema: json.RawMessage(`{"type":"object"}`)}
			}
			result, err := c.Generate(context.Background(), req)
			if err == nil || result.Text != "" || result.JSON != nil || strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "fixture-token") {
				t.Fatalf("unsafe result=%+v err=%v", result, err)
			}
			if tc.limit && (ai.ErrorKindOf(err) != ai.ErrorRateLimited || ai.Retryable(err)) {
				t.Fatalf("quota misclassified: %v", err)
			}
		})
	}
}

func TestSubscriptionStreamRedactsTextCredential(t *testing.T) {
	body := `data: {"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"fixture-token"}` + "\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n"
	c, _ := NewSubscriptionClient(&http.Client{Transport: subscriptionFixtureTransport{body: body}}, "fixture", func(context.Context) (string, error) { return "fixture-token", nil })
	result, err := c.Generate(context.Background(), ai.Request{Parts: []ai.Part{ai.TextPart("anonymous fixture")}})
	if err != nil || result.Text != "[redacted]" {
		t.Fatalf("unsafe result=%+v err=%v", result, err)
	}
}
