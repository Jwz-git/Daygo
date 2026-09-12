package app

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform/secrets"
)

// backendWithStoreAndSecrets builds a backend whose keychain is the in-memory
// fake, the shape every provider binding test needs.
func backendWithStoreAndSecrets(t *testing.T) (*Backend, *recordingEmitter, *secrets.Fake) {
	t.Helper()
	backend, emitter := backendWithStore(t)
	fake := secrets.NewFake()
	backend.setSecrets(fake)
	return backend, emitter, fake
}

func validProviderInput() ProviderInputDTO {
	return ProviderInputDTO{
		DisplayName: "Fixture Provider",
		Protocol:    "openai",
		Endpoint:    "https://api.example.com/v1",
		Model:       "fixture-model",
	}
}

func TestProviderCRUDRoundTrip(t *testing.T) {
	backend, _, _ := backendWithStoreAndSecrets(t)

	id, err := backend.AddProvider(validProviderInput())
	if err != nil {
		t.Fatalf("AddProvider: %v", err)
	}
	if id == "" {
		t.Fatal("AddProvider returned an empty id")
	}

	list, err := backend.ListProviders()
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	if len(list) != 1 || list[0].ID != id || list[0].DisplayName != "Fixture Provider" {
		t.Fatalf("list = %+v", list)
	}

	input := validProviderInput()
	input.DisplayName = "Renamed"
	input.Model = "other-model"
	if err := backend.UpdateProvider(id, input); err != nil {
		t.Fatalf("UpdateProvider: %v", err)
	}
	list, _ = backend.ListProviders()
	if len(list) != 1 || list[0].DisplayName != "Renamed" || list[0].Model != "other-model" {
		t.Fatalf("update did not apply: %+v", list)
	}

	if err := backend.DeleteProvider(id); err != nil {
		t.Fatalf("DeleteProvider: %v", err)
	}
	list, _ = backend.ListProviders()
	if len(list) != 0 {
		t.Fatalf("list after delete = %+v", list)
	}
}

// The canary secret must never surface in any DTO, error string, or event.
func TestProviderSecretNeverCrossesTheBoundary(t *testing.T) {
	backend, emitter, _ := backendWithStoreAndSecrets(t)
	canary := "sk-canary-NEVER-LEAK-000000"

	input := validProviderInput()
	input.Secret = canary
	_, err := backend.AddProvider(input)
	if err != nil {
		t.Fatalf("AddProvider: %v", err)
	}

	list, err := backend.ListProviders()
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	for _, dto := range list {
		encoded := dto.DisplayName + dto.Endpoint + dto.Model + dto.Protocol + dto.ID
		if strings.Contains(encoded, canary) {
			t.Fatalf("secret leaked into a DTO: %+v", dto)
		}
	}

	// Error paths: unknown provider, invalid input with a secret attached.
	_, err = backend.AddProvider(ProviderInputDTO{DisplayName: "x", Protocol: "bad", Endpoint: "https://e.example.com", Model: "m", Secret: canary})
	if err == nil {
		t.Fatal("invalid protocol accepted")
	}
	if strings.Contains(err.Error(), canary) {
		t.Fatalf("secret leaked into an error: %q", err.Error())
	}
	for _, event := range emitter.events {
		if strings.Contains(fmt.Sprintf("%v", event.payload), canary) {
			t.Fatalf("secret leaked into an event: %v", event.payload)
		}
	}
}

func TestProviderSecretLifecycle(t *testing.T) {
	backend, _, fake := backendWithStoreAndSecrets(t)
	ctx := context.Background()

	id, err := backend.AddProvider(validProviderInput())
	if err != nil {
		t.Fatalf("AddProvider: %v", err)
	}
	list, _ := backend.ListProviders()
	if list[0].HasSecret {
		t.Fatal("HasSecret true before any key was stored")
	}

	if err := backend.SetProviderSecret(id, "sk-fixture-key"); err != nil {
		t.Fatalf("SetProviderSecret: %v", err)
	}
	stored, err := fake.Get(ctx, id)
	if err != nil || stored != "sk-fixture-key" {
		t.Fatalf("fake keychain = (%q, %v)", stored, err)
	}
	list, _ = backend.ListProviders()
	if !list[0].HasSecret {
		t.Fatal("HasSecret false after storing a key")
	}

	if err := backend.DeleteProviderSecret(id); err != nil {
		t.Fatalf("DeleteProviderSecret: %v", err)
	}
	list, _ = backend.ListProviders()
	if list[0].HasSecret {
		t.Fatal("HasSecret true after deleting the key")
	}

	// Deleting an absent key is not an error.
	if err := backend.DeleteProviderSecret(id); err != nil {
		t.Fatalf("DeleteProviderSecret absent = %v, want nil", err)
	}
}

func TestProviderValidation(t *testing.T) {
	backend, _, _ := backendWithStoreAndSecrets(t)

	cases := []struct {
		name  string
		input ProviderInputDTO
	}{
		{"empty name", ProviderInputDTO{Protocol: "openai", Endpoint: "https://e.example.com", Model: "m"}},
		{"bad protocol", ProviderInputDTO{DisplayName: "x", Protocol: "smtp", Endpoint: "https://e.example.com", Model: "m"}},
		{"relative endpoint", ProviderInputDTO{DisplayName: "x", Protocol: "openai", Endpoint: "api.example.com/v1", Model: "m"}},
		{"non-http scheme", ProviderInputDTO{DisplayName: "x", Protocol: "openai", Endpoint: "ftp://e.example.com", Model: "m"}},
		{"empty model", ProviderInputDTO{DisplayName: "x", Protocol: "openai", Endpoint: "https://e.example.com"}},
	}
	for _, tc := range cases {
		if _, err := backend.AddProvider(tc.input); err == nil {
			t.Errorf("%s: AddProvider accepted invalid input", tc.name)
		}
	}
}

func TestProviderRoutingChain(t *testing.T) {
	backend, emitter, _ := backendWithStoreAndSecrets(t)

	id1, err := backend.AddProvider(validProviderInput())
	if err != nil {
		t.Fatalf("AddProvider 1: %v", err)
	}
	input := validProviderInput()
	input.DisplayName = "Second"
	id2, err := backend.AddProvider(input)
	if err != nil {
		t.Fatalf("AddProvider 2: %v", err)
	}

	if err := backend.SetProviderRouting(ProviderRoutingDTO{Chain: []string{id1, id2}}); err != nil {
		t.Fatalf("SetProviderRouting: %v", err)
	}
	routing, err := backend.GetProviderRouting()
	if err != nil {
		t.Fatalf("GetProviderRouting: %v", err)
	}
	if len(routing.Chain) != 2 || routing.Chain[0] != id1 || routing.Chain[1] != id2 {
		t.Fatalf("routing = %+v", routing)
	}

	// Every write emits settings:changed so the UI re-pulls.
	if emitter.count(EventSettingsChanged) == 0 {
		t.Fatal("provider writes did not emit settings:changed")
	}

	// Unknown id rejected.
	if err := backend.SetProviderRouting(ProviderRoutingDTO{Chain: []string{"no-such-id"}}); err == nil {
		t.Fatal("routing accepted an unknown provider id")
	}
	// Duplicate rejected.
	if err := backend.SetProviderRouting(ProviderRoutingDTO{Chain: []string{id1, id1}}); err == nil {
		t.Fatal("routing accepted a duplicate provider id")
	}
}

// Deleting a provider must prune it from the routing chain.
func TestDeleteProviderPrunesRouting(t *testing.T) {
	backend, _, _ := backendWithStoreAndSecrets(t)

	id1, _ := backend.AddProvider(validProviderInput())
	input := validProviderInput()
	input.DisplayName = "Second"
	id2, _ := backend.AddProvider(input)

	if err := backend.SetProviderRouting(ProviderRoutingDTO{Chain: []string{id1, id2}}); err != nil {
		t.Fatalf("SetProviderRouting: %v", err)
	}
	if err := backend.DeleteProvider(id1); err != nil {
		t.Fatalf("DeleteProvider: %v", err)
	}

	routing, err := backend.GetProviderRouting()
	if err != nil {
		t.Fatalf("GetProviderRouting: %v", err)
	}
	if len(routing.Chain) != 1 || routing.Chain[0] != id2 {
		t.Fatalf("routing after delete = %+v, want only id2", routing)
	}
}

func TestProviderOperationsWithoutStoreFail(t *testing.T) {
	backend := newBackend(fixedClock{}, nil, nil, false, false)

	if _, err := backend.ListProviders(); err == nil {
		t.Fatal("ListProviders without a store succeeded")
	} else {
		var appErr *apperr.Error
		if !asAppErr(err, &appErr) || appErr.Code != apperr.DatabaseError {
			t.Fatalf("ListProviders error = %v, want database_error", err)
		}
	}
}

// TestProvider without a stored key is invalid_argument, not a probe attempt.
func TestProviderWithoutKeyIsRejected(t *testing.T) {
	backend, _, _ := backendWithStoreAndSecrets(t)

	id, err := backend.AddProvider(validProviderInput())
	if err != nil {
		t.Fatalf("AddProvider: %v", err)
	}
	_, err = backend.TestProvider(id)
	var appErr *apperr.Error
	if !asAppErr(err, &appErr) || appErr.Code != apperr.InvalidArgument {
		t.Fatalf("TestProvider without key = %v, want invalid_argument", err)
	}
}

func asAppErr(err error, target **apperr.Error) bool {
	if e, ok := err.(*apperr.Error); ok {
		*target = e
		return true
	}
	return false
}
