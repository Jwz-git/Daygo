package anthropic

import (
	"context"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	daygoai "github.com/Jwz-git/Daygo/internal/ai"
)

type Client struct {
	client anthropicsdk.Client
	model  string
}

func NewClient(httpClient *http.Client, endpoint, model, secret string) (*Client, error) {
	base, err := daygoai.AnthropicBaseURL(endpoint)
	if err != nil {
		return nil, err
	}
	if model == "" {
		return nil, daygoai.NewError(daygoai.ErrorInvalidRequest, "provider model is required", 0, nil)
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	client := anthropicsdk.NewClient(
		// Daygo resolves credentials itself. SDK environment/profile defaults
		// must not add unrelated authorization headers or credential sources.
		option.WithoutEnvironmentDefaults(),
		option.WithAPIKey(secret),
		option.WithBaseURL(base),
		option.WithHTTPClient(httpClient),
		option.WithMaxRetries(0),
	)
	return &Client{client: client, model: model}, nil
}

func (c *Client) Generate(ctx context.Context, request daygoai.Request) (daygoai.Result, error) {
	if err := request.Validate(); err != nil {
		return daygoai.Result{}, err
	}
	blocks := make([]anthropicsdk.ContentBlockParamUnion, 0, len(request.Parts))
	for _, part := range request.Parts {
		switch part.Kind() {
		case daygoai.PartText:
			blocks = append(blocks, anthropicsdk.NewTextBlock(part.Text()))
		case daygoai.PartImage:
			blocks = append(blocks, anthropicsdk.NewImageBlockBase64(
				string(part.MediaType()), base64.StdEncoding.EncodeToString(part.Bytes()),
			))
		}
	}
	maxTokens := int64(request.MaxOutputTokens)
	if maxTokens == 0 {
		maxTokens = 4096
	}
	params := anthropicsdk.MessageNewParams{
		MaxTokens: maxTokens,
		Messages:  []anthropicsdk.MessageParam{anthropicsdk.NewUserMessage(blocks...)},
		Model:     anthropicsdk.Model(c.model),
	}
	if request.Output != nil {
		schema, err := outputSchema(request.Output.Schema)
		if err != nil {
			return daygoai.Result{}, err
		}
		params.OutputConfig = anthropicsdk.OutputConfigParam{
			Format: anthropicsdk.JSONOutputFormatParam{Schema: schema},
		}
	}
	message, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return daygoai.Result{}, mapError(ctx, err, request.Output != nil)
	}
	result := daygoai.Result{
		Usage: daygoai.Usage{
			InputTokens:      optionalUsage(message.Usage.JSON.InputTokens.Valid(), message.Usage.InputTokens),
			OutputTokens:     optionalUsage(message.Usage.JSON.OutputTokens.Valid(), message.Usage.OutputTokens),
			CacheReadTokens:  optionalUsage(message.Usage.JSON.CacheReadInputTokens.Valid(), message.Usage.CacheReadInputTokens),
			CacheWriteTokens: optionalUsage(message.Usage.JSON.CacheCreationInputTokens.Valid(), message.Usage.CacheCreationInputTokens),
		},
	}
	// This adapter returns one finished text/JSON generation, not a native
	// tool loop. Missing stop_reason stays compatible with older gateways.
	switch message.StopReason {
	case "", anthropicsdk.StopReasonEndTurn, anthropicsdk.StopReasonStopSequence:
	case anthropicsdk.StopReasonRefusal:
		return result, daygoai.NewError(daygoai.ErrorInvalidOutput, "provider refused the request", 0, nil)
	default:
		return result, daygoai.NewError(daygoai.ErrorInvalidOutput, "provider response did not complete", 0, nil)
	}
	var text strings.Builder
	for _, block := range message.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	if strings.TrimSpace(text.String()) == "" {
		return result, daygoai.NewError(daygoai.ErrorInvalidOutput, "provider returned no text", 0, nil)
	}
	if request.Output != nil {
		structured, err := daygoai.ParseStructuredOutput(text.String(), *request.Output)
		if err != nil {
			return result, err
		}
		result.JSON = structured
	}
	result.Text = text.String()
	result.Model = string(message.Model)
	return result, nil
}

func mapError(ctx context.Context, err error, structuredOutput bool) error {
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return daygoai.NewError(daygoai.ErrorTimeout, "provider request timed out", 0, ctx.Err())
		}
		return daygoai.NewError(daygoai.ErrorCanceled, "provider request canceled", 0, ctx.Err())
	}
	var apiErr *anthropicsdk.Error
	if errors.As(err, &apiErr) {
		status := apiErr.StatusCode
		var mapped *daygoai.Error
		switch {
		case structuredOutput && (status == http.StatusBadRequest || status == http.StatusNotFound || status == http.StatusUnprocessableEntity) && rejectsStructuredOutput(apiErr.RawJSON()):
			mapped = daygoai.NewError(daygoai.ErrorUnsupportedFeature, "provider does not support native structured output", status, nil)
		case status == http.StatusUnauthorized || status == http.StatusForbidden:
			mapped = daygoai.NewError(daygoai.ErrorAuthentication, "provider authentication failed", status, nil)
		case status == http.StatusRequestTimeout:
			mapped = daygoai.NewError(daygoai.ErrorTimeout, "provider request timed out", status, nil)
		case status == http.StatusTooManyRequests:
			mapped = daygoai.NewError(daygoai.ErrorRateLimited, "provider rate limit reached", status, nil)
		case status >= 500:
			mapped = daygoai.NewError(daygoai.ErrorUnavailable, "provider service is unavailable", status, nil)
		default:
			mapped = daygoai.NewError(daygoai.ErrorInvalidRequest, "provider rejected request", status, nil)
		}
		if apiErr.Response != nil {
			mapped.RetryAfter = parseRetryAfter(apiErr.Response.Header.Get("Retry-After"))
		}
		return mapped
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return daygoai.NewError(daygoai.ErrorTimeout, "provider request timed out", 0, err)
	}
	return daygoai.NewError(daygoai.ErrorUnavailable, "provider request failed", 0, err)
}

func parseRetryAfter(value string) time.Duration {
	if seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if delay := time.Until(when); delay > 0 {
			return delay
		}
	}
	return 0
}

func int64Pointer(value int64) *int64 { return &value }

func optionalUsage(present bool, value int64) *int64 {
	if !present {
		return nil
	}
	return int64Pointer(value)
}
