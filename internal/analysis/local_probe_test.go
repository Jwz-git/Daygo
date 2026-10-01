package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/ai/factory"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/platform/secrets"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type probeTransport struct{}

func (probeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(b))
	var d struct {
		Choices []struct {
			Finish  string `json:"finish_reason"`
			Message struct {
				Content   string `json:"content"`
				Reasoning string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			Input  int `json:"prompt_tokens"`
			Output int `json:"completion_tokens"`
		} `json:"usage"`
	}
	json.Unmarshal(b, &d)
	for _, c := range d.Choices {
		fmt.Printf("HTTP=%d finish=%s content_bytes=%d reasoning_bytes=%d input_tokens=%d output_tokens=%d\n", resp.StatusCode, c.Finish, len(c.Message.Content), len(c.Message.Reasoning), d.Usage.Input, d.Usage.Output)
	}
	return resp, nil
}
func TestLocalAnonymousProbe(t *testing.T) {
	if os.Getenv("DAYGO_ANONYMOUS_PROBE") != "1" {
		t.Skip("explicit probe only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	store, err := storage.OpenReadOnly(ctx, filepath.Join(os.Getenv("APPDATA"), "Daygo", "daygo.sqlite"), time.Local, nil)
	if err != nil {
		t.Fatal("open failed")
	}
	defer store.Close()
	ps, err := store.Providers().List(ctx)
	if err != nil {
		t.Fatal("list failed")
	}
	for _, p := range ps {
		for _, model := range p.Models {
			if model != "qwen3-vl-8b" {
				continue
			}
			key, err := secrets.New().Get(ctx, p.ID)
			if err != nil {
				t.Fatal("secret unavailable")
			}
			cfg := factory.Config{Protocol: ai.Protocol(p.Protocol), Endpoint: p.Endpoint, Model: model, Secret: key}
			if os.Getenv("DAYGO_PROBE_INSTRUCT_DIRECT") == "1" {
				cfg.Endpoint = "http://127.0.0.1:11434/v1"
				cfg.Model = "qwen3-vl:8b-instruct"
				cfg.Secret = ""
			}
			client, err := factory.NewClient(&http.Client{Transport: probeTransport{}}, cfg)
			if err != nil {
				t.Fatal("client failed")
			}
			data, err := os.ReadFile("../ai/assets/connection-test.png")
			if err != nil {
				t.Fatal(err)
			}
			img, _ := ai.ImagePart(ai.MediaPNG, data)
			start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
			end := start.Add(15 * time.Minute)
			req := ai.Request{Purpose: ai.PurposeTranscribe, Parts: []ai.Part{ai.TextPart(transcribePrompt([]storage.AnalysisFrame{{CapturedAt: start}}, "zh-CN")), img}, Output: &transcribeOutput, MaxOutputTokens: 3072}
			began := time.Now()
			result, err := client.Generate(ctx, req)
			if err != nil {
				t.Fatalf("transcribe: %v", err)
			}
			var tr transcribeEnvelope
			if err := json.Unmarshal(result.JSON, &tr); err != nil || len(tr.Observations) == 0 {
				t.Fatal("no valid observations")
			}
			t.Logf("transcription validated: observations=%d elapsed=%s", len(tr.Observations), time.Since(began).Round(time.Millisecond))
			var obs []storage.Observation
			for _, o := range tr.Observations {
				obs = append(obs, storage.Observation{Start: start, End: end, Observation: o.Observation})
			}
			req = ai.Request{Purpose: ai.PurposeCards, Parts: []ai.Part{ai.TextPart(cardsPrompt(start, end, nil, obs, []domain.Category{{Name: "Work"}, {Name: "Personal"}}, "zh-CN", cardModeFresh))}, Output: &cardsOutput, MaxOutputTokens: 4096}
			began = time.Now()
			result, err = client.Generate(ctx, req)
			if err != nil {
				t.Fatalf("cards: %v", err)
			}
			var cr cardsEnvelope
			if err := json.Unmarshal(result.JSON, &cr); err != nil || len(cr.Cards) == 0 {
				t.Fatal("no valid cards")
			}
			for _, card := range cr.Cards {
				if card.Title == "" || card.Summary == "" {
					t.Fatal("empty card content")
				}
				if card.Category != "Work" && card.Category != "Personal" {
					t.Fatal("unknown category")
				}
				from, e1 := timeutil.ResolveClock(card.Start, start.Add(7*time.Minute), time.UTC)
				to, e2 := timeutil.ResolveClock(card.End, start.Add(7*time.Minute), time.UTC)
				if e1 != nil || e2 != nil || !from.Equal(start) || !to.Equal(end) {
					t.Fatal("card does not cover anonymous window")
				}
			}
			t.Logf("cards validated: cards=%d elapsed=%s", len(cr.Cards), time.Since(began).Round(time.Millisecond))
			return
		}
	}
	t.Fatal("configured model not found")
}
