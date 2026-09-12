package analysis

import (
	"sort"
	"time"

	"github.com/Jwz-git/Daygo/internal/storage"
)

// DetectIdle reports whether a batch of frames is wholly idle (docs/04 §4.4).
// It is a pure function over (captured_at, idle_seconds_at_capture) pairs;
// NULL samples (idle sampling unavailable) are distinct from zero and count
// against availability but neither for nor against idleness.
//
// Algorithm: convert each frame's idle reading into the coverage interval
// [capturedAt - idle, capturedAt], clip to the batch span, merge overlaps,
// and require the merged coverage to reach CoverageRatio of the span with no
// hole wider than MaxUncoveredGap.
func DetectIdle(frames []storage.AnalysisFrame, rules IdleRules) bool {
	if len(frames) == 0 {
		return false
	}
	span := BatchPlan{Frames: frames}.Span()
	if span < rules.MinBatchDuration {
		return false
	}

	// Sample availability: frames with no reading at all.
	sampled := 0
	for _, f := range frames {
		if f.IdleSeconds != nil {
			sampled++
		}
	}
	if float64(sampled)/float64(len(frames)) < rules.SampleAvailabilityRatio {
		return false
	}

	// Qualified frames: the reading itself says the user was idle.
	qualified := 0
	type interval struct{ start, end int64 }
	var intervals []interval
	lo := frames[0].CapturedAt.Unix()
	hi := frames[len(frames)-1].CapturedAt.Unix()
	for _, f := range frames {
		if f.IdleSeconds == nil {
			continue
		}
		idle := time.Duration(*f.IdleSeconds) * time.Second
		// Only an idle reading produces coverage; an active frame (idle 0 or
		// small) covers nothing and does not count as a qualified frame.
		if idle >= rules.FrameIdleThreshold {
			qualified++
			start := max(f.CapturedAt.Unix()-int64(idle.Seconds()), lo)
			end := min(f.CapturedAt.Unix(), hi)
			if end > start {
				intervals = append(intervals, interval{start: start, end: end})
			}
		}
	}
	if sampled == 0 || float64(qualified)/float64(sampled) < rules.QualifiedFrameRatio {
		return false
	}

	// Merge and measure coverage and the largest hole.
	sort.Slice(intervals, func(i, j int) bool { return intervals[i].start < intervals[j].start })
	var covered float64
	maxGap := time.Duration(0)
	cursor := lo
	for _, iv := range intervals {
		if iv.start > cursor {
			gap := time.Duration(iv.start-cursor) * time.Second
			if gap > maxGap {
				maxGap = gap
			}
		}
		if iv.end > cursor {
			covered += float64(iv.end - max(cursor, iv.start))
			cursor = iv.end
		}
	}
	if hi > cursor {
		gap := time.Duration(hi-cursor) * time.Second
		if gap > maxGap {
			maxGap = gap
		}
	}

	total := float64(hi - lo)
	return covered/total >= rules.CoverageRatio && maxGap <= rules.MaxUncoveredGap
}
