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
	"regexp"
	"strconv"
	"strings"
	"time"

	daygoai "github.com/Jwz-git/Daygo/internal/ai"
)

const maxResponseBytes = 8 << 20

type Client struct {
	httpClient *http.Client
	endpoint   string
	model      string
	secret     string
}

func NewClient(httpClient *http.Client, endpoint, model, secret string) (*Client, error) {
	base, err := daygoai.OpenAIBaseURL(endpoint)
	if err != nil {
		return nil, err
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if model == "" {
		return nil, daygoai.NewError(daygoai.ErrorInvalidRequest, "provider model is required", 0, nil)
	}
	return &Client{httpClient: httpClient, endpoint: base + "chat/completions", model: model, secret: secret}, nil
}

func (c *Client) Generate(ctx context.Context, request daygoai.Request) (daygoai.Result, error) {
	if err := request.Validate(); err != nil {
		return daygoai.Result{}, err
	}
	body, err := c.requestBody(request)
	if err != nil {
		return daygoai.Result{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
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
			responseBody,
			request.Output != nil,
			chatUnsupportedKeywords...,
		)
	}
	return parseResponse(responseBody, request.Output)
}

// chatUnsupportedKeywords are the markers a Chat Completions error body uses
// when the rejection is about structured output itself rather than the
// request in general. Status 400/404/422 plus one of these is the only
// combination that maps to unsupported_feature; every other 4xx keeps its
// regular classification (a bad model name or parameter must not be reported
// as "no structured output").
var chatUnsupportedKeywords = []string{"response_format", "json_schema", "json_object"}

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
		// Includes reasoning tokens. max_tokens is deprecated and rejected by
		// o-series models, including those exposed under gateway aliases.
		payload["max_completion_tokens"] = request.MaxOutputTokens
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
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content string `json:"content"`
			Refusal string `json:"refusal"`
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
	// Preserve measured usage for failed attempts, but expose text/JSON only
	// after completion and local validation have both succeeded.
	result := daygoai.Result{}
	if response.Usage != nil {
		result.Usage.InputTokens = response.Usage.PromptTokens
		result.Usage.OutputTokens = response.Usage.CompletionTokens
		if response.Usage.PromptDetails != nil {
			result.Usage.CacheReadTokens = response.Usage.PromptDetails.CachedTokens
		}
	}
	if len(response.Choices) == 0 {
		return result, daygoai.NewError(daygoai.ErrorInvalidOutput, "provider returned no text", 0, nil)
	}
	choice := response.Choices[0]
	if choice.FinishReason == "length" {
		return result, daygoai.NewError(daygoai.ErrorInvalidOutput, daygoai.ErrTokenLimit.Error(), 0, daygoai.ErrTokenLimit)
	}
	if choice.FinishReason != "" && choice.FinishReason != "stop" {
		return result, daygoai.NewError(daygoai.ErrorInvalidOutput, "provider response did not complete", 0, nil)
	}
	if choice.Message.Refusal != "" {
		return result, daygoai.NewError(daygoai.ErrorInvalidOutput, "provider refused the request", 0, nil)
	}
	if strings.TrimSpace(choice.Message.Content) == "" {
		return result, daygoai.NewError(daygoai.ErrorInvalidOutput, "provider returned no text", 0, nil)
	}
	if output != nil {
		structured, err := daygoai.ParseStructuredOutput(choice.Message.Content, *output)
		if err != nil {
			return result, err
		}
		result.JSON = structured
	}
	result.Text = choice.Message.Content
	result.Model = response.Model
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

func statusError(status int, retryDelay time.Duration, body []byte, structuredOutput bool, unsupportedKeywords ...string) error {
	kind := daygoai.ErrorInvalidRequest
	message := fmt.Sprintf("provider rejected request with HTTP %d", status)
	switch {
	case structuredOutput && (status == http.StatusBadRequest || status == http.StatusNotFound || status == http.StatusUnprocessableEntity) &&
		bodyMentions(body, unsupportedKeywords...):
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
	if kind != daygoai.ErrorAuthentication && bodyMentions(body, "rate_limit", "rate limit", "too many requests", "resource_exhausted", "quota exceeded", "quota", "tpm", "tokens per minute") {
		kind = daygoai.ErrorRateLimited
	}
	if retryDelay == 0 && kind == daygoai.ErrorRateLimited {
		retryDelay = extractRetryAfterFromBody(body)
	}
	if detail := providerErrorCode(body); detail != "" {
		message += " (" + detail + ")"
	}
	err := daygoai.NewError(kind, message, status, nil)
	err.RetryAfter = retryDelay
	return err
}

var retryDelayRegex = regexp.MustCompile(`(?i)(?:retry after|try again in|retry_after["\s:]+)\s*(\d+)`)

func extractRetryAfterFromBody(body []byte) time.Duration {
	if len(body) > 4096 {
		body = body[:4096]
	}
	m := retryDelayRegex.FindSubmatch(body)
	if len(m) > 1 {
		if secs, err := strconv.ParseInt(string(m[1]), 10, 64); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return 0
}

// bodyMentions reports whether the error body contains any keyword. Only the
// first 4 KiB are scanned: error payloads put the relevant fields up front,
// and the cap keeps a hostile endpoint from making this expensive.
func bodyMentions(body []byte, keywords ...string) bool {
	if len(body) > 4096 {
		body = body[:4096]
	}
	lowered := strings.ToLower(string(body))
	for _, keyword := range keywords {
		if strings.Contains(lowered, keyword) {
			return true
		}
	}
	return false
}

// providerErrorCode surfaces the provider's machine-readable error code (or
// type) so a 400 tells the user whether it was a bad model, a bad parameter
// or something else. Only short printable values pass; the provider's
// human-readable message never crosses this boundary — it can echo request
// content, so it is used for classification only.
func providerErrorCode(body []byte) string {
	var payload struct {
		Error *struct {
			Type string `json:"type"`
			Code any    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.Error == nil {
		return ""
	}
	if code, ok := payload.Error.Code.(string); ok {
		if detail := printableDetail(code); detail != "" {
			return detail
		}
	}
	return printableDetail(payload.Error.Type)
}

func printableDetail(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 64 {
		return ""
	}
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 || value[i] > 0x7e {
			return ""
		}
	}
	return value
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
