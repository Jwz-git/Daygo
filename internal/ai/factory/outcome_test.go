package factory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	daygoai "github.com/Jwz-git/Daygo/internal/ai"
)

// Even valid JSON text is not a usable generation when the wire protocol
// explicitly reports truncation, refusal or an unfinished tool turn.
func TestProtocolClientsRejectUnfinishedOutputs(t *testing.T) {
	const text = `"{\"name\":\"anonymous\"}"`
	const private = "fixture-private-detail"
	cases := []struct {
		name     string
		protocol daygoai.Protocol
		fields   string
	}{
		{"responses token limit", daygoai.ProtocolOpenAIResponses, `"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"message","content":[{"type":"output_text","text":` + text + `}]}]`},
		{"responses token limit without status", daygoai.ProtocolOpenAIResponses, `"incomplete_details":{"reason":"max_output_tokens"},"output":[]`},
		{"responses token limit with completed", daygoai.ProtocolOpenAIResponses, `"status":"completed","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"message","content":[{"type":"output_text","text":` + text + `}]}]`},
		{"responses unknown incomplete reason", daygoai.ProtocolOpenAIResponses, `"status":"incomplete","incomplete_details":{"reason":"` + private + `"},"output":[]`},
		{"responses error overrides incomplete reason", daygoai.ProtocolOpenAIResponses, `"status":"failed","error":{"code":"server_error","message":"` + private + `"},"incomplete_details":{"reason":"max_output_tokens"},"output":[]`},
		{"responses failed", daygoai.ProtocolOpenAIResponses, `"status":"failed","error":{"code":"server_error","message":"` + private + `"},"output":[{"type":"message","content":[{"type":"output_text","text":` + text + `}]}]`},
		{"responses queued", daygoai.ProtocolOpenAIResponses, `"status":"queued","output":[{"type":"message","content":[{"type":"output_text","text":` + text + `}]}]`},
		{"responses in progress", daygoai.ProtocolOpenAIResponses, `"status":"in_progress","output":[{"type":"message","content":[{"type":"output_text","text":` + text + `}]}]`},
		{"responses cancelled", daygoai.ProtocolOpenAIResponses, `"status":"cancelled","output":[{"type":"message","content":[{"type":"output_text","text":` + text + `}]}]`},
		{"responses unknown state", daygoai.ProtocolOpenAIResponses, `"status":"` + private + `","output":[{"type":"message","content":[{"type":"output_text","text":` + text + `}]}]`},
		{"responses message incomplete", daygoai.ProtocolOpenAIResponses, `"status":"completed","output":[{"type":"message","status":"incomplete","content":[{"type":"output_text","text":` + text + `}]}]`},
		{"responses refusal with text", daygoai.ProtocolOpenAIResponses, `"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":` + text + `},{"type":"refusal","refusal":"` + private + `"}]}]`},
		{"responses error with completed", daygoai.ProtocolOpenAIResponses, `"status":"completed","error":{"code":"server_error","message":"` + private + `"},"output":[{"type":"message","content":[{"type":"output_text","text":` + text + `}]}]`},
		{"responses whitespace", daygoai.ProtocolOpenAIResponses, `"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":" \n "}]}]`},
		{"anthropic token limit", daygoai.ProtocolAnthropicMessages, `"stop_reason":"max_tokens","content":[{"type":"text","text":` + text + `}]`},
		{"anthropic context limit", daygoai.ProtocolAnthropicMessages, `"stop_reason":"model_context_window_exceeded","content":[{"type":"text","text":` + text + `}]`},
		{"anthropic refusal", daygoai.ProtocolAnthropicMessages, `"stop_reason":"refusal","stop_details":{"reason":"` + private + `"},"content":[{"type":"text","text":` + text + `}]`},
		{"anthropic tool turn", daygoai.ProtocolAnthropicMessages, `"stop_reason":"tool_use","content":[{"type":"text","text":` + text + `}]`},
		{"anthropic paused turn", daygoai.ProtocolAnthropicMessages, `"stop_reason":"pause_turn","content":[{"type":"text","text":` + text + `}]`},
		{"anthropic unknown stop", daygoai.ProtocolAnthropicMessages, `"stop_reason":"` + private + `","content":[{"type":"text","text":` + text + `}]`},
		{"anthropic whitespace", daygoai.ProtocolAnthropicMessages, `"stop_reason":"end_turn","content":[{"type":"text","text":" \n "}]`},
		{"chat token limit", daygoai.ProtocolOpenAIChat, `"choices":[{"finish_reason":"length","message":{"content":` + text + `}}]`},
		{"chat filter", daygoai.ProtocolOpenAIChat, `"choices":[{"finish_reason":"content_filter","message":{"content":` + text + `}}]`},
		{"chat tool turn", daygoai.ProtocolOpenAIChat, `"choices":[{"finish_reason":"tool_calls","message":{"content":` + text + `}}]`},
		{"chat legacy function", daygoai.ProtocolOpenAIChat, `"choices":[{"finish_reason":"function_call","message":{"content":` + text + `}}]`},
		{"chat refusal with text", daygoai.ProtocolOpenAIChat, `"choices":[{"finish_reason":"stop","message":{"content":` + text + `,"refusal":"` + private + `"}}]`},
		{"chat unknown stop", daygoai.ProtocolOpenAIChat, `"choices":[{"finish_reason":"` + private + `","message":{"content":` + text + `}}]`},
		{"chat whitespace", daygoai.ProtocolOpenAIChat, `"choices":[{"finish_reason":"stop","message":{"content":" \n "}}]`},
	}
	tokenLimits := map[string]bool{
		"responses token limit": true, "responses token limit without status": true,
		"responses token limit with completed": true, "anthropic token limit": true,
		"anthropic context limit": true, "chat token limit": true,
	}
	for _, tc := range cases {
		for _, structured := range []bool{false, true} {
			name := tc.name + "/text"
			if structured {
				name = tc.name + "/schema"
			}
			t.Run(name, func(t *testing.T) {
				usage := `"prompt_tokens":12,"completion_tokens":4`
				if tc.protocol != daygoai.ProtocolOpenAIChat {
					usage = `"input_tokens":12,"output_tokens":4`
				}
				calls := 0
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls++
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"model":"` + private + `","usage":{` + usage + `},` + tc.fields + `}`))
				}))
				defer server.Close()
				provider, err := NewClient(server.Client(), Config{Protocol: tc.protocol, Endpoint: server.URL, Model: "fixture-model", Secret: "fixture-secret"})
				if err != nil {
					t.Fatal(err)
				}
				var observed daygoai.Attempt
				provider = daygoai.WithAttemptObserver(provider, "fixture-provider", tc.protocol, "fixture-model", daygoai.AttemptObserverFunc(func(_ context.Context, a daygoai.Attempt) { observed = a }))
				policy := daygoai.DefaultRetryPolicy()
				policy.Sleep = func(context.Context, time.Duration) error { return nil }
				provider = daygoai.WithRetry(provider, policy)
				request := daygoai.Request{Parts: []daygoai.Part{daygoai.TextPart("fixture")}}
				if structured {
					request.Output = &daygoai.OutputSchema{Name: "fixture", Strict: true, Schema: []byte(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"],"additionalProperties":false}`)}
				}
				result, err := provider.Generate(context.Background(), request)
				wantCalls := policy.MaxAttempts
				if tokenLimits[tc.name] {
					wantCalls = 1
				}
				if daygoai.ErrorKindOf(err) != daygoai.ErrorInvalidOutput || result.Model != "" || result.Text != "" || len(result.JSON) != 0 || calls != wantCalls {
					t.Fatalf("kind = %s, text = %q, JSON = %s, calls = %d", daygoai.ErrorKindOf(err), result.Text, result.JSON, calls)
				}
				if daygoai.Retryable(err) == tokenLimits[tc.name] {
					t.Fatalf("retryable=%v, token limit=%v", daygoai.Retryable(err), tokenLimits[tc.name])
				}
				if strings.Contains(err.Error(), private) {
					t.Fatal("provider detail appeared in the error")
				}
				if observed.Outcome != "failed" || observed.RequestedModel != "fixture-model" || observed.ActualModel != "" || observed.InputTokens == nil || *observed.InputTokens != 12 || observed.OutputTokens == nil || *observed.OutputTokens != 4 {
					t.Fatalf("attempt = %+v", observed)
				}
			})
		}
	}
}

func TestCardsProtocolOutputBudgetDefaults(t *testing.T) {
	for _, protocol := range []daygoai.Protocol{daygoai.ProtocolOpenAIChat, daygoai.ProtocolOpenAIResponses, daygoai.ProtocolAnthropicMessages} {
		t.Run(string(protocol), func(t *testing.T) {
			var payload map[string]json.RawMessage
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				w.Header().Set("Content-Type", "application/json")
				switch protocol {
				case daygoai.ProtocolOpenAIChat:
					_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"ok"}}]}`))
				case daygoai.ProtocolOpenAIResponses:
					_, _ = w.Write([]byte(`{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}]}`))
				case daygoai.ProtocolAnthropicMessages:
					_, _ = w.Write([]byte(`{"stop_reason":"end_turn","content":[{"type":"text","text":"ok"}]}`))
				}
			}))
			defer server.Close()
			provider, err := NewClient(server.Client(), Config{Protocol: protocol, Endpoint: server.URL, Model: "fixture-model"})
			if err != nil {
				t.Fatal(err)
			}
			result, err := provider.Generate(context.Background(), daygoai.Request{Purpose: daygoai.PurposeCards, Parts: []daygoai.Part{daygoai.TextPart("fixture")}})
			if err != nil || result.Text != "ok" {
				t.Fatalf("result=%+v, error=%v", result, err)
			}
			if protocol == daygoai.ProtocolAnthropicMessages {
				if string(payload["max_tokens"]) != "8192" {
					t.Fatalf("required max_tokens=%s, want preserved 8192 allowance", payload["max_tokens"])
				}
			} else {
				for _, key := range []string{"max_tokens", "max_completion_tokens", "max_output_tokens"} {
					if _, present := payload[key]; present {
						t.Fatalf("card request unexpectedly overrides provider default with %s", key)
					}
				}
			}
		})
	}
}

func TestProtocolClientsPreserveCompletionCompatibility(t *testing.T) {
	for _, protocol := range []daygoai.Protocol{daygoai.ProtocolOpenAIChat, daygoai.ProtocolOpenAIResponses, daygoai.ProtocolAnthropicMessages} {
		for _, status := range []string{"present", "omitted"} {
			for _, usage := range []string{"omitted", "null", "zero", "input only", "output only"} {
				t.Run(string(protocol)+"/"+status+"/"+usage, func(t *testing.T) {
					var body string
					var counts string
					switch protocol {
					case daygoai.ProtocolOpenAIChat:
						finish := ""
						if status == "present" {
							finish = `"finish_reason":"stop",`
						}
						body = `"choices":[{` + finish + `"message":{"content":"ok"}}]`
						counts = `{"prompt_tokens":0,"completion_tokens":0}`
					case daygoai.ProtocolOpenAIResponses:
						body = `"output":[{"type":"reasoning"},{"type":"message","content":[{"type":"output_text","text":"o"},{"type":"output_text","text":"k"}]}]`
						if status == "present" {
							body = `"status":"completed","error":null,` + body
						}
						counts = `{"input_tokens":0,"output_tokens":0}`
					case daygoai.ProtocolAnthropicMessages:
						body = `"content":[{"type":"text","text":"o"},{"type":"text","text":"k"}]`
						if status == "present" {
							body = `"stop_reason":"end_turn",` + body
						}
						counts = `{"input_tokens":0,"output_tokens":0}`
					}
					if usage == "null" {
						body += `,"usage":null`
					} else if usage == "zero" {
						body += `,"usage":` + counts
					} else if usage == "input only" {
						if protocol == daygoai.ProtocolOpenAIChat {
							body += `,"usage":{"prompt_tokens":0}`
						} else {
							body += `,"usage":{"input_tokens":0}`
						}
					} else if usage == "output only" {
						if protocol == daygoai.ProtocolOpenAIChat {
							body += `,"usage":{"completion_tokens":0}`
						} else {
							body += `,"usage":{"output_tokens":0}`
						}
					}
					server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write([]byte(`{"model":"fixture-model",` + body + `}`))
					}))
					defer server.Close()
					provider, err := NewClient(server.Client(), Config{Protocol: protocol, Endpoint: server.URL, Model: "fixture-model"})
					if err != nil {
						t.Fatal(err)
					}
					result, err := provider.Generate(context.Background(), daygoai.Request{Parts: []daygoai.Part{daygoai.TextPart("fixture")}})
					if err != nil || result.Text != "ok" {
						t.Fatalf("text = %q, error = %v", result.Text, err)
					}
					wantInput := usage == "zero" || usage == "input only"
					wantOutput := usage == "zero" || usage == "output only"
					if (result.Usage.InputTokens != nil) != wantInput || (result.Usage.OutputTokens != nil) != wantOutput {
						t.Fatal("usage presence changed")
					}
					if (wantInput && *result.Usage.InputTokens != 0) || (wantOutput && *result.Usage.OutputTokens != 0) {
						t.Fatal("explicit zero usage was lost")
					}
				})
			}
		}
	}
}
