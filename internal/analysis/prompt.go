package analysis

import (
	"fmt"
	"strings"
	"time"

	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// transcribePrompt describes each frame's capture time so the model can tell
// motion from stillness, and asks for observations referenced by frame index.
// The time format matches FormatClock so nothing has to be parsed back.
func transcribePrompt(group []storage.AnalysisFrame, language string) string {
	var b strings.Builder
	b.WriteString("You are transcribing a group of consecutive screen captures from one computer. ")
	b.WriteString("Each image is one screenshot, in order. Describe what the user was doing in this group: ")
	b.WriteString("group frames into one observation per distinct activity, and give from_frame/to_frame ")
	b.WriteString("as 0-based indices into this group's images.\n\n")
	b.WriteString("Frames:\n")
	for i, f := range group {
		fmt.Fprintf(&b, "  frame %d: captured at %s\n", i, formatFrameClock(f.CapturedAt))
	}
	b.WriteString("\nWrite plain, factual one-to-two-sentence observations. List the applications or ")
	b.WriteString("web sites visible. Do not speculate about content you cannot read.\n")
	if language != "" {
		fmt.Fprintf(&b, "Write observations in %s.\n", language)
	}
	return b.String()
}

// cardsPrompt renders the sliding-window context: existing cards around the
// batch (what the model may continue or merge with), this batch's fresh
// observations, the category list with details, and the output rules.
func cardsPrompt(batchStart, batchEnd time.Time,
	existing []domain.TimelineCard, obs []storage.Observation,
	categories []domain.Category, language string) string {

	var b strings.Builder
	b.WriteString("You are generating timeline activity cards for a time-tracking app. ")
	b.WriteString("You receive observations of screen activity and previously generated cards nearby. ")
	b.WriteString("Rewrite the activity cards for the current window, continuing or merging with ")
	b.WriteString("nearby cards when the activity is the same.\n\n")

	fmt.Fprintf(&b, "Current window: %s to %s.\n\n",
		formatFrameClock(batchStart), formatFrameClock(batchEnd))

	b.WriteString("Nearby existing cards (do not emit these again; use them for continuity and merge boundaries):\n")
	if len(existing) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, c := range existing {
		fmt.Fprintf(&b, "  %s – %s  %s / %s: %s\n",
			c.Start, c.End, c.Category, c.Subcategory, c.Title)
	}

	b.WriteString("\nFresh observations for the current window:\n")
	if len(obs) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, o := range obs {
		fmt.Fprintf(&b, "  %s – %s: %s\n",
			formatFrameClock(o.Start), formatFrameClock(o.End), o.Observation)
	}

	b.WriteString("\nCategories (category MUST be one of these names):\n")
	for _, c := range categories {
		if c.Details != "" {
			fmt.Fprintf(&b, "  %s — %s\n", c.Name, c.Details)
		} else {
			fmt.Fprintf(&b, "  %s\n", c.Name)
		}
	}

	b.WriteString("\nOutput rules:\n")
	b.WriteString("- start and end are clock strings like \"10:21 AM\" or \"3:05 PM\", inside the current window.\n")
	b.WriteString("- Every card must overlap the current window; do not re-emit cards fully outside it.\n")
	b.WriteString("- end after start; if an activity crosses midnight, end may be earlier than start.\n")
	b.WriteString("- subcategory, detailed_summary, appSites and distractions may be empty; never omit keys.\n")
	b.WriteString("- distractions lists applications or sites that look unrelated to the main activity.\n")
	if language != "" {
		fmt.Fprintf(&b, "- Write title, summary and detailed_summary in %s.\n", language)
	}
	return b.String()
}

// formatFrameClock renders an instant for prompts. It uses the local zone;
// the card shell clocks are re-derived by ResolveClock against the batch
// anchor on insert, so this rendering is for the model's eyes only.
func formatFrameClock(t time.Time) string {
	return t.Format("3:04 PM")
}
