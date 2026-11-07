package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
)

var modelID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

type Client struct {
	http     *http.Client
	endpoint string
	secret   string
}

func NewClient(client *http.Client, endpoint, model, secret string) (*Client, error) {
	base, err := ai.GeminiBaseURL(endpoint)
	if err != nil {
		return nil, err
	}
	model = strings.TrimPrefix(model, "models/")
	if !modelID.MatchString(model) {
		return nil, ai.NewError(ai.ErrorInvalidRequest, "invalid Gemini model ID", 0, nil)
	}
	if client == nil {
		client = http.DefaultClient
	}
	// Never forward the API key or replay screen content to a redirect target.
	transport := *client
	transport.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{http: &transport, endpoint: base + "models/" + model + ":generateContent", secret: secret}, nil
}

type mediaBlob struct {
	MIMEType ai.MediaType `json:"mimeType"`
	Data     []byte       `json:"data"`
}

type contentPart struct {
	Text       string     `json:"text,omitempty"`
	InlineData *mediaBlob `json:"inlineData,omitempty"`
	Thought    bool       `json:"thought,omitempty"`
}

type content struct {
	Parts []contentPart `json:"parts"`
}

type generationConfig struct {
	MaxOutputTokens    int             `json:"maxOutputTokens,omitempty"`
	ResponseMIMEType   string          `json:"responseMimeType,omitempty"`
	ResponseJSONSchema json.RawMessage `json:"responseJsonSchema,omitempty"`
}

func (c *Client) Generate(ctx context.Context, request ai.Request) (ai.Result, error) {
	if err := request.Validate(); err != nil {
		return ai.Result{}, err
	}
	parts := make([]contentPart, 0, len(request.Parts))
	for _, part := range request.Parts {
		switch part.Kind() {
		case ai.PartText:
			parts = append(parts, contentPart{Text: part.Text()})
		case ai.PartImage, ai.PartVideo:
			parts = append(parts, contentPart{InlineData: &mediaBlob{MIMEType: part.MediaType(), Data: part.Bytes()}})
		}
	}
	config := generationConfig{MaxOutputTokens: request.MaxOutputTokens}
	if request.Output != nil {
		config.ResponseMIMEType = "application/json"
		config.ResponseJSONSchema = request.Output.Schema
	}
	payload := struct {
		Contents []content        `json:"contents"`
		Config   generationConfig `json:"generationConfig"`
	}{Contents: []content{{Parts: parts}}, Config: config}
	body, err := json.Marshal(payload)
	if err != nil {
		return ai.Result{}, ai.NewError(ai.ErrorInvalidRequest, "cannot encode Gemini request", 0, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return ai.Result{}, ai.NewError(ai.ErrorInvalidRequest, "cannot create Gemini request", 0, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.secret)
	res, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ai.Result{}, ctx.Err()
		}
		return ai.Result{}, ai.NewError(ai.ErrorUnavailable, "Gemini request failed", 0, err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return ai.Result{}, statusError(res)
	}

	const maxResponseBytes = 4 << 20
	body, err = io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return ai.Result{}, ctx.Err()
		}
		return ai.Result{}, ai.NewError(ai.ErrorUnavailable, "cannot read Gemini response", res.StatusCode, err)
	}
	if len(body) > maxResponseBytes {
		return ai.Result{}, ai.NewError(ai.ErrorInvalidOutput, "Gemini response is too large", res.StatusCode, nil)
	}
	return parseResponse(body, request.Output)
}

func statusError(res *http.Response) error {
	kind := ai.ErrorInvalidRequest
	switch {
	case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
		kind = ai.ErrorAuthentication
	case res.StatusCode == http.StatusRequestTimeout || res.StatusCode == http.StatusGatewayTimeout:
		kind = ai.ErrorTimeout
	case res.StatusCode == http.StatusTooManyRequests:
		kind = ai.ErrorRateLimited
	case res.StatusCode >= 500:
		kind = ai.ErrorUnavailable
	}
	err := ai.NewError(kind, "Gemini rejected the request with HTTP "+strconv.Itoa(res.StatusCode), res.StatusCode, nil)
	retry := res.Header.Get("Retry-After")
	if seconds, parseErr := strconv.ParseInt(retry, 10, 32); parseErr == nil && seconds > 0 {
		err.RetryAfter = time.Duration(seconds) * time.Second
	} else if at, parseErr := http.ParseTime(retry); parseErr == nil {
		err.RetryAfter = max(0, time.Until(at))
	}
	return err
}

func parseResponse(body []byte, schema *ai.OutputSchema) (ai.Result, error) {
	var response struct {
		ModelVersion string
		Candidates   []struct {
			FinishReason string
			Content      content
		}
		UsageMetadata struct {
			PromptTokenCount        *int64
			CandidatesTokenCount    *int64
			CachedContentTokenCount *int64
		}
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return ai.Result{}, ai.NewError(ai.ErrorInvalidOutput, "Gemini returned invalid JSON", 0, err)
	}
	result := ai.Result{Usage: ai.Usage{
		InputTokens:     response.UsageMetadata.PromptTokenCount,
		OutputTokens:    response.UsageMetadata.CandidatesTokenCount,
		CacheReadTokens: response.UsageMetadata.CachedContentTokenCount,
	}}
	if len(response.Candidates) == 0 {
		return result, ai.NewError(ai.ErrorInvalidOutput, "Gemini returned no candidate", 0, nil)
	}
	candidate := response.Candidates[0]
	switch candidate.FinishReason {
	case "STOP":
	case "MAX_TOKENS":
		return result, ai.NewError(ai.ErrorInvalidOutput, "Gemini reached its output limit", 0, ai.ErrTokenLimit)
	default:
		return result, ai.NewError(ai.ErrorInvalidOutput, "Gemini generation did not complete", 0, nil)
	}
	var text strings.Builder
	for _, part := range candidate.Content.Parts {
		if !part.Thought {
			text.WriteString(part.Text)
		}
	}
	if strings.TrimSpace(text.String()) == "" {
		return result, ai.NewError(ai.ErrorInvalidOutput, "Gemini returned no text", 0, nil)
	}
	if schema != nil {
		structured, err := ai.ParseStructuredOutput(text.String(), *schema)
		if err != nil {
			return result, err
		}
		result.JSON = structured
	}
	result.Text = text.String()
	result.Model = response.ModelVersion
	return result, nil
}
