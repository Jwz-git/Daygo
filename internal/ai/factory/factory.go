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
}

// NewClient builds the protocol client for cfg. The returned provider is bare:
// retry, fallback and attempt observation are applied by the caller, so a
// connection test can invoke exactly one attempt.
func NewClient(httpClient *http.Client, cfg Config) (daygoai.Provider, error) {
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
