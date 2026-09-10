package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	daygoai "github.com/Jwz-git/Daygo/internal/ai"
)

func TestResponsesGenerateMapsMultimodalStructuredRequest(t *testing.T) {
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
			"output":[{"type":"message","role":"assistant","content":[
				{"type":"output_text","text":"{\"name\":\"anonymous\"}","annotations":[]}
			]}],
			"usage":{"input_tokens":12,"output_tokens":4,"input_tokens_details":{"cached_tokens":3,"cache_write_tokens":2}}
		}`))
	}))
	defer server.Close()

	client, err := NewResponsesClient(server.Client(), server.URL+"/v1", "requested-model", "fixture-secret")
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
	if gotPath != "/v1/responses" || gotAuthorization != "Bearer fixture-secret" {
		t.Fatalf("path=%q authorization=%q", gotPath, gotAuthorization)
	}
	input := gotBody["input"].([]any)
	content := input[0].(map[string]any)["content"].([]any)
	if content[0].(map[string]any)["type"] != "input_text" {
		t.Fatalf("text content = %#v", content[0])
	}
	imageURL := content[1].(map[string]any)["image_url"].(string)
	if !strings.HasPrefix(imageURL, "data:image/png;base64,") {
		t.Fatalf("image URL = %q", imageURL)
	}
	format := gotBody["text"].(map[string]any)["format"].(map[string]any)
	if format["type"] != "json_schema" || format["name"] != "item" {
		t.Fatalf("text format = %#v", format)
	}
	if gotBody["max_output_tokens"] != float64(100) {
		t.Fatalf("max_output_tokens = %#v", gotBody["max_output_tokens"])
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

func TestResponsesGenerateClassifiesAndRedactsErrorBody(t *testing.T) {
	const sensitive = "fixture-sensitive-body"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(sensitive))
	}))
	defer server.Close()
	client, err := NewResponsesClient(server.Client(), server.URL, "model", "secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Generate(context.Background(), daygoai.Request{
		Parts:  []daygoai.Part{daygoai.TextPart("test")},
		Output: &daygoai.OutputSchema{Name: "item", Strict: true, Schema: []byte(`{"type":"object"}`)},
	})
	if daygoai.ErrorKindOf(err) != daygoai.ErrorUnsupportedFeature || daygoai.HTTPStatusOf(err) != 404 {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), sensitive) {
		t.Fatal("error exposed response body")
	}
}
