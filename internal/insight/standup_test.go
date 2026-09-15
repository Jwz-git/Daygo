package insight

import (
	"strings"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/domain"
)

func TestParseStandupOutputRoundTrip(t *testing.T) {
	text := `{"highlights_title":"完成事项","highlights":["a","b"],"tasks_title":"下一步","tasks":[],"blockers_title":"限制","blockers_body":"x"}`
	envelope, err := ParseStandupOutput(text)
	if err != nil {
		t.Fatalf("ParseStandupOutput: %v", err)
	}
	if envelope.HighlightsTitle != "完成事项" || len(envelope.Highlights) != 2 {
		t.Fatalf("envelope = %+v", envelope)
	}
}

func TestParseStandupOutputRejectsInvalid(t *testing.T) {
	if _, err := ParseStandupOutput("not json"); err == nil {
		t.Fatal("non-JSON accepted")
	}
	// Missing a required key is a schema violation, not a zero value.
	if _, err := ParseStandupOutput(`{"highlights_title":"x","highlights":[],"tasks_title":"x","tasks":[]}`); err == nil {
		t.Fatal("output missing blockers fields accepted")
	}
	// A fenced response still parses: extractJSON strips the fence.
	fenced := "```json\n" + `{"highlights_title":"x","highlights":[],"tasks_title":"x","tasks":[],"blockers_title":"x","blockers_body":""}` + "\n```"
	if _, err := ParseStandupOutput(fenced); err != nil {
		t.Fatalf("fenced output rejected: %v", err)
	}
}

func TestStandupPromptCarriesActivityAndLanguage(t *testing.T) {
	cards := []domain.TimelineCard{{
		Start:         "10:00 AM",
		End:           "11:00 AM",
		Category:      "Coding",
		Title:         "Wrote the standup prompt",
		Summary:       "Implemented and tested",
		StartTs:       time.Now().Unix(),
		CreatedAtUnix: 0,
	}}
	prompt := StandupPrompt("2026-09-09", cards, "zh-CN")
	for _, want := range []string{"2026-09-09", "Coding", "Wrote the standup prompt", "zh-CN"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q:\n%s", want, prompt)
		}
	}
	// System cards never reach the prompt — that filter lives in the caller —
	// but the prompt must not add idle instructions of its own.
	if strings.Contains(prompt, "Idle") {
		t.Errorf("prompt mentions Idle: %s", prompt)
	}
}
