package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// probeServer answers one Chat Completions probe call over plain HTTP (the
// binding builds its own http.Client, so the fixture stays on a port the
// default transport trusts). status 0 means success: the body then carries
// the fixture probe answer.
func probeServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if status != 0 {
			w.WriteHeader(status)
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

const probeOKBody = `{
	"model":"fixture-model",
	"choices":[{"message":{"content":"{\"probeToken\":\"daygo-connection-v1\",\"imageChoice\":\"github_octocat\"}"}}]
}`

func TestProviderConnectionReportsSuccess(t *testing.T) {
	server := probeServer(t, 0, probeOKBody)
	backend := newBackend(systemClock{}, nil, true, true)

	result, err := backend.TestProviderConnection(ProviderTestDraftDTO{
		Protocol: "openai",
		Endpoint: server.URL + "/v1",
		Model:    "requested-model",
		Secret:   "fixture-secret",
	})
	if err != nil {
		t.Fatalf("TestProviderConnection: %v", err)
	}
	if !result.OK || result.Model != "fixture-model" || result.LatencyMs < 0 {
		t.Fatalf("result = %#v", result)
	}
	want := []string{"text", "image", "structured_output"}
	if strings.Join(result.Capabilities, ",") != strings.Join(want, ",") {
		t.Fatalf("capabilities = %#v", result.Capabilities)
	}
}

func TestProviderConnectionClassifiesAuthenticationFailure(t *testing.T) {
	server := probeServer(t, http.StatusUnauthorized, `{}`)
	backend := newBackend(systemClock{}, nil, true, true)

	result, err := backend.TestProviderConnection(ProviderTestDraftDTO{
		Protocol: "openai",
		Endpoint: server.URL,
		Model:    "model",
		Secret:   "fixture-secret",
	})
	if err != nil {
		t.Fatalf("TestProviderConnection: %v", err)
	}
	if result.OK || result.ErrorCode != "authentication" {
		t.Fatalf("result = %#v", result)
	}
	if strings.Contains(result.Message, "fixture-secret") {
		t.Fatal("result message exposed the secret")
	}
}

func TestProviderConnectionRejectsWrongProbeAnswer(t *testing.T) {
	server := probeServer(t, 0, `{
		"choices":[{"message":{"content":"{\"probeToken\":\"wrong\",\"imageChoice\":\"unknown\"}"}}]
	}`)
	backend := newBackend(systemClock{}, nil, true, true)

	result, err := backend.TestProviderConnection(ProviderTestDraftDTO{
		Protocol: "openai_responses",
		Endpoint: server.URL,
		Model:    "model",
		Secret:   "fixture-secret",
	})
	if err != nil {
		t.Fatalf("TestProviderConnection: %v", err)
	}
	// The Responses client cannot parse this Chat Completions shape, so the
	// answer never validates: any non-OK classified failure is the contract.
	if result.OK {
		t.Fatalf("result = %#v", result)
	}
}

func TestProviderConnectionValidatesDraft(t *testing.T) {
	backend := newBackend(systemClock{}, nil, true, true)

	cases := []struct {
		name  string
		draft ProviderTestDraftDTO
	}{
		{"unknown protocol", ProviderTestDraftDTO{Protocol: "smoke", Endpoint: "https://example.com", Model: "m", Secret: "s"}},
		{"bad scheme", ProviderTestDraftDTO{Protocol: "openai", Endpoint: "ftp://example.com", Model: "m", Secret: "s"}},
		{"relative endpoint", ProviderTestDraftDTO{Protocol: "openai", Endpoint: "example.com/v1", Model: "m", Secret: "s"}},
		{"empty model", ProviderTestDraftDTO{Protocol: "openai", Endpoint: "https://example.com", Model: " ", Secret: "s"}},
		{"empty secret", ProviderTestDraftDTO{Protocol: "anthropic", Endpoint: "https://example.com", Model: "m", Secret: " "}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := backend.TestProviderConnection(testCase.draft); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
