package analysis

import (
	"fmt"
	"time"

	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

/*
 * Card validation, ported from Dayflow's output validator: the model's cards
 * must cover the rewrite span with chronological, non-overlapping cards between
 * 15 and 60 minutes long (2026-09-21 decision). The floor is what keeps a
 * window from being diced into five-minute cards: a short activity is merged
 * into its neighbour instead of ending a card early, and reaching the floor
 * outranks keeping it separate — across a category boundary too, whenever the
 * minutes it needs belong to a differently categorized neighbour (docs/04
 * §4.3.1). The card carrying the rewrite's end is exempt: the supplied evidence
 * stops there, and the next sliding-window pass owns whatever follows it. A
 * failed check produces a human-readable issue for the correction prompt, and
 * the generation loop retries (up to three attempts) before giving up.
 */

const (
	// minCardDuration applies to every card except the last one of the rewrite.
	minCardDuration = 15 * time.Minute
	maxCardDuration = 60 * time.Minute
	// Clock strings carry minute precision, so boundary checks tolerate a
	// minute of rounding slack.
	boundarySlack = time.Minute
)

type cardSpan struct {
	Start time.Time
	End   time.Time
	Title string
}

// resolveCardSpans derives each shell's clock strings into an ordered span and
// returns one human-readable issue per shell that cannot become a valid span
// (unparseable clock, or an end at or before its start). Surfacing these as
// issues — rather than silently dropping them — lets the correction loop retry
// and keeps a degenerate card (e.g. 4:30pm~4:29pm) from reaching storage, where
// it would otherwise persist as a ~24h cross-midnight span (docs/03 §3.5).
func resolveCardSpans(shells []domain.CardShell, windowStart, windowEnd time.Time, loc *time.Location) ([]cardSpan, []string) {
	anchor := windowStart.Add(windowEnd.Sub(windowStart) / 2)
	spans := make([]cardSpan, 0, len(shells))
	var issues []string
	for i, shell := range shells {
		name := fmt.Sprintf("card %d (%s)", i+1, shell.Title)
		start, err := timeutil.ResolveClock(shell.Start, anchor, loc)
		if err != nil {
			issues = append(issues, fmt.Sprintf("%s has an unparseable start time %q", name, shell.Start))
			continue
		}
		end, err := timeutil.ResolveClock(shell.End, anchor, loc)
		if err != nil {
			issues = append(issues, fmt.Sprintf("%s has an unparseable end time %q", name, shell.End))
			continue
		}
		// Cross-midnight clock strings resolve onto the next day already; anything
		// still inverted is unorderable noise the model must correct.
		if !end.After(start) {
			issues = append(issues, fmt.Sprintf(
				"%s ends at %s, at or before its start %s; a card must end after it starts",
				name, end.Format("3:04 PM"), start.Format("3:04 PM")))
			continue
		}
		spans = append(spans, cardSpan{Start: start, End: end, Title: shell.Title})
	}
	return spans, issues
}

// validateCards checks the resolved spans against the Dayflow rules and
// returns one human-readable issue per violation (empty means valid).
func validateCards(spans []cardSpan, rewriteStart, batchEnd time.Time, requiresSingleCard bool) []string {
	var issues []string

	if requiresSingleCard {
		if len(spans) != 1 {
			issues = append(issues, fmt.Sprintf(
				"fresh segment mode requires exactly one card, got %d", len(spans)))
		}
	} else if len(spans) == 0 {
		issues = append(issues, "no cards returned")
		return issues
	}

	for i, span := range spans {
		duration := span.End.Sub(span.Start)
		name := fmt.Sprintf("card %d (%s)", i+1, span.Title)
		if duration > maxCardDuration {
			issues = append(issues, fmt.Sprintf(
				"%s is %s long; every card must be at most 60 minutes",
				name, duration))
		}
		if duration < minCardDuration && i < len(spans)-1 {
			issues = append(issues, fmt.Sprintf(
				"%s is only %s long; merge it into a neighboring card to reach 15 minutes. Only the last card of the window may be shorter",
				name, duration))
		}
	}

	if requiresSingleCard {
		return issues
	}

	// Ordering and overlaps.
	for i := 1; i < len(spans); i++ {
		if spans[i].Start.Before(spans[i-1].Start) {
			issues = append(issues, fmt.Sprintf(
				"card %d (%s) starts before card %d; cards must be chronological", i+1, spans[i].Title, i))
		}
		if spans[i].Start.Before(spans[i-1].End.Add(-boundarySlack)) {
			issues = append(issues, fmt.Sprintf(
				"card %d (%s) overlaps card %d; cards must not overlap", i+1, spans[i].Title, i))
		}
	}

	// Coverage: the first card reaches the span start, the last reaches the
	// window end, and no consecutive pair leaves a real gap behind.
	if len(spans) > 0 {
		if spans[0].Start.After(rewriteStart.Add(boundarySlack)) {
			issues = append(issues, fmt.Sprintf(
				"the cards start at %s but must cover the span from %s; close the uncovered boundary by extending the first card",
				spans[0].Start.Format("3:04 PM"), rewriteStart.Format("3:04 PM")))
		}
		if spans[len(spans)-1].End.Before(batchEnd.Add(-boundarySlack)) {
			issues = append(issues, fmt.Sprintf(
				"the cards end at %s but must reach the window end at %s; close the uncovered boundary by extending the last card",
				spans[len(spans)-1].End.Format("3:04 PM"), batchEnd.Format("3:04 PM")))
		}
		for i := 1; i < len(spans); i++ {
			gap := spans[i].Start.Sub(spans[i-1].End)
			if gap > boundarySlack {
				issues = append(issues, fmt.Sprintf(
					"a %s gap sits between card %d and card %d; if the inputs have a real gap there, extend the neighboring cards to meet it instead",
					gap, i, i+1))
			}
		}
	}

	return issues
}

/*
 * validateScopedCards is validateCards plus the rule that makes a single-card
 * rewrite safe: neither outer boundary may move. validateCards only asks the
 * cards to *reach* the window's ends — a card that runs past the end still
 * passes there, and the rewrite would then overlap the neighbour the user did
 * not ask to touch (storage refuses a rewrite that cannot own a whole victim
 * card, so the whole regeneration would fail instead of the card's own).
 */
func validateScopedCards(spans []cardSpan, windowStart, windowEnd time.Time) []string {
	issues := validateCards(spans, windowStart, windowEnd, false)
	if len(spans) == 0 {
		return issues
	}
	if spans[0].Start.Before(windowStart.Add(-boundarySlack)) {
		issues = append(issues, fmt.Sprintf(
			"the cards start at %s, before the window start at %s; the card before this window is not part of this rewrite",
			spans[0].Start.Format("3:04 PM"), windowStart.Format("3:04 PM")))
	}
	if last := spans[len(spans)-1]; last.End.After(windowEnd.Add(boundarySlack)) {
		issues = append(issues, fmt.Sprintf(
			"the cards end at %s, past the window end at %s; the card after this window is not part of this rewrite",
			last.End.Format("3:04 PM"), windowEnd.Format("3:04 PM")))
	}
	return issues
}
