package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	daygoai "github.com/Jwz-git/Daygo/internal/ai"
)

func TestGenerateMapsMultimodalStructuredRequest(t *testing.T) {
	var gotPath string
	var gotAPIKey string
	var gotBody map[string]any

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("X-Api-Key")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"msg_fixture","type":"message","role":"assistant","model":"fixture-model",
			"content":[{"type":"text","text":"{\"name\":\"anonymous\"}"}],
			"stop_reason":"end_turn","stop_sequence":null,
			"usage":{"input_tokens":12,"output_tokens":4,"cache_creation_input_tokens":2,"cache_read_input_tokens":3}
		}`))
	}))
	defer server.Close()

	client, err := NewClient(server.Client(), server.URL, "requested-model", "fixture-secret")
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
	if gotPath != "/v1/messages" || gotAPIKey != "fixture-secret" {
		t.Fatalf("path=%q api key=%q", gotPath, gotAPIKey)
	}

	messages := gotBody["messages"].([]any)
	content := messages[0].(map[string]any)["content"].([]any)
	imageSource := content[1].(map[string]any)["source"].(map[string]any)

	if imageSource["media_type"] != "image/png" || imageSource["data"] != "AQID" {
		t.Fatalf("image source = %#v", imageSource)
	}
	outputConfig := gotBody["output_config"].(map[string]any)
	format := outputConfig["format"].(map[string]any)
	if format["type"] != "json_schema" {
		t.Fatalf("output_config = %#v", outputConfig)
	}
	if result.Model != "fixture-model" || string(result.JSON) != `{"name":"anonymous"}` {
		t.Fatalf("result = %#v", result)
	}
	if result.Usage.InputTokens == nil || *result.Usage.InputTokens != 12 ||
		result.Usage.OutputTokens == nil || *result.Usage.OutputTokens != 4 ||
		result.Usage.CacheReadTokens == nil || *result.Usage.CacheReadTokens != 3 ||
		result.Usage.CacheWriteTokens == nil || *result.Usage.CacheWriteTokens != 2 {
		t.Fatalf("usage = %#v", result.Usage)
	}
}

func TestGenerateClassifiesAndRedactsAPIErrorWithoutRetry(t *testing.T) {
	const sensitive = "fixture-sensitive-body"
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"api_error","message":"` + sensitive + `"}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.Client(), server.URL, "model", "secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Generate(context.Background(), daygoai.Request{Parts: []daygoai.Part{daygoai.TextPart("test")}})
	if daygoai.ErrorKindOf(err) != daygoai.ErrorUnavailable || daygoai.HTTPStatusOf(err) != 500 {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), sensitive) {
		t.Fatal("error exposed response body")
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", calls.Load())
	}
}

func TestGenerateCancelsInFlightRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	}))
	defer server.Close()

	client, err := NewClient(server.Client(), server.URL, "model", "secret")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-started
		cancel()
	}()

	_, err = client.Generate(ctx, daygoai.Request{Parts: []daygoai.Part{daygoai.TextPart("test")}})
	close(release)
	if daygoai.ErrorKindOf(err) != daygoai.ErrorCanceled {
		t.Fatalf("error = %v", err)
	}
}
