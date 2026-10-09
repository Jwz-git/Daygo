package app

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform/secrets"
)

func TestTryProviderRequiresOwnerAndStoredKey(t *testing.T) {
	b, _, _ := backendWithStoreAndSecrets(t)
	_, err := b.TryProvider(ProviderPlaygroundRequestDTO{ProviderID: "fixture", Model: "fixture", Text: "fixture"})
	requireCode(t, err, apperr.NotCaptureOwner)
	b = playgroundBackend(t)
	id, err := b.AddProvider(validProviderInput())
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.TryProvider(ProviderPlaygroundRequestDTO{ProviderID: id, Model: "fixture-model", Text: "fixture"})
	requireCode(t, err, apperr.InvalidArgument)
}

func TestTryProviderFiltersReflectedSecretBeforeObservation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"model":"fixture-secret","choices":[{"message":{"content":"reply fixture-secret"}}]}`)
	}))
	defer server.Close()
	b := playgroundBackend(t)
	input := validProviderInput()
	input.Endpoint, input.Secret = server.URL, "fixture-secret"
	id, err := b.AddProvider(input)
	if err != nil {
		t.Fatal(err)
	}
	result, err := b.TryProvider(ProviderPlaygroundRequestDTO{ProviderID: id, Model: "fixture-model", Text: "fixture"})
	if err != nil || result.Text != "reply [redacted]" || result.Model != "[redacted]" {
		t.Fatalf("redaction failed: %v", err)
	}
}

type playgroundFixtureProvider func(context.Context, ai.Request) (ai.Result, error)

func (f playgroundFixtureProvider) Generate(ctx context.Context, req ai.Request) (ai.Result, error) {
	return f(ctx, req)
}

func TestPlaygroundFilterRejectsEmptyTextAndSanitizesObservedModel(t *testing.T) {
	p := playgroundResponseFilter{secret: "fixture-secret", provider: playgroundFixtureProvider(func(context.Context, ai.Request) (ai.Result, error) {
		return ai.Result{Model: "fixture-secret", Text: "  "}, nil
	})}
	var observed ai.Attempt
	wrapped := ai.WithAttemptObserver(p, "fixture", ai.ProtocolOpenAIChat, "fixture", ai.AttemptObserverFunc(func(_ context.Context, attempt ai.Attempt) { observed = attempt }))
	_, err := wrapped.Generate(context.Background(), ai.Request{Purpose: ai.PurposeTest, Parts: []ai.Part{ai.TextPart("fixture")}})
	if ai.ErrorKindOf(err) != ai.ErrorInvalidOutput || observed.ActualModel != "[redacted]" || observed.Outcome != "failed" {
		t.Fatal("observer received unsafe or incorrect outcome")
	}
}

// Anonymous one-pixel PNG; no user files or captures are read by these fixtures.
const playgroundPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII="

func playgroundBackend(t *testing.T) *Backend {
	t.Helper()
	b, _ := writerBackendWithStore(t, t.TempDir())
	b.setSecrets(secrets.NewFake())
	return b
}

func TestTryProviderSendsUserInputWithoutSchemaOrFallback(t *testing.T) {
	for _, tc := range []struct{ protocol, path, response string }{
		{"openai", "/chat/completions", `{"model":"actual","choices":[{"message":{"content":"<script>fixture</script> reply"}}]}`},
		{"openai_responses", "/responses", `{"model":"actual","output":[{"type":"message","content":[{"type":"output_text","text":"<script>fixture</script> reply"}]}]}`},
		{"anthropic", "/v1/messages", `{"model":"actual","content":[{"type":"text","text":"<script>fixture</script> reply"}]}`},
	} {
		t.Run(tc.protocol, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				body, _ := io.ReadAll(r.Body)
				if r.URL.Path != tc.path || !strings.Contains(string(body), "describe fixture") || !strings.Contains(string(body), playgroundPNG) || strings.Contains(string(body), "json_schema") {
					t.Errorf("unexpected request path or input")
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, tc.response)
			}))
			defer server.Close()
			b := playgroundBackend(t)
			input := validProviderInput()
			input.Protocol, input.Endpoint, input.Secret = tc.protocol, server.URL, "fixture-secret"
			id, err := b.AddProvider(input)
			if err != nil {
				t.Fatal(err)
			}
			result, err := b.TryProvider(ProviderPlaygroundRequestDTO{ProviderID: id, Model: "fixture-model", Text: "describe fixture", ImageBase64: playgroundPNG, ImageType: "image/png"})
			if err != nil || !result.OK || result.Text != "<script>fixture</script> reply" || result.Model != "actual" || calls != 1 {
				t.Fatalf("unexpected result: %+v, %v, calls=%d", result, err, calls)
			}
		})
	}
}

func TestAnthropicSavedPastedEndpointWorksForTrialAndModels(t *testing.T) {
	for _, suffix := range []string{"/v1", "/v1/messages", "/proxy/anthropic/v1/messages/"} {
		t.Run(suffix, func(t *testing.T) {
			prefix := ""
			if strings.HasPrefix(suffix, "/proxy") {
				prefix = "/proxy/anthropic"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Api-Key") != "fixture-secret" || r.Header.Get("User-Agent") != "fixture-agent/1.0" {
					t.Error("configured key or User-Agent missing")
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case prefix + "/v1/messages":
					_, _ = io.WriteString(w, `{"model":"fixture-model","content":[{"type":"text","text":"ok"}]}`)
				case prefix + "/v1/models":
					_, _ = io.WriteString(w, `{"data":[{"id":"fixture-model"}]}`)
				default:
					t.Errorf("unexpected path: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			b := playgroundBackend(t)
			input := validProviderInput()
			input.Protocol, input.Endpoint, input.Secret, input.UserAgent = "anthropic", server.URL+suffix, "fixture-secret", "fixture-agent/1.0"
			id, err := b.AddProvider(input)
			if err != nil {
				t.Fatal(err)
			}
			trial, err := b.TryProvider(ProviderPlaygroundRequestDTO{ProviderID: id, Model: "fixture-model", Text: "fixture"})
			if err != nil || !trial.OK || trial.Text != "ok" {
				t.Fatalf("trial = %+v, error = %v", trial, err)
			}
			models, err := b.ListProviderModels(ProviderModelsRequestDTO{ProviderID: id})
			if err != nil || !models.OK || len(models.Models) != 1 || models.Models[0] != "fixture-model" {
				t.Fatalf("models = %+v, error = %v", models, err)
			}
		})
	}
}

// Cover both newly saved URLs and records produced before trailing-slash
// normalization was fixed. Reading the latter must not rewrite the record.
func TestOpenAISavedPastedEndpointWorksForTrialAndModels(t *testing.T) {
	for _, protocol := range []string{"openai", "openai_responses"} {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%v", protocol, legacy), func(t *testing.T) {
				path := "/chat/completions"
				response := `{"choices":[{"message":{"content":"ok"}}]}`
				if protocol == "openai_responses" {
					path = "/responses"
					response = `{"output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}]}`
				}
				posts, gets := 0, 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("Authorization") != "Bearer fixture-secret" || r.Header.Get("User-Agent") != "fixture-agent/1.0" {
						t.Error("configured key or User-Agent missing")
					}
					w.Header().Set("Content-Type", "application/json")
					switch r.URL.Path {
					case "/proxy/v1" + path:
						posts++
						var body map[string]any
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Errorf("decode request: %v", err)
							w.WriteHeader(http.StatusBadRequest)
							return
						}
						if protocol == "openai" && (body["max_completion_tokens"] != float64(2048) || body["max_tokens"] != nil) {
							t.Error("trial must use max_completion_tokens")
						}
						_, _ = io.WriteString(w, response)
					case "/proxy/v1/models":
						gets++
						_, _ = io.WriteString(w, `{"data":[{"id":"fixture-model"}]}`)
					default:
						t.Errorf("unexpected path: %s", r.URL.Path)
						w.WriteHeader(http.StatusNotFound)
					}
				}))
				defer server.Close()
				b := playgroundBackend(t)
				input := validProviderInput()
				input.Protocol, input.Endpoint, input.Secret, input.UserAgent = protocol, server.URL+"/proxy/v1"+path+"/", "fixture-secret", "fixture-agent/1.0"
				id, err := b.AddProvider(input)
				if err != nil {
					t.Fatal(err)
				}
				repo := b.store().Providers()
				row, err := repo.Get(context.Background(), id)
				if err != nil {
					t.Fatal(err)
				}
				if legacy {
					row.Endpoint = server.URL + "/proxy/v1" + path
					if err := repo.Update(context.Background(), id, row); err != nil {
						t.Fatal(err)
					}
				} else if row.Endpoint != server.URL+"/proxy/v1" {
					t.Fatalf("saved endpoint = %s", row.Endpoint)
				}
				trial, err := b.TryProvider(ProviderPlaygroundRequestDTO{ProviderID: id, Model: "fixture-model", Text: "fixture"})
				if err != nil || !trial.OK || trial.Text != "ok" {
					t.Fatalf("trial = %+v, error = %v", trial, err)
				}
				models, err := b.ListProviderModels(ProviderModelsRequestDTO{ProviderID: id})
				if err != nil || !models.OK || len(models.Models) != 1 || models.Models[0] != "fixture-model" || posts != 1 || gets != 1 {
					t.Fatalf("models = %+v, error = %v, posts=%d gets=%d", models, err, posts, gets)
				}
				after, err := repo.Get(context.Background(), id)
				if err != nil || after.Endpoint != row.Endpoint {
					t.Fatal("request changed the saved endpoint")
				}
			})
		}
	}
}

func TestPlaygroundRejectsInvalidInput(t *testing.T) {
	oversized, _ := base64.StdEncoding.DecodeString(playgroundPNG)
	binary.BigEndian.PutUint32(oversized[16:20], 20000001)
	binary.BigEndian.PutUint32(oversized[29:33], crc32.ChecksumIEEE(oversized[12:29]))
	for _, request := range []ProviderPlaygroundRequestDTO{
		{}, {Text: strings.Repeat("x", 16001)},
		{Text: "fixture", ImageType: "image/png"},
		{Text: "fixture", ImageType: "image/png", ImageBase64: "!"},
		{Text: "fixture", ImageType: "image/jpeg", ImageBase64: playgroundPNG},
		{Text: "fixture", ImageType: "image/png", ImageBase64: base64.StdEncoding.EncodeToString([]byte("not an image"))},
		{Text: "fixture", ImageType: "image/png", ImageBase64: strings.Repeat("A", 7<<20)},
		{ImageType: "image/png", ImageBase64: base64.StdEncoding.EncodeToString(oversized)},
	} {
		if _, err := playgroundParts(request); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
	for _, request := range []ProviderPlaygroundRequestDTO{{Text: "fixture"}, {ImageType: "image/png", ImageBase64: playgroundPNG}} {
		if _, err := playgroundParts(request); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTryProviderPropagatesHostCancellation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		cancel()
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
			t.Error("HTTP request did not stop after host cancellation")
		}
	}))
	defer server.Close()
	b := playgroundBackend(t)
	input := validProviderInput()
	input.Endpoint, input.Secret = server.URL, "fixture-secret"
	id, err := b.AddProvider(input)
	if err != nil {
		t.Fatal(err)
	}
	b.windowCtx = parent
	result, err := b.TryProvider(ProviderPlaygroundRequestDTO{ProviderID: id, Model: "fixture-model", Text: "fixture"})
	if err != nil || result.OK || result.ErrorCode != "canceled" {
		t.Fatalf("cancellation result=%+v err=%v", result, err)
	}
}

func TestTryProviderRejectsUnconfiguredModelAndSanitizesFailure(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(401)
		_, _ = io.WriteString(w, `{"error":{"message":"fixture-secret private-input"}}`)
	}))
	defer server.Close()
	b := playgroundBackend(t)
	input := validProviderInput()
	input.Endpoint, input.Secret = server.URL, "fixture-secret"
	id, err := b.AddProvider(input)
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.TryProvider(ProviderPlaygroundRequestDTO{ProviderID: id, Model: "other", Text: "private-input"})
	requireCode(t, err, apperr.InvalidArgument)
	if calls != 0 {
		t.Fatal("invalid model reached network")
	}
	result, err := b.TryProvider(ProviderPlaygroundRequestDTO{ProviderID: id, Model: "fixture-model", Text: "private-input"})
	if err != nil || result.OK || result.ErrorCode != "authentication" || calls != 1 {
		t.Fatalf("result=%+v err=%v calls=%d", result, err, calls)
	}
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), "fixture-secret") || strings.Contains(string(encoded), "private-input") {
		t.Fatal("failure leaked content")
	}
}
