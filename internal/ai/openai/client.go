package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	daygoai "github.com/Jwz-git/Daygo/internal/ai"
)

const maxResponseBytes = 8 << 20

type Client struct {
	httpClient *http.Client
	endpoint   *url.URL
	model      string
	secret     string
}

func NewClient(httpClient *http.Client, endpoint, model, secret string) (*Client, error) {
	base, err := url.Parse(endpoint)
	if err != nil || base.Scheme == "" || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") {
		return nil, daygoai.NewError(daygoai.ErrorInvalidRequest, "provider endpoint must be an absolute HTTP URL", 0, err)
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if model == "" {
		return nil, daygoai.NewError(daygoai.ErrorInvalidRequest, "provider model is required", 0, nil)
	}
	return &Client{httpClient: httpClient, endpoint: base, model: model, secret: secret}, nil
}

func (c *Client) Generate(ctx context.Context, request daygoai.Request) (daygoai.Result, error) {
	if err := request.Validate(); err != nil {
		return daygoai.Result{}, err
	}
	body, err := c.requestBody(request)
	if err != nil {
		return daygoai.Result{}, err
	}
	requestURL := *c.endpoint
	requestURL.Path = strings.TrimRight(requestURL.Path, "/") + "/chat/completions"
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), bytes.NewReader(body))
	if err != nil {
		return daygoai.Result{}, daygoai.NewError(daygoai.ErrorInvalidRequest, "cannot create provider request", 0, err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if c.secret != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+c.secret)
	}

	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return daygoai.Result{}, transportError(ctx, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return daygoai.Result{}, daygoai.NewError(daygoai.ErrorUnavailable, "cannot read provider response", response.StatusCode, err)
	}
	if len(responseBody) > maxResponseBytes {
		return daygoai.Result{}, daygoai.NewError(daygoai.ErrorInvalidOutput, "provider response is too large", response.StatusCode, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return daygoai.Result{}, statusError(
			response.StatusCode,
			retryAfter(response.Header.Get("Retry-After")),
			request.Output != nil,
		)
	}
	return parseResponse(responseBody, request.Output)
}

func (c *Client) requestBody(request daygoai.Request) ([]byte, error) {
	content := make([]map[string]any, 0, len(request.Parts))
	for _, part := range request.Parts {
		switch part.Kind() {
		case daygoai.PartText:
			content = append(content, map[string]any{"type": "text", "text": part.Text()})
		case daygoai.PartImage:
			encoded := base64.StdEncoding.EncodeToString(part.Bytes())
			content = append(content, map[string]any{
				"type":      "image_url",
				"image_url": map[string]any{"url": "data:" + string(part.MediaType()) + ";base64," + encoded},
			})
		}
	}
	payload := map[string]any{
		"model":    c.model,
		"messages": []any{map[string]any{"role": "user", "content": content}},
	}
	if request.MaxOutputTokens > 0 {
		payload["max_tokens"] = request.MaxOutputTokens
	}
	if request.Output != nil {
		var schema any
		if err := json.Unmarshal(request.Output.Schema, &schema); err != nil {
			return nil, daygoai.NewError(daygoai.ErrorInvalidRequest, "output schema is invalid", 0, err)
		}
		payload["response_format"] = map[string]any{
			"type":        "json_schema",
			"json_schema": map[string]any{"name": request.Output.Name, "strict": request.Output.Strict, "schema": schema},
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, daygoai.NewError(daygoai.ErrorInvalidRequest, "cannot encode provider request", 0, err)
	}
	return body, nil
}

type completionResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     *int64 `json:"prompt_tokens"`
		CompletionTokens *int64 `json:"completion_tokens"`
		PromptDetails    *struct {
			CachedTokens *int64 `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
}

func parseResponse(body []byte, output *daygoai.OutputSchema) (daygoai.Result, error) {
	var response completionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return daygoai.Result{}, daygoai.NewError(daygoai.ErrorInvalidOutput, "provider returned invalid JSON", 0, err)
	}
	if len(response.Choices) == 0 || response.Choices[0].Message.Content == "" {
		return daygoai.Result{}, daygoai.NewError(daygoai.ErrorInvalidOutput, "provider returned no text", 0, nil)
	}
	result := daygoai.Result{Text: response.Choices[0].Message.Content, Model: response.Model}
	if response.Usage != nil {
		result.Usage.InputTokens = response.Usage.PromptTokens
		result.Usage.OutputTokens = response.Usage.CompletionTokens
		if response.Usage.PromptDetails != nil {
			result.Usage.CacheReadTokens = response.Usage.PromptDetails.CachedTokens
		}
	}
	if output != nil {
		structured, err := daygoai.ParseStructuredOutput(result.Text, *output)
		if err != nil {
			return daygoai.Result{}, err
		}
		result.JSON = structured
	}
	return result, nil
}

func transportError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return daygoai.NewError(daygoai.ErrorTimeout, "provider request timed out", 0, ctx.Err())
		}
		return daygoai.NewError(daygoai.ErrorCanceled, "provider request canceled", 0, ctx.Err())
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return daygoai.NewError(daygoai.ErrorTimeout, "provider request timed out", 0, err)
	}
	return daygoai.NewError(daygoai.ErrorUnavailable, "provider request failed", 0, err)
}

func statusError(status int, retryDelay time.Duration, structuredOutput bool) error {
	kind := daygoai.ErrorInvalidRequest
	message := fmt.Sprintf("provider rejected request with HTTP %d", status)
	switch {
	case structuredOutput && (status == http.StatusBadRequest || status == http.StatusNotFound || status == http.StatusUnprocessableEntity):
		kind = daygoai.ErrorUnsupportedFeature
		message = "provider does not support native structured output"
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		kind = daygoai.ErrorAuthentication
	case status == http.StatusRequestTimeout:
		kind = daygoai.ErrorTimeout
	case status == http.StatusTooManyRequests:
		kind = daygoai.ErrorRateLimited
	case status >= 500:
		kind = daygoai.ErrorUnavailable
	case status == http.StatusNotFound:
		kind = daygoai.ErrorInvalidRequest
	}
	err := daygoai.NewError(kind, message, status, nil)
	err.RetryAfter = retryDelay
	return err
}

func retryAfter(value string) time.Duration {
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
