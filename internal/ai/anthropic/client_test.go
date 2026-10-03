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

func TestGenerateAcceptsVersionedAndPastedEndpoints(t *testing.T) {
	cases := []struct{ endpoint, path string }{
		{"", "/v1/messages"},
		{"/v1", "/v1/messages"},
		{"/v1/", "/v1/messages"},
		{"/v1/messages", "/v1/messages"},
		{"/v1/messages/", "/v1/messages"},
		{"/v1/models", "/v1/messages"},
		{"/proxy/anthropic", "/proxy/anthropic/v1/messages"},
		{"/proxy/anthropic/v1", "/proxy/anthropic/v1/messages"},
	}
	for _, tc := range cases {
		t.Run(tc.endpoint, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path || r.URL.RawQuery != "" {
					t.Errorf("request URL = %s, want path %s without query", r.URL, tc.path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"model":"fixture-model","content":[{"type":"text","text":"ok"}]}`))
			}))
			defer server.Close()
			client, err := NewClient(server.Client(), server.URL+tc.endpoint+"?ignored=1#fragment", "model", "fixture-secret")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.Generate(context.Background(), daygoai.Request{Parts: []daygoai.Part{daygoai.TextPart("fixture")}}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGenerateIgnoresSDKEnvironmentCredentials(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "fixture-unrelated-token")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "fixture-configured-key" || r.Header.Get("Authorization") != "" {
			t.Error("request did not use only the configured credential")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"fixture-model","content":[{"type":"text","text":"ok"}]}`))
	}))
	defer server.Close()
	client, err := NewClient(server.Client(), server.URL, "model", "fixture-configured-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Generate(context.Background(), daygoai.Request{Parts: []daygoai.Part{daygoai.TextPart("fixture")}}); err != nil {
		t.Fatal(err)
	}
}

// The production frame-transcription schema uses minimum: 0. Anthropic
// rejects that keyword, but a negative index must still fail local validation.
func TestGenerateAdaptsNumericConstraintsAndValidatesOriginalSchema(t *testing.T) {
	output := &daygoai.OutputSchema{Name: "frames", Strict: true, Schema: []byte(`{
		"type":"object","properties":{"observations":{"type":"array","items":{
			"type":"object","properties":{"from_frame":{"type":"integer","minimum":0}},
			"required":["from_frame"],"additionalProperties":false
		}}},"required":["observations"],"additionalProperties":false
	}`)}
	original := string(output.Schema)
	for _, tc := range []struct {
		name string
		text string
		kind daygoai.ErrorKind
	}{
		{"valid frame", `{"observations":[{"from_frame":0}]}`, ""},
		{"negative frame", `{"observations":[{"from_frame":-1}]}`, daygoai.ErrorInvalidOutput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					OutputConfig struct {
						Format struct {
							Type   string         `json:"type"`
							Schema map[string]any `json:"schema"`
						} `json:"format"`
					} `json:"output_config"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if body.OutputConfig.Format.Type != "json_schema" {
					t.Error("request lost native structured output")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				properties := body.OutputConfig.Format.Schema["properties"].(map[string]any)
				items := properties["observations"].(map[string]any)["items"].(map[string]any)
				frame := items["properties"].(map[string]any)["from_frame"].(map[string]any)
				if _, present := frame["minimum"]; present {
					t.Error("unsupported minimum constraint sent to Anthropic")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if description, _ := frame["description"].(string); !strings.Contains(description, `"minimum":0`) {
					t.Errorf("description lost the numeric constraint: %q", description)
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"model": "model", "content": []map[string]string{{"type": "text", "text": tc.text}}})
			}))
			defer server.Close()
			client, err := NewClient(server.Client(), server.URL, "model", "fixture-secret")
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.Generate(context.Background(), daygoai.Request{Parts: []daygoai.Part{daygoai.TextPart("fixture")}, Output: output})
			if daygoai.ErrorKindOf(err) != tc.kind {
				t.Fatalf("error = %v, want %s", err, tc.kind)
			}
			if tc.kind == "" && string(result.JSON) != tc.text {
				t.Fatalf("JSON = %s", result.JSON)
			}
			if string(output.Schema) != original {
				t.Fatal("original validation schema was mutated")
			}
		})
	}
}

func TestGenerateClassifiesStructuredOutputRejections(t *testing.T) {
	for _, tc := range []struct {
		name, message string
		status        int
		structured    bool
		want          daygoai.ErrorKind
	}{
		{"unsupported output", "output_config.format is not supported", 400, true, daygoai.ErrorUnsupportedFeature},
		{"gateway output format", "unknown output_format parameter", 422, true, daygoai.ErrorUnsupportedFeature},
		{"bad model", "model not found", 404, true, daygoai.ErrorInvalidRequest},
		{"unstructured request", "output_config.format is not supported", 400, false, daygoai.ErrorInvalidRequest},
		{"authentication", "output_config.format fixture-sensitive", 401, true, daygoai.ErrorAuthentication},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(map[string]any{"type": "error", "error": map[string]string{"type": "invalid_request_error", "message": tc.message + " fixture-sensitive"}})
			}))
			defer server.Close()
			client, err := NewClient(server.Client(), server.URL, "model", "fixture-secret")
			if err != nil {
				t.Fatal(err)
			}
			request := daygoai.Request{Parts: []daygoai.Part{daygoai.TextPart("fixture")}}
			if tc.structured {
				request.Output = &daygoai.OutputSchema{Name: "fixture", Strict: true, Schema: []byte(`{"type":"object","properties":{},"additionalProperties":false}`)}
			}
			_, err = client.Generate(context.Background(), request)
			if daygoai.ErrorKindOf(err) != tc.want || daygoai.HTTPStatusOf(err) != tc.status {
				t.Fatalf("error = %v, want %s / %d", err, tc.want, tc.status)
			}
			if strings.Contains(err.Error(), "fixture-sensitive") || calls.Load() != 1 {
				t.Fatal("error leaked content or silently retried without schema")
			}
		})
	}
}
