package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	daygoai "github.com/Jwz-git/Daygo/internal/ai"
)

func TestGenerateMapsMultimodalStructuredRequest(t *testing.T) {
	var gotPath string
	var gotAuthorization string
	var gotBody map[string]any

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuthorization = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"model":"fixture-model",
			"choices":[{"message":{"content":"{\"name\":\"anonymous\"}"}}],
			"usage":{"prompt_tokens":12,"completion_tokens":4,"prompt_tokens_details":{"cached_tokens":3}}
		}`))
	}))
	defer server.Close()

	client, err := NewClient(server.Client(), server.URL+"/v1", "requested-model", "fixture-secret")
	if err != nil {
		t.Fatal(err)
	}
	image, err := daygoai.ImagePart(daygoai.MediaPNG, []byte{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	output := &daygoai.OutputSchema{Name: "item", Strict: true, Schema: []byte(`{
		"type":"object","properties":{"name":{"type":"string"}},"required":["name"],"additionalProperties":false
	}`)}
	result, err := client.Generate(context.Background(), daygoai.Request{
		Parts: []daygoai.Part{daygoai.TextPart("describe"), image}, Output: output, MaxOutputTokens: 100,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if gotPath != "/v1/chat/completions" || gotAuthorization != "Bearer fixture-secret" {
		t.Fatalf("path=%q authorization=%q", gotPath, gotAuthorization)
	}
	if gotBody["response_format"].(map[string]any)["type"] != "json_schema" {
		t.Fatalf("response_format = %#v", gotBody["response_format"])
	}
	if gotBody["max_completion_tokens"] != float64(100) || gotBody["max_tokens"] != nil {
		t.Fatalf("structured request token limit = %#v", gotBody)
	}

	messages := gotBody["messages"].([]any)
	content := messages[0].(map[string]any)["content"].([]any)
	imageURL := content[1].(map[string]any)["image_url"].(map[string]any)["url"].(string)

	if !strings.HasPrefix(imageURL, "data:image/png;base64,") {
		t.Fatalf("image URL = %q", imageURL)
	}
	if result.Model != "fixture-model" || string(result.JSON) != `{"name":"anonymous"}` {
		t.Fatalf("result = %#v", result)
	}
	if result.Usage.InputTokens == nil || *result.Usage.InputTokens != 12 || result.Usage.CacheReadTokens == nil || *result.Usage.CacheReadTokens != 3 {
		t.Fatalf("usage = %#v", result.Usage)
	}
}

func TestGenerateRejectsUnsupportedStructuredOutput(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"response_format is not supported","type":"invalid_request_error"}}`))
	}))
	defer server.Close()
	client, err := NewClient(server.Client(), server.URL, "model", "secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Generate(context.Background(), daygoai.Request{
		Parts:  []daygoai.Part{daygoai.TextPart("test")},
		Output: &daygoai.OutputSchema{Name: "item", Strict: true, Schema: []byte(`{"type":"object"}`)},
	})
	if daygoai.ErrorKindOf(err) != daygoai.ErrorUnsupportedFeature {
		t.Fatalf("error = %v", err)
	}
}

// Model aliases must use the modern parameter too; guessing from the model
// name would leave gateways exposing reasoning models under an alias broken.
func TestGenerateUsesMaxCompletionTokens(t *testing.T) {
	for _, tokens := range []int{0, 2048} {
		t.Run(fmt.Sprint(tokens), func(t *testing.T) {
			calls := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode request: %v", err)
				}
				if _, legacy := body["max_tokens"]; legacy {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"error":{"param":"max_tokens","code":"unsupported_parameter"}}`))
					return
				}
				value, present := body["max_completion_tokens"]
				if (tokens > 0 && value != float64(tokens)) || (tokens == 0 && present) {
					t.Errorf("max_completion_tokens = %v, present=%v, tokens=%d", value, present, tokens)
				}
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
			}))
			defer server.Close()
			client, err := NewClient(server.Client(), server.URL+"/v1", "gateway-alias", "fixture-secret")
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.Generate(context.Background(), daygoai.Request{Parts: []daygoai.Part{daygoai.TextPart("fixture")}, MaxOutputTokens: tokens})
			if err != nil || result.Text != "ok" || calls != 1 {
				t.Fatalf("result = %+v, error = %v, calls = %d", result, err, calls)
			}
		})
	}
}

func TestGenerateDoesNotRetryRejectedTokenParameter(t *testing.T) {
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if body["max_completion_tokens"] != float64(100) || body["response_format"] == nil {
			t.Error("request lost its token limit or structured output")
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"param":"max_completion_tokens","code":"unsupported_parameter","message":"fixture-private-input"}}`))
	}))
	defer server.Close()
	client, err := NewClient(server.Client(), server.URL, "fixture-model", "fixture-secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Generate(context.Background(), daygoai.Request{
		Parts: []daygoai.Part{daygoai.TextPart("fixture")}, MaxOutputTokens: 100,
		Output: &daygoai.OutputSchema{Name: "item", Strict: true, Schema: []byte(`{"type":"object"}`)},
	})
	if daygoai.ErrorKindOf(err) != daygoai.ErrorInvalidRequest || calls != 1 || daygoai.Retryable(err) {
		t.Fatalf("error = %v, calls = %d", err, calls)
	}
	if strings.Contains(err.Error(), "fixture-private-input") {
		t.Fatal("error exposed provider response text")
	}
}

func TestGenerateAcceptsPastedEndpoints(t *testing.T) {
	for _, tc := range []struct{ endpoint, path string }{
		{"", "/chat/completions"},
		{"/v1/", "/v1/chat/completions"},
		{"/v1/chat/completions", "/v1/chat/completions"},
		{"/v1/chat/completions///?ignored=1#fragment", "/v1/chat/completions"},
		{"/proxy/openai/v1/chat/completions/", "/proxy/openai/v1/chat/completions"},
		{"/gateway/custom/", "/gateway/custom/chat/completions"},
		{"/proxy%2Ftenant/v1/chat/completions/", "/proxy%2Ftenant/v1/chat/completions"},
	} {
		t.Run(tc.endpoint, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.EscapedPath() != tc.path || r.URL.RawQuery != "" {
					t.Errorf("request URL = %s, want path %s without query", r.URL, tc.path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
			}))
			defer server.Close()
			client, err := NewClient(server.Client(), server.URL+tc.endpoint, "fixture-model", "fixture-secret")
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.Generate(context.Background(), daygoai.Request{Parts: []daygoai.Part{daygoai.TextPart("fixture")}})
			if err != nil || result.Text != "ok" {
				t.Fatalf("result = %+v, error = %v", result, err)
			}
		})
	}
}

func TestGenerateClassifiesBadRequestWithoutStructuredOutputMarkers(t *testing.T) {
	// A 400 caused by anything else (bad parameter, unknown model) must not be
	// reported as a missing structured-output capability.
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Unsupported parameter: 'max_tokens'","type":"invalid_request_error","code":"unsupported_parameter"}}`))
	}))
	defer server.Close()
	client, err := NewClient(server.Client(), server.URL, "model", "secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Generate(context.Background(), daygoai.Request{
		Parts:  []daygoai.Part{daygoai.TextPart("test")},
		Output: &daygoai.OutputSchema{Name: "item", Strict: true, Schema: []byte(`{"type":"object"}`)},
	})
	if daygoai.ErrorKindOf(err) != daygoai.ErrorInvalidRequest {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(err.Error(), "unsupported_parameter") {
		t.Fatalf("error does not carry the provider code: %v", err)
	}
}

func TestGenerateClassifiesModelNotFound(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"The model 'nope' does not exist","type":"invalid_request_error","code":"model_not_found"}}`))
	}))
	defer server.Close()
	client, err := NewClient(server.Client(), server.URL, "model", "secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Generate(context.Background(), daygoai.Request{
		Parts:  []daygoai.Part{daygoai.TextPart("test")},
		Output: &daygoai.OutputSchema{Name: "item", Strict: true, Schema: []byte(`{"type":"object"}`)},
	})
	if daygoai.ErrorKindOf(err) != daygoai.ErrorInvalidRequest {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(err.Error(), "model_not_found") {
		t.Fatalf("error does not carry the provider code: %v", err)
	}
}

func TestGenerateErrorDetailStaysPrintableAndShort(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"x","code":"` + strings.Repeat("a", 200) + `"}}`))
	}))
	defer server.Close()
	client, err := NewClient(server.Client(), server.URL, "model", "secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Generate(context.Background(), daygoai.Request{Parts: []daygoai.Part{daygoai.TextPart("test")}})
	if daygoai.ErrorKindOf(err) != daygoai.ErrorInvalidRequest {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), strings.Repeat("a", 100)) {
		t.Fatal("error carried an over-long provider code")
	}
}

func TestGenerateClassifiesAndRedactsErrorBody(t *testing.T) {
	const sensitive = "fixture-sensitive-body"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "45")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(sensitive))
	}))
	defer server.Close()

	client, err := NewClient(server.Client(), server.URL, "model", "secret")
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Generate(context.Background(), daygoai.Request{Parts: []daygoai.Part{daygoai.TextPart("test")}})
	if daygoai.ErrorKindOf(err) != daygoai.ErrorRateLimited || daygoai.HTTPStatusOf(err) != 429 {
		t.Fatalf("error = %v", err)
	}
	if daygoai.RetryAfterOf(err) != 45*time.Second {
		t.Fatalf("retry after = %s", daygoai.RetryAfterOf(err))
	}
	if strings.Contains(err.Error(), sensitive) {
		t.Fatal("error exposed response body")
	}
}

func TestGenerateClassifiesRateLimitInResponseBody(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Rate limit exceeded: Please try again in 20s.","type":"requests","code":"rate_limit_exceeded"}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.Client(), server.URL, "model", "secret")
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Generate(context.Background(), daygoai.Request{Parts: []daygoai.Part{daygoai.TextPart("test")}})
	if daygoai.ErrorKindOf(err) != daygoai.ErrorRateLimited {
		t.Fatalf("kind = %v, want rate_limited", daygoai.ErrorKindOf(err))
	}
	if daygoai.RetryAfterOf(err) != 20*time.Second {
		t.Fatalf("retry after = %s, want 20s", daygoai.RetryAfterOf(err))
	}
}
