package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// modelsDeadline bounds one ListModels call: user-triggered, interactive, no
// retry — the connection probe's 30s ceiling is the precedent; listing is
// lighter so it gets half.
const modelsDeadline = 15 * time.Second

// modelsResponseBytes caps the models payload. Real lists are a few KiB; the
// cap exists so a hostile or broken endpoint cannot balloon memory.
const modelsResponseBytes = 1 << 20

// maxModels caps the returned list. A user picks from a dropdown; anything
// beyond this is noise.
const maxModels = 100

// ListModels fetches the available model ids from a provider's endpoint.
// One request, no retry, no fallback: it is a convenience for the model
// dropdown, and a provider without a models endpoint reports a classified
// error the user answers by typing the model name manually.
func ListModels(ctx context.Context, protocol Protocol, endpoint, secret string) ([]string, error) {
	if !protocol.Valid() {
		return nil, NewError(ErrorInvalidRequest, "unknown provider protocol", 0, nil)
	}
	ctx, cancel := context.WithTimeout(ctx, modelsDeadline)
	defer cancel()

	var requestURL string
	var header func(*http.Request)
	switch protocol {
	case ProtocolAnthropicMessages:
		// The Anthropic models endpoint lives under /v1; the configured
		// endpoint already carries the version prefix for Messages.
		requestURL = strings.TrimRight(endpoint, "/") + "/v1/models"
		header = func(r *http.Request) {
			r.Header.Set("x-api-key", secret)
			r.Header.Set("anthropic-version", "2023-06-01")
		}
	default:
		// openai and openai_responses share the same /models listing.
		requestURL = strings.TrimRight(endpoint, "/") + "/models"
		header = func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+secret)
		}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, NewError(ErrorInvalidRequest, "cannot create models request", 0, err)
	}
	header(request)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, modelsTransportError(ctx, err)
	}
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(response.Body, modelsResponseBytes))
	if err != nil {
		return nil, NewError(ErrorUnavailable, "cannot read models response", response.StatusCode, err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, modelsStatusError(response.StatusCode)
	}

	// Both protocol families return {"data": [{"id": "..."}, ...]}.
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, NewError(ErrorInvalidOutput, "models response is not valid JSON", response.StatusCode, err)
	}

	models := make([]string, 0, len(payload.Data))
	for _, entry := range payload.Data {
		if id := strings.TrimSpace(entry.ID); id != "" {
			models = append(models, id)
		}
	}
	sort.Strings(models)
	if len(models) > maxModels {
		models = models[:maxModels]
	}
	return models, nil
}

// modelsTransportError classifies a failed HTTP round trip the same way the
// protocol clients do (timeout / cancellation / everything else unavailable).
func modelsTransportError(ctx context.Context, err error) error {
	switch {
	case ctx.Err() != nil:
		if ctx.Err() == context.DeadlineExceeded {
			return NewError(ErrorTimeout, "models request timed out", 0, ctx.Err())
		}
		return NewError(ErrorCanceled, "models request canceled", 0, ctx.Err())
	default:
		return NewError(ErrorUnavailable, "models request failed", 0, err)
	}
}

// modelsStatusError classifies a non-2xx listing response with the same
// mapping Generate uses, minus the structured-output special case.
func modelsStatusError(status int) error {
	kind := ErrorInvalidRequest
	message := "provider rejected the models request with HTTP " + strconv.Itoa(status)
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		kind = ErrorAuthentication
	case status == http.StatusRequestTimeout:
		kind = ErrorTimeout
	case status == http.StatusTooManyRequests:
		kind = ErrorRateLimited
	case status >= 500:
		kind = ErrorUnavailable
	}
	return NewError(kind, message, status, nil)
}
