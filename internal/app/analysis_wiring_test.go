package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Jwz-git/Daygo/internal/ai"
)

func TestAnalysisChainKeepsFallbackAcrossBatches(t *testing.T) {
	var primaryCalls, fallbackCalls atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		primaryCalls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer primary.Close()
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fallbackCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"fallback"},"finish_reason":"stop"}]}`)
	}))
	defer fallback.Close()

	backend, _, _ := backendWithStoreAndSecrets(t)
	input := validProviderInput()
	input.Endpoint = primary.URL + "/v1"
	primaryID, err := backend.AddProvider(input)
	if err != nil {
		t.Fatal(err)
	}
	input.DisplayName, input.Endpoint = "Fallback", fallback.URL+"/v1"
	fallbackID, err := backend.AddProvider(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.SetProviderRouting(ProviderRoutingDTO{Chain: []ProviderRoutingEntryDTO{
		{ProviderID: primaryID}, {ProviderID: fallbackID},
	}}); err != nil {
		t.Fatal(err)
	}
	source := newAnalysisChainSource(backend)
	request := ai.Request{Purpose: ai.PurposeCards, Parts: []ai.Part{ai.TextPart("fixture")}}
	for range 2 {
		chain, err := source.AnalysisChain(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if result, err := chain.Generate(t.Context(), request); err != nil || result.Text != "fallback" {
			t.Fatalf("generation: result=%+v err=%v", result, err)
		}
	}
	if primaryCalls.Load() != 1 || fallbackCalls.Load() != 2 {
		t.Fatalf("primary=%d fallback=%d, want 1 and 2", primaryCalls.Load(), fallbackCalls.Load())
	}
}
