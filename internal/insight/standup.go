package insight

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/domain"
)

// StandupOutput is the daily-standup generation contract. Titles are part of
// the model output so the saved recap carries section headings in the output
// language; the frontend falls back to its own i18n only when a title is
// empty.
var StandupOutput = ai.OutputSchema{Name: "daygo_standup", Schema: json.RawMessage(`{
	"type": "object",
	"properties": {
		"highlights_title": {"type": "string"},
		"highlights": {"type": "array", "items": {"type": "string"}},
		"tasks_title": {"type": "string"},
		"tasks": {"type": "array", "items": {"type": "string"}},
		"blockers_title": {"type": "string"},
		"blockers_body": {"type": "string"}
	},
	"required": ["highlights_title", "highlights", "tasks_title", "tasks", "blockers_title", "blockers_body"],
	"additionalProperties": false
}`), Strict: true}

// StandupEnvelope is the decoded model output of one recap generation.
type StandupEnvelope struct {
	HighlightsTitle string   `json:"highlights_title"`
	Highlights      []string `json:"highlights"`
	TasksTitle      string   `json:"tasks_title"`
	Tasks           []string `json:"tasks"`
	BlockersTitle   string   `json:"blockers_title"`
	BlockersBody    string   `json:"blockers_body"`
}

// ParseStandupOutput validates and decodes the model's recap response.
func ParseStandupOutput(text string) (StandupEnvelope, error) {
	raw, err := ai.ParseStructuredOutput(text, StandupOutput)
	if err != nil {
		return StandupEnvelope{}, err
	}
	var envelope StandupEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return StandupEnvelope{}, fmt.Errorf("decode standup: %w", err)
	}
	return envelope, nil
}

// StandupPrompt renders the standup-generation prompt from one calendar day's
// activity cards. Cards are pre-filtered by the caller (system and idle
// categories removed); the prompt only restates what the cards say, so the
// model cannot invent activity that was never recorded.
func StandupPrompt(standupDay string, cards []domain.TimelineCard, language string) string {
	var b strings.Builder
	b.WriteString("You are writing the daily standup recap for one day of a time-tracking app. ")
	fmt.Fprintf(&b, "The standup covers the calendar day %s. You receive the day's activity cards, ", standupDay)
	b.WriteString("each with its time range, category, title and summary. Produce the three standup ")
	b.WriteString("sections from this activity alone.\n\nActivity cards:\n")
	for _, c := range cards {
		fmt.Fprintf(&b, "  %s – %s  %s", c.Start, c.End, c.Category)
		if c.Subcategory != "" {
			fmt.Fprintf(&b, " / %s", c.Subcategory)
		}
		fmt.Fprintf(&b, ": %s\n", c.Title)
		if c.Summary != "" {
			fmt.Fprintf(&b, "    summary: %s\n", c.Summary)
		}
	}
	b.WriteString("\nOutput rules:\n")
	b.WriteString("- highlights lists the completed work of the day: 2 to 5 short bullets, each ")
	b.WriteString("naming a concrete outcome, most significant first. Merge related cards into one bullet.\n")
	b.WriteString("- tasks lists plausible next steps: 2 to 4 short bullets inferred from unfinished ")
	b.WriteString("or follow-on work visible in the cards. Do not invent projects that never appear.\n")
	b.WriteString("- blockers_body states current constraints visible in the activity (interruptions, ")
	b.WriteString("repeated debugging, context switching); write the empty string when none are visible.\n")
	b.WriteString("- The three *_title fields are short section headings in the output language.\n")
	b.WriteString("- Every bullet is one sentence, under 120 characters, without a leading dash.\n")
	b.WriteString("Return only a json object matching the requested schema; do not include markdown.\n")
	if language != "" {
		fmt.Fprintf(&b, "Write all output in %s.\n", language)
	}
	return b.String()
}
