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
	b.WriteString("You are the person whose activity log this is, writing a quick end-of-day recap for yourself.\n")
	b.WriteString("Your future self doesn't need a diary. You need the 3-5 things that actually moved the needle today so you can look back and know what happened.\n")
	fmt.Fprintf(&b, "The standup covers the calendar day %s. You receive the day's activity cards, ", standupDay)
	b.WriteString("each with its time range, category, title and summary.\n\n")
	b.WriteString("Read the log, find the real accomplishments, and write them up the way you'd tell a friend: \"here's what I actually got done today.\"\n\n")
	b.WriteString("Activity cards:\n")
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
	b.WriteString("\nSelection rules:\n")
	b.WriteString("- Put 0 to 5 items in \"highlights\" based on evidence quality.\n")
	b.WriteString("- Do NOT pad to reach 5. If only two things were genuinely meaningful, return two.\n")
	b.WriteString("- If nothing high-confidence exists, return an empty \"highlights\" array.\n\n")

	b.WriteString("What counts as an accomplishment:\n")
	b.WriteString("An accomplishment is something that has a clear before and after. You finished it, decided it, figured it out, or made something real. Anything where the state of the world changed because of what you did.\n")
	b.WriteString("Examples across roles:\n")
	b.WriteString("- A founder closed a conversation, sent a launch, locked in a positioning decision.\n")
	b.WriteString("- A student finished a problem set, nailed down a thesis argument, submitted an application.\n")
	b.WriteString("- A designer shipped a comp, got approval on a flow, resolved a UX question with evidence.\n")
	b.WriteString("- An engineer fixed a bug, landed a feature, unblocked a dependency.\n")
	b.WriteString("Not accomplishments: browsing, reading without a takeaway, meetings that ended without a decision, half-started tasks with no checkpoint.\n\n")

	b.WriteString("Writing rules:\n")
	b.WriteString("- Each item: one sentence, 8-20 words max (under 120 characters), without a leading dash.\n")
	b.WriteString("- Lead with what changed or what you decided, not the process of getting there.\n")
	b.WriteString("- Write like a real person. Plain, direct, no filler.\n")
	b.WriteString("- Banned words: leverage, surface, actionable, facilitate, optimize (unless literally about an optimizer), deep-dive, synergy, align (unless about visual alignment).\n")
	b.WriteString("- If something sounds like a consultant or a report generator wrote it, rewrite it in your own words.\n")
	b.WriteString("- Use only evidence from the log. Do not invent or assume details.\n")
	b.WriteString("- Name concrete things: the pricing page, the midterm essay, the onboarding flow, the partner deal. Not vague categories.\n")
	b.WriteString("- Include a number when it adds real signal (a metric, count, %, dollar amount, word count). If the log has a useful number, use it. Don't force one in.\n\n")

	b.WriteString("What to skip:\n")
	b.WriteString("- Browsing, entertainment, social media scrolling, side distractions.\n")
	b.WriteString("- Low-signal process noise: \"build succeeded,\" \"synced files,\" \"opened app.\"\n")
	b.WriteString("- Tool and workflow internals your future self won't care about: file names, class names, git/PR activity, IDE details, batch IDs.\n")
	b.WriteString("- Don't mention AI tools by name (Claude, ChatGPT, Cursor, Copilot) unless the work was explicitly about that tool. The accomplishment is the output, not the tool.\n")
	b.WriteString("- No em dashes. No hype. No self-praise.\n\n")

	b.WriteString("Next steps / tasks section:\n")
	b.WriteString("- Include \"tasks\" (0 to 3 items) only when the log shows a specific task that was clearly started but unfinished, or a concrete next step explicitly discussed or planned during the day.\n")
	b.WriteString("- Do not speculate. If nothing in the log points to a specific carryover task, return an empty array.\n")
	b.WriteString("- The bar: could you point to a specific moment in the log where this next step was set up? If not, leave it out.\n\n")

	b.WriteString("Blockers section:\n")
	b.WriteString("- \"blockers_body\" states current constraints or friction visible in the activity (interruptions, repeated debugging, context switching); write an empty string when none are visible.\n\n")

	b.WriteString("Section titles:\n")
	b.WriteString("- The three *_title fields (highlights_title, tasks_title, blockers_title) are short section headings in the output language.\n\n")

	b.WriteString("Examples:\n")
	b.WriteString("Good bullets:\n")
	b.WriteString("- \"Fixed the webhook retry bug that was dropping ~12% of partner callbacks.\"\n")
	b.WriteString("- \"Finished the pricing page FAQ and got sign-off from Lisa.\"\n")
	b.WriteString("- \"Narrowed the signup drop-off to the email verification step, 41% abandon rate.\"\n")
	b.WriteString("- \"Submitted the constitutional law essay, 2,800 words.\"\n")
	b.WriteString("- \"Locked in the 'automatic work journal' positioning after testing five alternatives.\"\n")
	b.WriteString("- \"Got verbal yes from the Acme partnership, sending the agreement tomorrow.\"\n")
	b.WriteString("- \"Finalized the onboarding flow redesign, down from 7 screens to 4.\"\n\n")
	b.WriteString("Bad bullets and why:\n")
	b.WriteString("- \"Updated AuthService.swift and pushed three commits.\" -> Implementation details nobody needs.\n")
	b.WriteString("- \"Surfaced conversion leakage insights and drafted actionable recommendations.\" -> Consultant-speak. What did you actually find?\n")
	b.WriteString("- \"Spent a focused session analyzing churn patterns to derive strategic retention insights.\" -> Describes the process, not the result. What did the analysis show?\n")
	b.WriteString("- \"Did some research on competitors.\" -> Too vague. What did you learn? What did you decide?\n")
	b.WriteString("- \"Had a productive brainstorm with the team.\" -> What came out of it?\n\n")

	b.WriteString("Output format:\n")
	b.WriteString("Return only a json object matching the requested schema; do not include markdown.\n")
	if language != "" {
		fmt.Fprintf(&b, "Write all output in %s.\n", language)
	}
	return b.String()
}
