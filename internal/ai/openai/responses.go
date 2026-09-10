package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	daygoai "github.com/Jwz-git/Daygo/internal/ai"
)

type ResponsesClient struct {
	httpClient *http.Client
	endpoint   *url.URL
	model      string
	secret     string
}

func NewResponsesClient(httpClient *http.Client, endpoint, model, secret string) (*ResponsesClient, error) {
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
	return &ResponsesClient{httpClient: httpClient, endpoint: base, model: model, secret: secret}, nil
}

func (c *ResponsesClient) Generate(ctx context.Context, request daygoai.Request) (daygoai.Result, error) {
	if err := request.Validate(); err != nil {
		return daygoai.Result{}, err
	}
	body, err := c.requestBody(request)
	if err != nil {
		return daygoai.Result{}, err
	}
	requestURL := *c.endpoint
	requestURL.Path = strings.TrimRight(requestURL.Path, "/") + "/responses"
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
	return parseResponsesResponse(responseBody, request.Output)
}

func (c *ResponsesClient) requestBody(request daygoai.Request) ([]byte, error) {
	content := make([]map[string]any, 0, len(request.Parts))
	for _, part := range request.Parts {
		switch part.Kind() {
		case daygoai.PartText:
			content = append(content, map[string]any{"type": "input_text", "text": part.Text()})
		case daygoai.PartImage:
			encoded := base64.StdEncoding.EncodeToString(part.Bytes())
			content = append(content, map[string]any{
				"type":      "input_image",
				"image_url": "data:" + string(part.MediaType()) + ";base64," + encoded,
			})
		}
	}
	payload := map[string]any{
		"model": c.model,
		"input": []any{map[string]any{"role": "user", "content": content}},
	}
	if request.MaxOutputTokens > 0 {
		payload["max_output_tokens"] = request.MaxOutputTokens
	}
	if request.Output != nil {
		var schema any
		if err := json.Unmarshal(request.Output.Schema, &schema); err != nil {
			return nil, daygoai.NewError(daygoai.ErrorInvalidRequest, "output schema is invalid", 0, err)
		}
		payload["text"] = map[string]any{
			"format": map[string]any{
				"type":   "json_schema",
				"name":   request.Output.Name,
				"strict": request.Output.Strict,
				"schema": schema,
			},
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, daygoai.NewError(daygoai.ErrorInvalidRequest, "cannot encode provider request", 0, err)
	}
	return body, nil
}

type responsesResponse struct {
	Model  string `json:"model"`
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Usage *struct {
		InputTokens  *int64 `json:"input_tokens"`
		OutputTokens *int64 `json:"output_tokens"`
		InputDetails *struct {
			CachedTokens     *int64 `json:"cached_tokens"`
			CacheWriteTokens *int64 `json:"cache_write_tokens"`
		} `json:"input_tokens_details"`
	} `json:"usage"`
}

func parseResponsesResponse(body []byte, output *daygoai.OutputSchema) (daygoai.Result, error) {
	var response responsesResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return daygoai.Result{}, daygoai.NewError(daygoai.ErrorInvalidOutput, "provider returned invalid JSON", 0, err)
	}
	var text strings.Builder
	for _, item := range response.Output {
		if item.Type != "message" {
			continue
		}
		for _, content := range item.Content {
			if content.Type == "output_text" {
				text.WriteString(content.Text)
			}
		}
	}
	if text.Len() == 0 {
		return daygoai.Result{}, daygoai.NewError(daygoai.ErrorInvalidOutput, "provider returned no text", 0, nil)
	}
	result := daygoai.Result{Text: text.String(), Model: response.Model}
	if response.Usage != nil {
		result.Usage.InputTokens = response.Usage.InputTokens
		result.Usage.OutputTokens = response.Usage.OutputTokens
		if response.Usage.InputDetails != nil {
			result.Usage.CacheReadTokens = response.Usage.InputDetails.CachedTokens
			result.Usage.CacheWriteTokens = response.Usage.InputDetails.CacheWriteTokens
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
