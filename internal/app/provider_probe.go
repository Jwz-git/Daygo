package app

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	daygoai "github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/ai/factory"
	"github.com/Jwz-git/Daygo/internal/app/apperr"
)

// ProviderTestDraftDTO is the untrusted form draft for one connection probe.
// The secret crosses the boundary for this call only: it builds a throwaway
// client and is never stored, logged or echoed back.
type ProviderTestDraftDTO struct {
	Protocol string `json:"protocol"`
	Endpoint string `json:"endpoint"`
	Model    string `json:"model"`
	Secret   string `json:"secret"`
}

// ProviderTestResultDTO reports one probe outcome. A failed probe is a result,
// not an error: OK stays false and ErrorCode/Message carry the classified
// reason, so the UI can show it next to the key field instead of a dialog.
type ProviderTestResultDTO struct {
	OK           bool     `json:"ok"`
	Model        string   `json:"model"`
	LatencyMs    int64    `json:"latencyMs"`
	Capabilities []string `json:"capabilities"`
	ErrorCode    string   `json:"errorCode"`
	Message      string   `json:"message"`
}

// TestProviderConnection sends one real probe to the draft provider: fixed
// instruction text, an embedded anonymous PNG and a strict JSON schema. The
// model must echo the probe token and identify the image for OK to be true —
// a bare HTTP 2xx is not a pass. No retry, no fallback, no persistence: draft
// fields are never written anywhere by this call.
func (b *Backend) TestProviderConnection(draft ProviderTestDraftDTO) (ProviderTestResultDTO, error) {
	protocol := daygoai.Protocol(strings.TrimSpace(draft.Protocol))
	if !protocol.Valid() {
		return ProviderTestResultDTO{}, apperr.E(apperr.InvalidArgument, "unknown provider protocol", nil)
	}

	endpoint, err := normalizeTestEndpoint(draft.Endpoint)
	if err != nil {
		return ProviderTestResultDTO{}, apperr.E(apperr.InvalidArgument, "endpoint must be a full http:// or https:// address", err)
	}
	model := strings.TrimSpace(draft.Model)
	if model == "" {
		return ProviderTestResultDTO{}, apperr.E(apperr.InvalidArgument, "model is required for the test", nil)
	}
	secret := strings.TrimSpace(draft.Secret)
	if secret == "" {
		return ProviderTestResultDTO{}, apperr.E(apperr.InvalidArgument, "api key is required for the test", nil)
	}

	// No client timeout: the 30-second probe deadline in ai.TestConnection
	// governs the whole exchange, including dial and TLS handshake.
	provider, err := factory.NewClient(&http.Client{}, factory.Config{
		Protocol: protocol,
		Endpoint: endpoint,
		Model:    model,
		Secret:   secret,
	})
	if err != nil {
		return testFailure(err), nil
	}

	result, err := daygoai.TestConnection(context.Background(), provider)
	if err != nil {
		return testFailure(err), nil
	}

	capabilities := make([]string, len(result.Capabilities))
	for index, capability := range result.Capabilities {
		capabilities[index] = string(capability)
	}
	return ProviderTestResultDTO{
		OK:           true,
		Model:        result.Model,
		LatencyMs:    result.Latency.Milliseconds(),
		Capabilities: capabilities,
	}, nil
}

// testFailure maps a probe error to a result row. Messages come from the ai
// layer's fixed sanitized strings; provider response bodies never reach them.
func testFailure(err error) ProviderTestResultDTO {
	return ProviderTestResultDTO{
		OK:        false,
		ErrorCode: string(daygoai.ErrorKindOf(err)),
		Message:   strings.TrimPrefix(err.Error(), "ai: "),
	}
}

// normalizeTestEndpoint accepts an absolute http(s) base URL and strips query,
// fragment and trailing slashes — the same shape the form's own validator
// produces, so both sides agree on what gets appended a request path.
func normalizeTestEndpoint(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", daygoai.NewError(daygoai.ErrorInvalidRequest, "endpoint is required", 0, nil)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return "", daygoai.NewError(daygoai.ErrorInvalidRequest, "endpoint is not an absolute URL", 0, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", daygoai.NewError(daygoai.ErrorInvalidRequest, "endpoint scheme must be http or https", 0, nil)
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}
