package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/ai/factory"
	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform/secrets"
)

// ProviderPlaygroundRequestDTO holds one transient user-selected input, never a path or key.
type ProviderPlaygroundRequestDTO struct {
	ProviderID  string `json:"providerId"`
	Model       string `json:"model"`
	Text        string `json:"text"`
	ImageType   string `json:"imageType"`
	ImageBase64 string `json:"imageBase64"`
}

type ProviderPlaygroundResultDTO struct {
	OK        bool   `json:"ok"`
	Text      string `json:"text"`
	Model     string `json:"model"`
	LatencyMs int64  `json:"latencyMs"`
	ErrorCode string `json:"errorCode"`
}

func playgroundParts(req ProviderPlaygroundRequestDTO) ([]ai.Part, error) {
	invalid := func() ([]ai.Part, error) {
		return nil, apperr.E(apperr.InvalidArgument, "invalid playground text or image", nil)
	}
	if !utf8.ValidString(req.Text) || utf8.RuneCountInString(req.Text) > 16000 {
		return invalid()
	}
	parts := make([]ai.Part, 0, 2)
	if strings.TrimSpace(req.Text) != "" {
		parts = append(parts, ai.TextPart(req.Text))
	}
	if req.ImageBase64 != "" {
		if len(req.ImageBase64) > base64.StdEncoding.EncodedLen(ai.MaxImageBytes) {
			return invalid()
		}
		data, err := base64.StdEncoding.Strict().DecodeString(req.ImageBase64)
		if err != nil || len(data) > ai.MaxImageBytes {
			return invalid()
		}
		config, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || (format != "png" && format != "jpeg") || "image/"+format != req.ImageType || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 20000000 {
			return invalid()
		}
		part, err := ai.ImagePart(ai.MediaType(req.ImageType), data)
		if err != nil {
			return invalid()
		}
		parts = append(parts, part)
	} else if req.ImageType != "" {
		return invalid()
	}
	if len(parts) == 0 {
		return invalid()
	}
	return parts, nil
}

// TryProvider makes exactly one unstructured request. Only attempt metadata is persisted.
// The synchronous call owns its deadline; no task goroutine or chat history is created.
func (b *Backend) TryProvider(req ProviderPlaygroundRequestDTO) (ProviderPlaygroundResultDTO, error) {
	parts, err := playgroundParts(req)
	if err != nil {
		return ProviderPlaygroundResultDTO{}, err
	}
	repo, err := b.providerStore()
	if err != nil {
		return ProviderPlaygroundResultDTO{}, err
	}
	if writable, owner := b.instanceOwnership(); !writable || !owner {
		return ProviderPlaygroundResultDTO{}, apperr.E(apperr.NotCaptureOwner, "model trials require the writable owner", nil)
	}
	if err := b.providerSecrets(); err != nil {
		return ProviderPlaygroundResultDTO{}, err
	}
	if req.ProviderID == "" || strings.TrimSpace(req.Model) == "" {
		return ProviderPlaygroundResultDTO{}, apperr.E(apperr.InvalidArgument, "provider and model are required", nil)
	}
	b.windowCtxMu.Lock()
	parent := b.windowCtx
	b.windowCtxMu.Unlock()
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	row, err := repo.Get(ctx, req.ProviderID)
	if err != nil {
		return ProviderPlaygroundResultDTO{}, mapStorageError("get playground provider", err)
	}
	model, err := resolveTestModel(row.Models, req.Model)
	if err != nil {
		return ProviderPlaygroundResultDTO{}, err
	}
	secret, err := b.secrets.Get(ctx, row.ID)
	if err != nil {
		if secrets.IsNotFound(err) {
			return ProviderPlaygroundResultDTO{}, apperr.E(apperr.InvalidArgument, "no api key stored for this provider", nil)
		}
		return ProviderPlaygroundResultDTO{}, apperr.E(apperr.NativeUnavailable, "reading the stored api key failed", nil)
	}
	provider, err := factory.NewClient(&http.Client{}, factory.Config{Protocol: ai.Protocol(row.Protocol), Endpoint: row.Endpoint, Model: model, Secret: secret})
	if err != nil {
		return ProviderPlaygroundResultDTO{ErrorCode: string(ai.ErrorKindOf(err))}, nil
	}
	provider = playgroundResponseFilter{provider: provider, secret: secret}
	sink := attemptSink{repo: b.store().LlmCalls()}
	var latencyMs int64
	provider = ai.WithAttemptObserver(provider, row.ID, ai.Protocol(row.Protocol), model, ai.AttemptObserverFunc(func(_ context.Context, attempt ai.Attempt) {
		latencyMs = attempt.FinishedAt.Sub(attempt.StartedAt).Milliseconds()
		dbCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		sink.RecordAttempt(dbCtx, attempt)
	}))
	result, err := provider.Generate(ctx, ai.Request{Purpose: ai.PurposeTest, Parts: parts, MaxOutputTokens: 2048, MaxImages: 1})
	out := ProviderPlaygroundResultDTO{Model: model, LatencyMs: latencyMs}
	if err != nil {
		out.ErrorCode = string(ai.ErrorKindOf(err))
		return out, nil
	}
	out.OK, out.Text = true, result.Text
	if result.Model != "" {
		out.Model = result.Model
	}
	return out, nil
}

// Filter before observation as well as before the DTO. Gateways control the reply
// and actual-model field, so neither may echo the write-only credential.
type playgroundResponseFilter struct {
	provider ai.Provider
	secret   string
}

func (p playgroundResponseFilter) Generate(ctx context.Context, req ai.Request) (ai.Result, error) {
	result, err := p.provider.Generate(ctx, req)
	if p.secret != "" {
		result.Text = strings.ReplaceAll(result.Text, p.secret, "[redacted]")
		result.Model = strings.ReplaceAll(result.Model, p.secret, "[redacted]")
	}
	if err == nil && strings.TrimSpace(result.Text) == "" {
		err = ai.NewError(ai.ErrorInvalidOutput, "model trial returned no text", 0, nil)
	}
	return result, err
}
