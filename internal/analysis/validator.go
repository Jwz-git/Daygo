package analysis

import (
	"fmt"
	"time"

	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

/*
 * Card validation, ported from Dayflow's output validator: the model's cards
 * must cover the rewrite span with non-overlapping 10-60 minute cards. A
 * failed check produces a human-readable issue for the correction prompt, and
 * the generation loop retries (up to three attempts) before giving up.
 */

const (
	minCardDuration = 10 * time.Minute
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

func resolveCardSpans(shells []domain.CardShell, batch storage.Batch, loc *time.Location) []cardSpan {
	anchor := batch.Start.Add(batch.End.Sub(batch.Start) / 2)
	spans := make([]cardSpan, 0, len(shells))
	for _, shell := range shells {
		start, err := timeutil.ResolveClock(shell.Start, anchor, loc)
		if err != nil {
			continue
		}
		end, err := timeutil.ResolveClock(shell.End, anchor, loc)
		if err != nil {
			continue
		}
		if !end.After(start) {
			// Cross-midnight clock strings resolve to the next day already;
			// anything still inverted is unorderable noise.
			continue
		}
		spans = append(spans, cardSpan{Start: start, End: end, Title: shell.Title})
	}
	return spans
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

	wholeSpan := batchEnd.Sub(rewriteStart)
	for i, span := range spans {
		duration := span.End.Sub(span.Start)
		name := fmt.Sprintf("card %d (%s)", i+1, span.Title)
		// The 10-minute floor applies whenever the covered span can support
		// it; a span shorter than ten minutes total is the one exception.
		if duration < minCardDuration && wholeSpan >= minCardDuration {
			issues = append(issues, fmt.Sprintf(
				"%s is %s long; every card must be at least 10 minutes — absorb it into the adjacent episode",
				name, duration))
		}
		if duration > maxCardDuration {
			issues = append(issues, fmt.Sprintf(
				"%s is %s long; every card must be at most 60 minutes",
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
