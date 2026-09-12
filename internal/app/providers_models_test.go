package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A saved provider's models come from the keychain secret; the request itself
// goes to the endpoint the row stores.
func TestListProviderModelsForSavedProvider(t *testing.T) {
	backend, _, fake := backendWithStoreAndSecrets(t)

	id, err := backend.AddProvider(validProviderInput())
	if err != nil {
		t.Fatalf("AddProvider: %v", err)
	}
	if err := backend.SetProviderSecret(id, "sk-models-key"); err != nil {
		t.Fatalf("SetProviderSecret: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("path = %q, want /models", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-models-key" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "m1"}, {"id": "m0"}}})
	}))
	t.Cleanup(server.Close)

	// Repoint the provider at the test server.
	input := validProviderInput()
	input.Endpoint = server.URL
	if err := backend.UpdateProvider(id, input); err != nil {
		t.Fatalf("UpdateProvider: %v", err)
	}

	result, err := backend.ListProviderModels(ProviderModelsRequestDTO{ProviderID: id})
	if err != nil {
		t.Fatalf("ListProviderModels: %v", err)
	}
	if !result.OK || len(result.Models) != 2 || result.Models[0] != "m0" {
		t.Fatalf("result = %+v", result)
	}
	// The keychain still holds the key; nothing was consumed.
	if _, err := fake.Get(context.Background(), id); err != nil {
		t.Fatalf("keychain key vanished: %v", err)
	}
}

// The draft path carries the secret for this call only and never stores it.
func TestListProviderModelsForDraft(t *testing.T) {
	backend, _, _ := backendWithStoreAndSecrets(t)
	canary := "sk-draft-canary"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+canary {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"data": [{"id": "draft-model"}]}`))
	}))
	t.Cleanup(server.Close)

	result, err := backend.ListProviderModels(ProviderModelsRequestDTO{
		Protocol: "openai",
		Endpoint: server.URL,
		Secret:   canary,
	})
	if err != nil {
		t.Fatalf("ListProviderModels: %v", err)
	}
	if !result.OK || len(result.Models) != 1 || result.Models[0] != "draft-model" {
		t.Fatalf("result = %+v", result)
	}

	// Nothing was persisted.
	list, err := backend.ListProviders()
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("draft listing created providers: %+v", list)
	}
}

// A provider without a stored key is rejected before any request goes out.
func TestListProviderModelsSavedWithoutKey(t *testing.T) {
	backend, _, _ := backendWithStoreAndSecrets(t)

	id, err := backend.AddProvider(validProviderInput())
	if err != nil {
		t.Fatalf("AddProvider: %v", err)
	}

	_, err = backend.ListProviderModels(ProviderModelsRequestDTO{ProviderID: id})
	if err == nil {
		t.Fatal("listing without a stored key succeeded")
	}
}

// A failing endpoint reports a result row, not an error, mirroring the probe.
func TestListProviderModelsFailureIsAResult(t *testing.T) {
	backend, _, _ := backendWithStoreAndSecrets(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	result, err := backend.ListProviderModels(ProviderModelsRequestDTO{
		Protocol: "openai",
		Endpoint: server.URL,
		Secret:   "k",
	})
	if err != nil {
		t.Fatalf("ListProviderModels returned an error: %v", err)
	}
	if result.OK || result.ErrorCode != "authentication" {
		t.Fatalf("result = %+v, want ok=false authentication", result)
	}
}
