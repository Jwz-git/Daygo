package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform/secrets"
)

// standupFixtureServer serves one OpenAI-shaped completion whose content is
// the model's standup JSON. It records the request body so the test can
// assert the prompt carried the day's activity, and lets the caller break the
// response to exercise the failure paths.
func standupFixtureServer(t *testing.T, content string) (*httptest.Server, *[]string) {
	t.Helper()
	requests := &[]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read fixture request: %v", err)
		}
		*requests = append(*requests, string(body))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "fixture-model",
			"choices": []any{map[string]any{
				"message": map[string]any{"content": content},
			}},
		})
	}))
	t.Cleanup(server.Close)
	return server, requests
}

func standupFixtureContent() string {
	return `{"highlights_title":"完成事项","highlights":["完成了站会生成绑定"],"tasks_title":"下一步","tasks":["补真实 macOS 验证"],"blockers_title":"当前限制","blockers_body":""}`
}

// configureStandupProvider registers an OpenAI-protocol provider against the
// fixture server and makes it the routing chain, so GenerateDailyRecap walks
// the same provider path as the analysis pipeline.
func configureStandupProvider(t *testing.T, backend *Backend, endpoint string) {
	t.Helper()
	id, err := backend.AddProvider(ProviderInputDTO{
		DisplayName: "Fixture Standup Provider",
		Protocol:    "openai",
		Endpoint:    endpoint,
		Model:       "fixture-model",
	})
	if err != nil {
		t.Fatalf("AddProvider: %v", err)
	}
	if err := backend.SetProviderRouting(ProviderRoutingDTO{Chain: []string{id}}); err != nil {
		t.Fatalf("SetProviderRouting: %v", err)
	}
}

func TestGenerateDailyRecapStoresModelOutput(t *testing.T) {
	backend, emitter := backendWithStore(t)
	fake := secrets.NewFake()
	backend.setSecrets(fake)
	seedWeeklyCards(t, backend) // provides the Coding category and one card

	server, requests := standupFixtureServer(t, standupFixtureContent())
	configureStandupProvider(t, backend, server.URL)

	dto, err := backend.GenerateDailyRecap("2026-09-09")
	if err != nil {
		t.Fatalf("GenerateDailyRecap: %v", err)
	}
	if dto.HighlightsTitle != "完成事项" || len(dto.Highlights) != 1 {
		t.Fatalf("dto = %+v", dto)
	}
	if dto.GeneratedAtTs == nil {
		t.Fatal("generatedAtTs is nil")
	}
	if emitter.count(EventRecapUpdated) != 1 {
		t.Fatalf("recap:updated count = %d", emitter.count(EventRecapUpdated))
	}
	if len(*requests) != 1 {
		t.Fatalf("fixture received %d requests", len(*requests))
	}
	// The prompt carries the day's activity, not just the date.
	if !strings.Contains((*requests)[0], "Coding") {
		t.Errorf("prompt did not carry the seeded card: %s", (*requests)[0])
	}

	// The stored entry is readable through the normal Get path.
	got, err := backend.GetDailyRecap("2026-09-09")
	if err != nil {
		t.Fatalf("GetDailyRecap: %v", err)
	}
	if got.HighlightsTitle != dto.HighlightsTitle || got.TasksTitle != dto.TasksTitle {
		t.Fatalf("stored recap = %+v, want %+v", got, dto)
	}
}

func TestGenerateDailyRecapRejectsInvalidInputAndReadOnly(t *testing.T) {
	backend, _ := backendWithStore(t)

	_, err := backend.GenerateDailyRecap("09/09/2026")
	assertAppCode(t, err, apperr.InvalidArgument)

	dir := t.TempDir()
	writerBackendWithStore(t, dir)
	readerStore := openTestStore(t, dir, true)
	reader := newBackend(fixedClock{}, nil, readerStore, false, false)
	reader.setEventEmitter(&recordingEmitter{})
	_, err = reader.GenerateDailyRecap("2026-09-09")
	assertAppCode(t, err, apperr.NotCaptureOwner)
}

func TestGenerateDailyRecapWithoutProvider(t *testing.T) {
	backend, _ := backendWithStore(t)
	seedWeeklyCards(t, backend)

	// No provider configured: the chain is empty and the error must be the
	// dedicated provider_not_configured code, not a generic failure.
	_, err := backend.GenerateDailyRecap("2026-09-09")
	assertAppCode(t, err, apperr.ProviderNotConfigured)
}

func TestGenerateDailyRecapBadModelOutput(t *testing.T) {
	backend, _ := backendWithStore(t)
	fake := secrets.NewFake()
	backend.setSecrets(fake)
	seedWeeklyCards(t, backend)

	server, _ := standupFixtureServer(t, "not json at all")
	configureStandupProvider(t, backend, server.URL)

	_, err := backend.GenerateDailyRecap("2026-09-09")
	assertAppCode(t, err, apperr.ProviderFailed)

	// The failed generation must not have wiped any previously stored recap.
	got, err := backend.GetDailyRecap("2026-09-09")
	if err != nil {
		t.Fatalf("GetDailyRecap: %v", err)
	}
	if got.GeneratedAtTs != nil {
		t.Fatalf("failed generation stored a recap: %+v", got)
	}
}
