package analysis

import (
	"encoding/json"

	"github.com/Jwz-git/Daygo/internal/ai"
)

// transcribeOutput is the frame-transcription contract. The model references
// frames by index (0-based, in send order) and never touches timestamps: the
// service maps from_frame/to_frame to the group's own frame clocks, so the
// model cannot get Unix arithmetic wrong.
var transcribeOutput = ai.OutputSchema{Name: "daygo_transcribe", Schema: json.RawMessage(`{
	"type": "object",
	"properties": {
		"observations": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"from_frame": {"type": "integer", "minimum": 0},
					"to_frame":   {"type": "integer", "minimum": 0},
					"observation": {"type": "string"},
					"apps": {"type": "array", "items": {"type": "string"}}
				},
				"required": ["from_frame", "to_frame", "observation", "apps"],
				"additionalProperties": false
			}
		}
	},
	"required": ["observations"],
	"additionalProperties": false
}`), Strict: true}

// cardsOutput is the card-generation contract. start/end are clock strings in
// the contract format "h:mm AM/PM" — the same shape timeline_cards stores and
// ResolveClock parses.
var cardsOutput = ai.OutputSchema{Name: "daygo_cards", Schema: json.RawMessage(`{
	"type": "object",
	"properties": {
		"cards": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"start":   {"type": "string"},
					"end":     {"type": "string"},
					"category": {"type": "string"},
					"subcategory": {"type": "string"},
					"title":   {"type": "string"},
					"summary": {"type": "string"},
					"detailed_summary": {"type": "string"},
					"appSites": {"type": "array", "items": {"type": "string"}},
					"distractions": {"type": "array", "items": {"type": "string"}}
				},
				"required": ["start", "end", "category", "subcategory", "title", "summary",
					"detailed_summary", "appSites", "distractions"],
				"additionalProperties": false
			}
		}
	},
	"required": ["cards"],
	"additionalProperties": false
}`), Strict: true}

type transcribeEnvelope struct {
	Observations []struct {
		FromFrame   int      `json:"from_frame"`
		ToFrame     int      `json:"to_frame"`
		Observation string   `json:"observation"`
		Apps        []string `json:"apps"`
	} `json:"observations"`
}

type cardsEnvelope struct {
	Cards []struct {
		Start           string   `json:"start"`
		End             string   `json:"end"`
		Category        string   `json:"category"`
		Subcategory     string   `json:"subcategory"`
		Title           string   `json:"title"`
		Summary         string   `json:"summary"`
		DetailedSummary string   `json:"detailed_summary"`
		AppSites        []string `json:"appSites"`
		Distractions    []string `json:"distractions"`
	} `json:"cards"`
}
