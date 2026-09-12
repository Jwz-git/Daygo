package analysis

import (
	"time"

	"github.com/Jwz-git/Daygo/internal/storage"
)

// BatchPlan is a proposed batch: the frames, the span, and whether the
// scheduler may still grow it.
type BatchPlan struct {
	Frames []storage.AnalysisFrame
}

// Span is first-frame to last-frame timestamp, NOT frame count times
// interval. A 90-frame batch at a 10-second interval spans 890 seconds; the
// one-interval shortfall against TargetBatchDuration is deliberate and every
// comparison below preserves it (docs/04 §4.3.1).
func (p BatchPlan) Span() time.Duration {
	if len(p.Frames) == 0 {
		return 0
	}
	return p.Frames[len(p.Frames)-1].CapturedAt.Sub(p.Frames[0].CapturedAt)
}

// Start and End are the batch's time bounds.
func (p BatchPlan) Start() time.Time { return p.Frames[0].CapturedAt }
func (p BatchPlan) End() time.Time   { return p.Frames[len(p.Frames)-1].CapturedAt }

// splitResult is SplitFrames' output: batches sealed by a gap or by reaching
// the target duration, plus the trailing batch that is still growing.
type splitResult struct {
	// Closed batches can no longer grow: a gap or the target duration sealed
	// them. The scheduler decides per batch whether it is long enough to
	// analyze (>= MinAnalysisDuration) or becomes skipped_short.
	Closed []BatchPlan
	// Latest is the trailing run of frames. The caller must NOT create a
	// batch from it unless its span has already reached TargetBatchDuration.
	Latest BatchPlan
}

// SplitFrames divides time-ordered frames into batches (docs/04 §4.3.1):
// a gap larger than MaxSplitGap seals a batch, and so does reaching the
// target duration. The trailing run is returned separately as Latest: it is
// the batch that is still filling, and the caller drops it unless its span
// already meets the target — which is the off-by-one-interval rule: 90 frames
// at 10s span 890s < 900s, so even a "complete" 15-minute batch waits one
// more interval before it is processed.
func SplitFrames(frames []storage.AnalysisFrame) splitResult {
	var result splitResult
	var current []storage.AnalysisFrame

	seal := func() {
		if len(current) > 0 {
			result.Closed = append(result.Closed, BatchPlan{Frames: current})
			current = nil
		}
	}

	for i, f := range frames {
		if i > 0 {
			gap := f.CapturedAt.Sub(frames[i-1].CapturedAt)
			span := frames[i-1].CapturedAt.Sub(current[0].CapturedAt)
			if gap > MaxSplitGap || span >= TargetBatchDuration {
				seal()
			}
		}
		current = append(current, f)
	}
	result.Latest = BatchPlan{Frames: current}
	return result
}
