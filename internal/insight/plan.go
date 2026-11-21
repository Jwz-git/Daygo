package insight

import (
	"sort"
	"strings"
	"time"

	"github.com/Jwz-git/Daygo/internal/storage"
)

// PlanCoverage is what the recorded cards say about one plan block up to now:
// card time in the block's own category, and distraction time inside it.
// Both are derived on read and never stored (docs/modules/plan.md).
type PlanCoverage struct {
	MatchedMinutes     float64
	DistractionMinutes float64
}

// IsDistractionCategory is the built-in Distraction rule shared with the
// weekly focus split (isWeeklyFocus) and the daily presentation.
func IsDistractionCategory(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	return normalized == "distraction" || normalized == "distractions"
}

// PlanBlockCoverage measures one block [startTs, endTs) up to nowTs. category
// is the block's category name; empty means no category, so nothing matches.
// isDistraction decides which card categories count as distraction (the
// built-in rule plus, for callers that have one, the day goal's set).
func PlanBlockCoverage(startTs, endTs, nowTs int64, category string, spans []storage.CardSpan,
	isDistraction func(string) bool, loc *time.Location) PlanCoverage {
	to := min(endTs, nowTs)
	if to <= startTs {
		return PlanCoverage{}
	}
	distractions := NewDistractionIntervals(spans, isDistraction, loc)
	return planBlockCoverage(startTs, to, category, spans, distractions)
}

func planBlockCoverage(fromTs, toTs int64, category string, spans []storage.CardSpan,
	distractions DistractionIntervals) PlanCoverage {
	if toTs <= fromTs {
		return PlanCoverage{}
	}
	var matched int64
	if category != "" {
		for _, span := range spans {
			if span.Category == category {
				matched += overlapSeconds(span.StartTs, span.EndTs, fromTs, toTs)
			}
		}
	}
	return PlanCoverage{
		MatchedMinutes:     float64(matched) / 60,
		DistractionMinutes: distractions.Minutes(fromTs, toTs),
	}
}

// DistractionMinutes is the union of distraction time in [fromTs, toTs).
func DistractionMinutes(fromTs, toTs int64, spans []storage.CardSpan, isDistraction func(string) bool, loc *time.Location) float64 {
	return NewDistractionIntervals(spans, isDistraction, loc).Minutes(fromTs, toTs)
}

// DistractionIntervals holds the ordered, disjoint distraction intervals
// derived from whole cards and embedded metadata.
type DistractionIntervals struct {
	intervals []WeeklyInterval
}

// NewDistractionIntervals parses each relevant span once for all time windows
// in a plan review or reminder pass.
func NewDistractionIntervals(spans []storage.CardSpan, isDistraction func(string) bool, loc *time.Location) DistractionIntervals {
	var intervals []WeeklyInterval
	for _, span := range spans {
		if span.EndTs <= span.StartTs {
			continue
		}
		if isDistraction(span.Category) {
			intervals = append(intervals, WeeklyInterval{StartTs: span.StartTs, EndTs: span.EndTs})
			continue // Embedded intervals are already covered by the whole span.
		}
		_, _, embedded := segmentMetadata(span, loc)
		intervals = append(intervals, embedded...)
	}
	sort.Slice(intervals, func(i, j int) bool { return intervals[i].StartTs < intervals[j].StartTs })
	merged := intervals[:0]
	for _, interval := range intervals {
		if last := len(merged) - 1; last >= 0 && interval.StartTs <= merged[last].EndTs {
			merged[last].EndTs = max(merged[last].EndTs, interval.EndTs)
		} else {
			merged = append(merged, interval)
		}
	}
	return DistractionIntervals{intervals: merged}
}

// Minutes measures the distraction union clipped to [fromTs, toTs).
func (d DistractionIntervals) Minutes(fromTs, toTs int64) float64 {
	if toTs <= fromTs {
		return 0
	}
	first := sort.Search(len(d.intervals), func(i int) bool { return d.intervals[i].EndTs > fromTs })
	var total int64
	for _, interval := range d.intervals[first:] {
		if interval.StartTs >= toTs {
			break
		}
		total += overlapSeconds(interval.StartTs, interval.EndTs, fromTs, toTs)
	}
	return float64(total) / 60
}

func overlapSeconds(aStart, aEnd, bStart, bEnd int64) int64 {
	start, end := max(aStart, bStart), min(aEnd, bEnd)
	if end <= start {
		return 0
	}
	return end - start
}

// PlanReview pairs a stored block with its coverage up to now.
type PlanReview struct {
	Block    storage.PlanBlock
	Coverage PlanCoverage
}

// PlanReviews measures every block of a day. distractionCategories are the
// day goal's distraction category names, added to the built-in rule. The
// app bindings and the agent read face both call this, so the in-app panel,
// the CLI and MCP report the same numbers.
func PlanReviews(blocks []storage.PlanBlock, spans []storage.CardSpan, distractionCategories []string,
	nowTs int64, loc *time.Location) []PlanReview {
	reviews := make([]PlanReview, 0, len(blocks))
	if len(blocks) == 0 {
		return reviews
	}
	distractions := NewDistractionIntervals(spans, DistractionRule(distractionCategories), loc)
	for _, block := range blocks {
		reviews = append(reviews, PlanReview{
			Block:    block,
			Coverage: planBlockCoverage(block.StartTs, min(block.EndTs, nowTs), block.CategoryName, spans, distractions),
		})
	}
	return reviews
}

// DistractionRule is the built-in Distraction rule plus a goal's own
// distraction categories.
func DistractionRule(extra []string) func(string) bool {
	set := make(map[string]struct{}, len(extra))
	for _, name := range extra {
		set[name] = struct{}{}
	}
	return func(name string) bool {
		if IsDistractionCategory(name) {
			return true
		}
		_, ok := set[name]
		return ok
	}
}
