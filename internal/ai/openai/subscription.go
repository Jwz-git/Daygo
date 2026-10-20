package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/Jwz-git/Daygo/internal/ai"
)

const SubscriptionEndpoint = "https://api.openai.com/v1"

// SubscriptionClient implements the distinct ChatGPT plan Responses contract.
// The token source is called for every attempt, so long-lived chains renew.
type SubscriptionClient struct {
	client          *http.Client
	endpoint, model string
	token           func(context.Context) (string, error)
}

func NewSubscriptionClient(client *http.Client, model string, token func(context.Context) (string, error)) (*SubscriptionClient, error) {
	if model == "" || token == nil {
		return nil, ai.NewError(ai.ErrorInvalidRequest, "subscription requires a model and authorization", 0, nil)
	}
	if client == nil {
		client = http.DefaultClient
	}
	clone := *client
	clone.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &SubscriptionClient{client: &clone, endpoint: SubscriptionEndpoint + "/responses", model: model, token: token}, nil
}
func (c *SubscriptionClient) Generate(ctx context.Context, req ai.Request) (ai.Result, error) {
	if err := req.Validate(); err != nil {
		return ai.Result{}, err
	}
	token, err := c.token(ctx)
	if err != nil {
		return ai.Result{}, err
	}
	if token == "" {
		return ai.Result{}, ai.NewError(ai.ErrorAuthentication, "ChatGPT authorization is required", 0, nil)
	}
	// Reuse image/schema mapping, with plan-specific transport and budget policy.
	req.MaxOutputTokens = 0
	mapper := ResponsesClient{model: c.model}
	body, err := mapper.requestBody(req)
	if err != nil {
		return ai.Result{}, err
	}
	var payload map[string]json.RawMessage
	_ = json.Unmarshal(body, &payload)
	payload["stream"] = json.RawMessage("true")
	payload["store"] = json.RawMessage("false")
	body, _ = json.Marshal(payload)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return ai.Result{}, ai.NewError(ai.ErrorInvalidRequest, "cannot create subscription request", 0, nil)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	response, err := c.client.Do(request)
	if err != nil {
		return ai.Result{}, transportError(ctx, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		return ai.Result{}, subscriptionError(response.StatusCode, raw)
	}
	if !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		return ai.Result{}, ai.NewError(ai.ErrorInvalidOutput, "subscription did not return a stream", 0, nil)
	}
	scanner := bufio.NewScanner(io.LimitReader(response.Body, maxResponseBytes+1))
	scanner.Buffer(make([]byte, 4096), maxResponseBytes)
	var event strings.Builder
	total := 0
	consume := func() (ai.Result, error, bool) {
		if event.Len() == 0 {
			return ai.Result{}, nil, false
		}
		raw := event.String()
		event.Reset()
		var e struct {
			Type     string          `json:"type"`
			Response json.RawMessage `json:"response"`
		}
		if json.Unmarshal([]byte(raw), &e) != nil {
			return ai.Result{}, ai.NewError(ai.ErrorInvalidOutput, "invalid subscription event", 0, nil), true
		}
		switch e.Type {
		case "response.failed", "error":
			return ai.Result{}, subscriptionError(0, []byte(raw)), true
		case "response.completed", "response.incomplete":
			var state struct {
				Status string `json:"status"`
			}
			_ = json.Unmarshal(e.Response, &state)
			if e.Type == "response.completed" && state.Status != "completed" {
				return ai.Result{}, ai.NewError(ai.ErrorInvalidOutput, "subscription response did not complete", 0, nil), true
			}
			safe := strings.ReplaceAll(string(e.Response), token, "[redacted]")
			result, err := parseResponsesResponse([]byte(safe), req.Output)
			if e.Type == "response.incomplete" && err == nil {
				return ai.Result{Usage: result.Usage}, ai.NewError(ai.ErrorInvalidOutput, "subscription response did not complete", 0, nil), true
			}
			return result, err, true
		}
		return ai.Result{}, nil, false
	}
	for scanner.Scan() {
		line := scanner.Text()
		total += len(line) + 1
		if total > maxResponseBytes {
			return ai.Result{}, ai.NewError(ai.ErrorInvalidOutput, "subscription stream is too large", 0, nil)
		}
		if line == "" {
			if result, err, done := consume(); done {
				return result, err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if event.Len() > 0 {
				event.WriteByte('\n')
			}
			event.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := scanner.Err(); err != nil {
		return ai.Result{}, transportError(ctx, err)
	}
	if result, err, done := consume(); done {
		return result, err
	}
	return ai.Result{}, ai.NewError(ai.ErrorInvalidOutput, "subscription stream ended before completion", 0, nil)
}
func subscriptionError(status int, body []byte) error {
	var p struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
		Response struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		} `json:"response"`
	}
	_ = json.Unmarshal(body, &p)
	code := p.Error.Code
	if code == "" {
		code = p.Response.Error.Code
	}
	switch code {
	case "subscription_sharing_usage_limit_exceeded":
		return ai.NewError(ai.ErrorRateLimited, "ChatGPT subscription usage limit reached", status, ai.ErrSubscriptionLimit)
	case "subscription_sharing_user_not_eligible", "chatpass_v2_scope_not_authorized", "chatpass_v2_invalid_authorization_context", "subscription_sharing_invalid_user":
		return ai.NewError(ai.ErrorAuthentication, "ChatGPT subscription permission is unavailable", status, nil)
	case "subscription_sharing_unsupported_capability", "subscription_sharing_route_not_supported":
		return ai.NewError(ai.ErrorUnsupportedFeature, "ChatGPT subscription does not support this request", status, nil)
	case "subscription_sharing_usage_unavailable", "subscription_sharing_user_unavailable":
		return ai.NewError(ai.ErrorUnavailable, "ChatGPT subscription is temporarily unavailable", status, nil)
	}
	if status == 401 || status == 403 {
		return ai.NewError(ai.ErrorAuthentication, "ChatGPT authorization was rejected", status, nil)
	}
	if status == 429 {
		return ai.NewError(ai.ErrorRateLimited, "ChatGPT request was rate limited", status, nil)
	}
	if status >= 500 {
		return ai.NewError(ai.ErrorUnavailable, "ChatGPT subscription is temporarily unavailable", status, nil)
	}
	return ai.NewError(ai.ErrorInvalidRequest, "ChatGPT subscription request failed", status, nil)
}
