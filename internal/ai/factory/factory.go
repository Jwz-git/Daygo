package factory

import (
	"net/http"

	daygoai "github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/ai/anthropic"
	"github.com/Jwz-git/Daygo/internal/ai/openai"
)

// Config describes one provider connection. Secret stays in this struct and in
// the constructed client; it must not be logged or persisted by callers.
type Config struct {
	Protocol daygoai.Protocol
	Endpoint string
	Model    string
	Secret   string
	// UserAgent overrides the User-Agent header on every request to this
	// provider. Empty keeps the Go/SDK default.
	UserAgent string
}

// NewClient builds the protocol client for cfg. The returned provider is bare:
// retry, fallback and attempt observation are applied by the caller, so a
// connection test can invoke exactly one attempt.
func NewClient(httpClient *http.Client, cfg Config) (daygoai.Provider, error) {
	httpClient = withUserAgent(httpClient, cfg.UserAgent)
	switch cfg.Protocol {
	case daygoai.ProtocolOpenAIChat:
		return openai.NewClient(httpClient, cfg.Endpoint, cfg.Model, cfg.Secret)
	case daygoai.ProtocolOpenAIResponses:
		return openai.NewResponsesClient(httpClient, cfg.Endpoint, cfg.Model, cfg.Secret)
	case daygoai.ProtocolAnthropicMessages:
		return anthropic.NewClient(httpClient, cfg.Endpoint, cfg.Model, cfg.Secret)
	default:
		return nil, daygoai.NewError(daygoai.ErrorInvalidRequest, "unknown provider protocol", 0, nil)
	}
}

// userAgentTransport stamps a fixed User-Agent on every request it forwards.
type userAgentTransport struct {
	base      http.RoundTripper
	userAgent string
}

func (t userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// RoundTripper's contract forbids mutating the caller's request, and a retry
	// may reuse it, so the header is set on a clone.
	clone := req.Clone(req.Context())
	clone.Header.Set("User-Agent", t.userAgent)
	return t.base.RoundTrip(clone)
}

// withUserAgent returns a client whose requests carry userAgent, or the input
// unchanged when empty. Applying it at the transport layer, not per protocol
// client, is what lets it override the Anthropic SDK's own default User-Agent.
func withUserAgent(client *http.Client, userAgent string) *http.Client {
	if userAgent == "" {
		return client
	}
	if client == nil {
		client = http.DefaultClient
	}
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	clone := *client
	clone.Transport = userAgentTransport{base: base, userAgent: userAgent}
	return &clone
}
