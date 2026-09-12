package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func modelsServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func TestListModelsOpenAIShape(t *testing.T) {
	var gotAuth, gotPath string
	server := modelsServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"id": "model-b"},
				{"id": "model-a"},
				{"id": ""}, // blank ids are dropped
			},
		})
	})

	models, err := ListModels(context.Background(), ProtocolOpenAIChat, server.URL, "sk-test")
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotPath != "/models" {
		t.Fatalf("path = %q, want /models", gotPath)
	}
	if len(models) != 2 || models[0] != "model-a" || models[1] != "model-b" {
		t.Fatalf("models = %v, want sorted without blanks", models)
	}
}

func TestListModelsAnthropicShape(t *testing.T) {
	var gotKey, gotVersion, gotPath string
	server := modelsServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"id": "claude-x"}, {"id": "claude-y"}]}`))
	})

	models, err := ListModels(context.Background(), ProtocolAnthropicMessages, server.URL, "sk-ant-test")
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if gotKey != "sk-ant-test" || gotVersion != "2023-06-01" {
		t.Fatalf("headers = %q / %q", gotKey, gotVersion)
	}
	if gotPath != "/v1/models" {
		t.Fatalf("path = %q, want /v1/models", gotPath)
	}
	if len(models) != 2 {
		t.Fatalf("models = %v", models)
	}
}

// openai_responses shares the openai /models listing.
func TestListModelsResponsesUsesModelsPath(t *testing.T) {
	var gotPath string
	server := modelsServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"data": []}`))
	})

	if _, err := ListModels(context.Background(), ProtocolOpenAIResponses, server.URL, "k"); err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if gotPath != "/models" {
		t.Fatalf("path = %q, want /models", gotPath)
	}
}

func TestListModelsClassifiesStatuses(t *testing.T) {
	cases := []struct {
		status int
		want   ErrorKind
	}{
		{http.StatusUnauthorized, ErrorAuthentication},
		{http.StatusForbidden, ErrorAuthentication},
		{http.StatusNotFound, ErrorInvalidRequest},
		{http.StatusTooManyRequests, ErrorRateLimited},
		{http.StatusInternalServerError, ErrorUnavailable},
	}
	for _, tc := range cases {
		server := modelsServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
		})
		_, err := ListModels(context.Background(), ProtocolOpenAIChat, server.URL, "k")
		if ErrorKindOf(err) != tc.want {
			t.Errorf("status %d: kind = %v, want %v", tc.status, ErrorKindOf(err), tc.want)
		}
		if HTTPStatusOf(err) != tc.status {
			t.Errorf("status %d: HTTPStatus = %d", tc.status, HTTPStatusOf(err))
		}
	}
}

// A provider without a models endpoint reports a classified error — the UI
// falls back to manual entry; it must not see a crash or an empty success.
func TestListModelsUnreachableEndpoint(t *testing.T) {
	server := modelsServer(t, func(w http.ResponseWriter, r *http.Request) {})
	url := server.URL
	server.Close()

	_, err := ListModels(context.Background(), ProtocolOpenAIChat, url, "k")
	if ErrorKindOf(err) != ErrorUnavailable {
		t.Fatalf("kind = %v, want unavailable", ErrorKindOf(err))
	}
}

func TestListModelsInvalidJSON(t *testing.T) {
	server := modelsServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	})

	_, err := ListModels(context.Background(), ProtocolOpenAIChat, server.URL, "k")
	if ErrorKindOf(err) != ErrorInvalidOutput {
		t.Fatalf("kind = %v, want invalid_output", ErrorKindOf(err))
	}
}

func TestListModelsCapsAtHundred(t *testing.T) {
	server := modelsServer(t, func(w http.ResponseWriter, r *http.Request) {
		data := make([]map[string]string, 150)
		for i := range data {
			data[i] = map[string]string{"id": "m"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	})

	models, err := ListModels(context.Background(), ProtocolOpenAIChat, server.URL, "k")
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != maxModels {
		t.Fatalf("len = %d, want %d", len(models), maxModels)
	}
}

func TestListModelsRejectsUnknownProtocol(t *testing.T) {
	if _, err := ListModels(context.Background(), Protocol("smtp"), "https://e.example.com", "k"); ErrorKindOf(err) != ErrorInvalidRequest {
		t.Fatalf("kind = %v, want invalid_request", ErrorKindOf(err))
	}
}
