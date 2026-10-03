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
	var matched int64
	if category != "" {
		for _, span := range spans {
			if span.Category == category {
				matched += overlapSeconds(span.StartTs, span.EndTs, startTs, to)
			}
		}
	}
	return PlanCoverage{
		MatchedMinutes:     float64(matched) / 60,
		DistractionMinutes: DistractionMinutes(startTs, to, spans, isDistraction, loc),
	}
}

// DistractionMinutes is the union of distraction time in [fromTs, toTs): whole
// cards in a distraction category plus the distraction intervals embedded in
// any card. The union counts an embedded interval inside a distraction card
// once.
func DistractionMinutes(fromTs, toTs int64, spans []storage.CardSpan, isDistraction func(string) bool, loc *time.Location) float64 {
	var intervals []WeeklyInterval
	add := func(start, end int64) {
		start, end = max(start, fromTs), min(end, toTs)
		if end > start {
			intervals = append(intervals, WeeklyInterval{StartTs: start, EndTs: end})
		}
	}
	for _, span := range spans {
		if isDistraction(span.Category) {
			add(span.StartTs, span.EndTs)
		}
		_, _, embedded := segmentMetadata(span, loc)
		for _, interval := range embedded {
			add(interval.StartTs, interval.EndTs)
		}
	}
	return float64(unionSeconds(intervals)) / 60
}

func overlapSeconds(aStart, aEnd, bStart, bEnd int64) int64 {
	start, end := max(aStart, bStart), min(aEnd, bEnd)
	if end <= start {
		return 0
	}
	return end - start
}

func unionSeconds(intervals []WeeklyInterval) int64 {
	if len(intervals) == 0 {
		return 0
	}
	sort.Slice(intervals, func(i, j int) bool { return intervals[i].StartTs < intervals[j].StartTs })
	var total int64
	current := intervals[0]
	for _, next := range intervals[1:] {
		if next.StartTs <= current.EndTs {
			current.EndTs = max(current.EndTs, next.EndTs)
			continue
		}
		total += current.EndTs - current.StartTs
		current = next
	}
	return total + current.EndTs - current.StartTs
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
	isDistraction := DistractionRule(distractionCategories)
	reviews := make([]PlanReview, 0, len(blocks))
	for _, block := range blocks {
		reviews = append(reviews, PlanReview{
			Block:    block,
			Coverage: PlanBlockCoverage(block.StartTs, block.EndTs, nowTs, block.CategoryName, spans, isDistraction, loc),
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
