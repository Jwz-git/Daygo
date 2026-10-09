package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
)

func promptBlock(t *testing.T, prompt, tag string) []byte {
	t.Helper()
	_, rest, ok := strings.Cut(prompt, "<"+tag+">\n")
	if !ok {
		t.Fatalf("prompt missing <%s>", tag)
	}
	raw, _, ok := strings.Cut(rest, "\n</"+tag+">")
	if !ok {
		t.Fatalf("prompt missing </%s>", tag)
	}
	return []byte(raw)
}

// Stored DTO names and icon objects are not the model output contract. Context
// should use the same field vocabulary and a valid JSON array, without changing
// the stored metadata itself.
func TestPromptPreviousCardsUseSchemaVocabulary(t *testing.T) {
	cards := []domain.TimelineCard{
		{Start: "9:30 AM", End: "9:45 AM", Category: "Coding", Title: "Anonymous first", DetailedSummary: "Anonymous detail", Metadata: `{"appSites":{"primary":"example.com","secondary":"Browser"},"activityPoints":[{"time":"9:35 AM","description":"anonymous point"}]}`},
		{Start: "9:45 AM", End: "10:00 AM", Category: "Coding", Title: "Anonymous second"},
	}
	prompt := cardsPrompt(base, base.Add(15*time.Minute), cards, nil, []domain.Category{{Name: "Coding"}}, "zh-CN", cardModeOngoing)
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(promptBlock(t, prompt, "previous_cards"), &rows); err != nil {
		t.Fatalf("history is not a JSON array: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("history rows=%d, want 2", len(rows))
	}
	if _, wrong := rows[0]["detailedSummary"]; wrong {
		t.Fatal("history uses DTO detailedSummary instead of detailed_summary")
	}
	var details string
	if err := json.Unmarshal(rows[0]["detailed_summary"], &details); err != nil || details != "Anonymous detail" {
		t.Fatalf("details=%q, err=%v", details, err)
	}
	var sites []string
	if err := json.Unmarshal(rows[0]["appSites"], &sites); err != nil || !reflect.DeepEqual(sites, []string{"example.com", "Browser"}) {
		t.Fatalf("appSites=%v, err=%v; want primary/secondary string array", sites, err)
	}
	var points []cardActivityPoint
	if err := json.Unmarshal(rows[0]["activityPoints"], &points); err != nil || len(points) != 1 || points[0].Description != "anonymous point" {
		t.Fatalf("points=%v, err=%v", points, err)
	}
	if cards[0].Metadata != `{"appSites":{"primary":"example.com","secondary":"Browser"},"activityPoints":[{"time":"9:35 AM","description":"anonymous point"}]}` {
		t.Fatal("prompt changed stored metadata")
	}
}

func TestPromptOutputExamplesMatchSchemas(t *testing.T) {
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	inputs := []storage.AnalysisFrame{{CapturedAt: start}, {CapturedAt: start.Add(30 * time.Second)}, {CapturedAt: start.Add(time.Minute)}}
	transcription := promptBlock(t, transcribePrompt(inputs, "zh-CN"), "output_example")
	if err := ai.ValidateJSON(transcription, transcribeOutput); err != nil {
		t.Fatalf("transcription example does not match schema: %v", err)
	}
	var tr transcribeEnvelope
	if err := json.Unmarshal(transcription, &tr); err != nil || len(tr.Observations) != 1 || tr.Observations[0].FromFrame != 0 || tr.Observations[0].ToFrame != 2 {
		t.Fatalf("transcription example=%s, err=%v; want frame indices 0..2", transcription, err)
	}
	for _, mode := range []cardMode{cardModeFresh, cardModeOngoing, cardModeScoped} {
		prompt := cardsPrompt(start, start.Add(15*time.Minute), nil, nil, []domain.Category{{Name: "Coding"}}, "zh-CN", mode)
		example := promptBlock(t, prompt, "output_example")
		if err := ai.ValidateJSON(example, cardsOutput); err != nil {
			t.Fatalf("mode %d: output example does not match schema: %v", mode, err)
		}
	}
	correction := cardsCorrectionPrompt(`{"cards":[]}`, []string{"anonymous issue"}, cardModeOngoing, start, start.Add(15*time.Minute))
	if err := ai.ValidateJSON(promptBlock(t, correction, "output_example"), cardsOutput); err != nil {
		t.Fatalf("correction example does not match schema: %v", err)
	}
}

// Brevity is shared by initial generation and stateless corrections. It must
// not encourage dropping JSON fields, evidence coverage or activity changes.
func TestCardPromptsShareLightBrevityGuidance(t *testing.T) {
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	for _, mode := range []cardMode{cardModeFresh, cardModeOngoing, cardModeScoped} {
		prompts := map[string]string{
			"generation": cardsPrompt(start, start.Add(15*time.Minute), nil, nil,
				[]domain.Category{{Name: "Coding"}}, "zh-CN", mode),
			"correction": cardsCorrectionPrompt(`{"cards":[]}`, []string{"anonymous issue"},
				mode, start, start.Add(15*time.Minute)),
		}
		for name, prompt := range prompts {
			t.Run(fmt.Sprintf("mode%d/%s", mode, name), func(t *testing.T) {
				guidance := string(promptBlock(t, prompt, "output_brevity"))
				for _, requirement := range []string{
					"avoid repeating the same details across fields",
					"Combine repetitive actions into short chronological lines",
					"each activityPoint description to one short sentence",
					"Keep useful specifics; add detail when it helps recall the activity",
					"Preserve evidence-supported time coverage and meaningful activity changes",
					"brevity never overrides required JSON fields or segmentation rules",
				} {
					if !strings.Contains(guidance, requirement) {
						t.Errorf("brevity guidance missing %q", requirement)
					}
				}
				if strings.ContainsAny(guidance, "0123456789") {
					t.Fatal("light guidance adds a numeric content limit")
				}
				if strings.Contains(prompt, "Every app, every switch, every action.") {
					t.Fatal("prompt still demands an exhaustive replay of minor actions")
				}
			})
		}
	}
}

func TestPromptFreshHistoryRemainsContextOnly(t *testing.T) {
	history := []domain.TimelineCard{{Start: "9:00 AM", End: "9:15 AM", Category: "Coding", Title: "Anonymous history"}}
	prompt := cardsPrompt(base, base.Add(15*time.Minute), history, nil, nil, "", cardModeFresh)
	if strings.Contains(prompt, "Return cards covering all the time represented by the supplied previous cards") {
		t.Fatal("fresh prompt contradicts leaving unrelated history untouched")
	}
	if !strings.Contains(prompt, "Previous cards are context only; do not reproduce or absorb them in fresh segment mode.") {
		t.Fatal("fresh prompt does not explicitly exclude historical output")
	}
}

func TestPromptOngoingCoverageUsesRewriteSpan(t *testing.T) {
	prompt := cardsPrompt(base, base.Add(15*time.Minute), nil, nil, nil, "", cardModeOngoing)
	if strings.Contains(prompt, "Every card must overlap the current window") {
		t.Fatal("ongoing prompt rejects earlier cards required to cover a merged rewrite")
	}
	if !strings.Contains(prompt, "Every card must overlap the chosen rewrite span") {
		t.Fatal("ongoing prompt does not identify the owned rewrite span")
	}
	if !strings.Contains(prompt, "Do not absorb a predecessor of a different category merely to reach 15 minutes") {
		t.Fatal("prompt lets the 15-minute floor override the pre-window category gate")
	}
}

func TestPromptOngoingBoundariesApplyToFirstAndLastCards(t *testing.T) {
	prompt := cardsPrompt(base, base.Add(30*time.Minute), nil, nil, nil, "", cardModeOngoing)
	if strings.Contains(prompt, "Without a merge, start is the window start and end is the window end") {
		t.Fatal("ongoing prompt assigns the full window to every card instead of its outer boundaries")
	}
	if !strings.Contains(prompt, "the first card starts at the owned rewrite start and the last card ends at the window end") {
		t.Fatal("ongoing prompt does not distinguish outer boundaries from internal activity splits")
	}
}

// Correction calls are stateless and must repeat the interface-language rule;
// copying the previous JSON alone does not carry the instruction.
func TestPipelineCorrectionRetainsOutputLanguage(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":4,"observation":"Anonymous activity","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"9:59 AM","category":"Coding","subcategory":"","title":"Anonymous invalid card","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
	})
	h.service.cfg.Language = func(context.Context) string { return "ja" }
	h.commitFrames(t, base, 31, 30*time.Second, func(int) *int { return nil })
	h.service.tick(context.Background())
	corrections := 0
	for _, req := range h.provider.calls {
		if req.Purpose == ai.PurposeCards && strings.Contains(req.Parts[0].Text(), "<previous_json>") {
			corrections++
			if !strings.Contains(req.Parts[0].Text(), "Write title, summary, detailed_summary and activityPoint descriptions in ja.") {
				t.Error("correction request dropped the selected language")
			}
		}
	}
	if corrections != 2 {
		t.Fatalf("corrections=%d, want 2", corrections)
	}
}

func TestScopedCorrectionRetainsOutputLanguage(t *testing.T) {
	h, _, target, _ := regenerateHarness(t)
	h.service.cfg.Language = func(context.Context) string { return "ja" }
	setCardsResponse(t, h, `{"cards":[{"start":"9:30 AM","end":"10:30 AM","category":"Coding","subcategory":"","title":"Anonymous invalid card","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`)
	if err := h.service.RegenerateCard(context.Background(), target); err == nil {
		t.Fatal("fixture expected a validation failure for persistent overrun")
	}
	corrections := 0
	for _, req := range h.provider.calls {
		if req.Purpose == ai.PurposeCards && strings.Contains(req.Parts[0].Text(), "<previous_json>") {
			corrections++
			if !strings.Contains(req.Parts[0].Text(), "Write title, summary, detailed_summary and activityPoint descriptions in ja.") {
				t.Error("scoped correction request dropped the selected language")
			}
		}
	}
	if corrections != 2 {
		t.Fatalf("corrections=%d, want 2", corrections)
	}
}
