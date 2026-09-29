package app

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
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
