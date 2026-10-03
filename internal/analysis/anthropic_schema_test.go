package analysis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/ai/factory"
)

// Exercise the production transcription schema, not a permissive substitute.
func TestAnthropicProductionTranscriptionSchema(t *testing.T) {
	const response = `{"observations":[{"from_frame":0,"to_frame":0,"observation":"Anonymous fixture","apps":[]}]}`
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			OutputConfig struct {
				Format struct {
					Schema map[string]any `json:"schema"`
				} `json:"format"`
			} `json:"output_config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		properties := body.OutputConfig.Format.Schema["properties"].(map[string]any)
		items := properties["observations"].(map[string]any)["items"].(map[string]any)
		fields := items["properties"].(map[string]any)
		for _, key := range []string{"from_frame", "to_frame"} {
			if _, present := fields[key].(map[string]any)["minimum"]; present {
				t.Errorf("production field %s sends unsupported minimum", key)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "fixture-model", "content": []map[string]string{{"type": "text", "text": response}}})
	}))
	defer server.Close()
	client, err := factory.NewClient(server.Client(), factory.Config{
		Protocol: ai.ProtocolAnthropicMessages, Endpoint: server.URL + "/v1", Model: "fixture-model", Secret: "fixture-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Generate(context.Background(), ai.Request{
		Purpose: ai.PurposeTranscribe, Parts: []ai.Part{ai.TextPart("Anonymous fixture")}, Output: &transcribeOutput,
	})
	if err != nil || string(result.JSON) != response {
		t.Fatalf("JSON = %s, error = %v", result.JSON, err)
	}
}
