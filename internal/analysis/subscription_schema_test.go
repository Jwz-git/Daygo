package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/ai/factory"
	"github.com/Jwz-git/Daygo/internal/ai/openai"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
)

type subscriptionFixtureTransport func(*http.Request) (*http.Response, error)

func (f subscriptionFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Exercise both production schemas over the actual subscription client, with
// anonymous generated image bytes. No external host or real credential is used.
func TestSubscriptionProductionSchemas(t *testing.T) {
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	var pixels bytes.Buffer
	if err := png.Encode(&pixels, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	imagePart, _ := ai.ImagePart(ai.MediaPNG, pixels.Bytes())
	for _, tc := range []struct {
		name, prompt string
		schema       *ai.OutputSchema
		parts        []ai.Part
	}{
		{"transcription", transcribePrompt([]storage.AnalysisFrame{{CapturedAt: start}}, "zh-CN"), &transcribeOutput, []ai.Part{ai.TextPart("anonymous fixture"), imagePart}},
		{"cards", cardsPrompt(start, start.Add(15*time.Minute), nil, nil, []domain.Category{{Name: "Coding"}}, "zh-CN", cardModeFresh), &cardsOutput, []ai.Part{ai.TextPart("anonymous observations")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output := promptBlock(t, tc.prompt, "output_example")
			transport := subscriptionFixtureTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != openai.SubscriptionEndpoint+"/responses" {
					t.Error("unexpected destination")
				}
				var payload struct {
					Store  bool `json:"store"`
					Stream bool `json:"stream"`
					Text   struct {
						Format struct {
							Schema json.RawMessage `json:"schema"`
						} `json:"format"`
					} `json:"text"`
					Input []struct {
						Content []struct {
							Type string `json:"type"`
						} `json:"content"`
					} `json:"input"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				var sent, want any
				if json.Unmarshal(payload.Text.Format.Schema, &sent) != nil || json.Unmarshal(tc.schema.Schema, &want) != nil {
					t.Fatal("schema could not be decoded")
				}
				if payload.Store || !payload.Stream || !reflect.DeepEqual(sent, want) {
					t.Error("production schema or transport changed")
				}
				if tc.name == "transcription" && payload.Input[0].Content[1].Type != "input_image" {
					t.Error("image omitted")
				}
				reply, _ := json.Marshal(map[string]any{"type": "response.completed", "response": map[string]any{"status": "completed", "model": "fixture", "output": []any{map[string]any{"type": "message", "content": []any{map[string]string{"type": "output_text", "text": string(output)}}}}}})
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(fmt.Sprintf("data: %s\n\n", reply)))}, nil
			})
			client, err := factory.NewClient(&http.Client{Transport: transport}, factory.Config{Protocol: ai.ProtocolChatGPT, Endpoint: openai.SubscriptionEndpoint, Model: "fixture", TokenSource: func(context.Context) (string, error) { return "anonymous-token", nil }})
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.Generate(context.Background(), ai.Request{Purpose: ai.PurposeTranscribe, Parts: tc.parts, Output: tc.schema, MaxOutputTokens: 8192})
			if err != nil || len(result.JSON) == 0 {
				t.Fatalf("production schema failed: %v", err)
			}
		})
	}
}
