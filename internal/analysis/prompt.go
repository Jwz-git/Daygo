package analysis

import (
	"encoding/json"
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
	b.WriteString("Return only a json object matching the requested schema; do not include markdown.\n")
	if language != "" {
		fmt.Fprintf(&b, "Write observations in %s.\n", language)
	}
	return b.String()
}

// cardsPrompt renders the sliding-window context: existing cards around the
// batch (what the model may continue or merge with), this batch's fresh
// observations, the category list with details, and the output rules.
//
// Card granularity follows the Dayflow model: each batch window yields ONE
// card covering the whole window, with the per-observation time points carried
// on the card as activityPoints; similar activity in adjacent cards merges
// into a single card spanning both windows.
func cardsPrompt(batchStart, batchEnd time.Time,
	existing []domain.TimelineCard, obs []storage.Observation,
	categories []domain.Category, language string) string {

	var b strings.Builder
	b.WriteString("You are generating the activity card for one time window of a time-tracking app. ")
	b.WriteString("You receive observations of screen activity and previously generated cards nearby. ")
	b.WriteString("Emit exactly ONE card for the current window. Compare the current observations with ")
	b.WriteString("the directly preceding card. MERGE when both describe the same ongoing task or tightly ")
	b.WriteString("related steps toward the same concrete outcome, even when the wording, application, or ")
	b.WriteString("subcategory changes slightly. Repeated debugging, implementation, review, and testing of ")
	b.WriteString("the same feature are one activity. Do not merge merely because the category is the same, ")
	b.WriteString("and do not merge across a meaningful idle gap or a clear change of goal. When merging, emit ")
	b.WriteString("one card whose start is the preceding card's start, whose activityPoints include all earlier ")
	b.WriteString("points, and whose title and summaries describe the whole combined activity.\n\n")

	fmt.Fprintf(&b, "Current window: %s to %s.\n\n",
		formatFrameClock(batchStart), formatFrameClock(batchEnd))

	b.WriteString("Nearby existing cards (do not re-emit these; merge into them when the activity is the same):\n")
	if len(existing) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, c := range existing {
		fmt.Fprintf(&b, "  %s – %s  %s / %s: %s\n",
			c.Start, c.End, c.Category, c.Subcategory, c.Title)
		if c.Summary != "" {
			fmt.Fprintf(&b, "    summary: %s\n", c.Summary)
		}
		if c.DetailedSummary != "" {
			fmt.Fprintf(&b, "    detailed_summary:\n%s\n", indentLines(c.DetailedSummary, "      "))
		}
		for _, p := range activityPointsOfMetadata(c.Metadata) {
			fmt.Fprintf(&b, "    %s  %s\n", p.Time, p.Description)
		}
	}

	b.WriteString("\nFresh observations for the current window:\n")
	if len(obs) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, o := range obs {
		apps := appsOfMetadata(o.Metadata)
		if len(apps) > 0 {
			fmt.Fprintf(&b, "  %s – %s [apps: %s]: %s\n",
				formatFrameClock(o.Start), formatFrameClock(o.End), strings.Join(apps, ", "), o.Observation)
		} else {
			fmt.Fprintf(&b, "  %s – %s: %s\n",
				formatFrameClock(o.Start), formatFrameClock(o.End), o.Observation)
		}
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
	b.WriteString("- Emit exactly one card per call; it covers the current window or, when merging, ")
	b.WriteString("the union of the window and the merged nearby card.\n")
	b.WriteString("- start and end are clock strings like \"10:21 AM\" or \"3:05 PM\". Without a merge, ")
	b.WriteString("start is the window start and end is the window end; with a merge, use the merged ")
	b.WriteString("card's start and this window's end.\n")
	b.WriteString("- Every card must overlap the current window; do not emit cards fully outside it.\n")
	b.WriteString("- end after start; if an activity crosses midnight, end may be earlier than start.\n")
	b.WriteString("- activityPoints lists the concrete time points of the window: one entry per ")
	b.WriteString("observation, time formatted like \"10:21 AM\" and inside the window; when merging, ")
	b.WriteString("include the merged card's earlier points too, in chronological order.\n")
	b.WriteString("- subcategory, detailed_summary, appSites and distractions may be empty; never omit keys.\n")
	b.WriteString("- summary is one sentence naming the apps/sites and the overall activity; keep it ")
	b.WriteString("under 120 characters.\n")
	b.WriteString("- detailed_summary is a chronological log, one paragraph per distinct phase of the ")
	b.WriteString("activity, in the form \"h:mm PM–h:mm PM: what happened\" (times as in the observations; ")
	b.WriteString("the hyphen between times is an en dash). Each paragraph covers a contiguous stretch of ")
	b.WriteString("activity and states concrete outcomes — commands sent, values confirmed, files or ")
	b.WriteString("sections touched — not restatements of the summary.\n")
	b.WriteString("- Keep detailed_summary bounded: at most 8 paragraphs and 1200 characters total. ")
	b.WriteString("When merging, reuse the merged card's paragraphs as the base; extend the last paragraph ")
	b.WriteString("whose time range and activity the new window continues, and only add a new paragraph ")
	b.WriteString("for a genuinely new phase. Drop or compress the oldest, least important paragraphs to ")
	b.WriteString("stay within the limits — recent detail matters more than old detail.\n")
	b.WriteString("- distractions lists applications or sites that look unrelated to the main activity.\n")
	b.WriteString("- Return only a json object matching the requested schema; do not include markdown.\n")
	if language != "" {
		fmt.Fprintf(&b, "- Write title, summary, detailed_summary and activityPoint descriptions in %s.\n", language)
	}
	return b.String()
}

// formatFrameClock renders an instant for prompts. It uses the local zone;
// the card shell clocks are re-derived by ResolveClock against the batch
// anchor on insert, so this rendering is for the model's eyes only.
func formatFrameClock(t time.Time) string {
	return t.Format("3:04 PM")
}

// indentLines pads every line of a multi-line string so a merged card's
// detailed_summary stays aligned inside the "Nearby existing cards" block.
func indentLines(text, padding string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = padding + line
	}
	return strings.Join(lines, "\n")
}

// appsOfMetadata extracts the apps list the transcription stage stored in an
// observation's metadata JSON, so the card prompt can carry it forward as
// structured context instead of asking the model to re-extract it from prose.
func appsOfMetadata(raw string) []string {
	if raw == "" {
		return nil
	}
	var meta struct {
		Apps []string `json:"apps"`
	}
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil
	}
	return meta.Apps
}

// activityPointOfMetadata mirrors the shape generateCards stores in card
// metadata (internal/analysis side); it exists so the prompt can re-render a
// previously merged card's time points for the model.
type cardActivityPoint struct {
	Time        string `json:"time"`
	Description string `json:"description"`
}

func activityPointsOfMetadata(raw string) []cardActivityPoint {
	if raw == "" {
		return nil
	}
	var meta struct {
		ActivityPoints []cardActivityPoint `json:"activityPoints"`
	}
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil
	}
	return meta.ActivityPoints
}
