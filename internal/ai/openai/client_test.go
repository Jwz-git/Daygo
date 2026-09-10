package openai

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
		w.WriteHeader(http.StatusBadRequest)
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
