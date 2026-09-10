package anthropic

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
	if !validEndpoint(endpoint) {
		return nil, daygoai.NewError(daygoai.ErrorInvalidRequest, "provider endpoint must be an absolute HTTP URL", 0, nil)
	}
	if model == "" {
		return nil, daygoai.NewError(daygoai.ErrorInvalidRequest, "provider model is required", 0, nil)
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	client := anthropicsdk.NewClient(
		option.WithAPIKey(secret),
		option.WithBaseURL(strings.TrimRight(endpoint, "/")+"/"),
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
		var schema map[string]any
		if err := json.Unmarshal(request.Output.Schema, &schema); err != nil {
			return daygoai.Result{}, daygoai.NewError(daygoai.ErrorInvalidRequest, "output schema is invalid", 0, err)
		}
		params.OutputConfig = anthropicsdk.OutputConfigParam{
			Format: anthropicsdk.JSONOutputFormatParam{Schema: schema},
		}
	}
	message, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return daygoai.Result{}, mapError(ctx, err)
	}
	var text strings.Builder
	for _, block := range message.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	if text.Len() == 0 {
		return daygoai.Result{}, daygoai.NewError(daygoai.ErrorInvalidOutput, "provider returned no text", 0, nil)
	}
	result := daygoai.Result{
		Text:  text.String(),
		Model: string(message.Model),
		Usage: daygoai.Usage{
			InputTokens:      int64Pointer(message.Usage.InputTokens),
			OutputTokens:     int64Pointer(message.Usage.OutputTokens),
			CacheReadTokens:  optionalUsage(message.Usage.JSON.CacheReadInputTokens.Valid(), message.Usage.CacheReadInputTokens),
			CacheWriteTokens: optionalUsage(message.Usage.JSON.CacheCreationInputTokens.Valid(), message.Usage.CacheCreationInputTokens),
		},
	}
	if request.Output != nil {
		structured, err := daygoai.ParseStructuredOutput(result.Text, *request.Output)
		if err != nil {
			return daygoai.Result{}, err
		}
		result.JSON = structured
	}
	return result, nil
}

func validEndpoint(endpoint string) bool {
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	return err == nil && request.URL.Host != "" && (request.URL.Scheme == "http" || request.URL.Scheme == "https")
}

func mapError(ctx context.Context, err error) error {
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
