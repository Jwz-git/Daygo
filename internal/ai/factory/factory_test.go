package factory

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	daygoai "github.com/Jwz-git/Daygo/internal/ai"
)

// TestNewClientRoutesEachProtocol spins one TLS fixture per protocol and
// verifies the factory dispatches to the right wire path.
func TestNewClientRoutesEachProtocol(t *testing.T) {
	cases := []struct {
		protocol daygoai.Protocol
		endpoint string // suffix; the Anthropic SDK already prefixes /v1
		path     string
	}{
		{daygoai.ProtocolOpenAIChat, "/v1", "/v1/chat/completions"},
		{daygoai.ProtocolOpenAIResponses, "/v1", "/v1/responses"},
		{daygoai.ProtocolAnthropicMessages, "", "/v1/messages"},
	}
	for _, testCase := range cases {
		t.Run(string(testCase.protocol), func(t *testing.T) {
			var gotPath string
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				switch testCase.protocol {
				case daygoai.ProtocolAnthropicMessages:
					_, _ = w.Write([]byte(`{
						"model":"fixture-model",
						"content":[{"type":"text","text":"ok"}],
						"usage":{"input_tokens":1,"output_tokens":1}
					}`))
				case daygoai.ProtocolOpenAIResponses:
					_, _ = w.Write([]byte(`{
						"model":"fixture-model",
						"output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],
						"usage":{"input_tokens":1,"output_tokens":1}
					}`))
				default:
					_, _ = w.Write([]byte(`{
						"model":"fixture-model",
						"choices":[{"message":{"content":"ok"}}],
						"usage":{"prompt_tokens":1,"completion_tokens":1}
					}`))
				}
			}))
			defer server.Close()

			provider, err := NewClient(server.Client(), Config{
				Protocol: testCase.protocol,
				Endpoint: server.URL + testCase.endpoint,
				Model:    "requested-model",
				Secret:   "fixture-secret",
			})
			if err != nil {
				t.Fatal(err)
			}
			result, err := provider.Generate(context.Background(), daygoai.Request{
				Parts: []daygoai.Part{daygoai.TextPart("describe")},
			})
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if gotPath != testCase.path || result.Text != "ok" || result.Model != "fixture-model" {
				t.Fatalf("path=%q result=%#v", gotPath, result)
			}
		})
	}
}

func TestNewClientRejectsUnknownProtocol(t *testing.T) {
	_, err := NewClient(nil, Config{Protocol: "carrier-pigeon", Endpoint: "https://example.test", Model: "m"})
	if daygoai.ErrorKindOf(err) != daygoai.ErrorInvalidRequest {
		t.Fatalf("error = %v", err)
	}
}
