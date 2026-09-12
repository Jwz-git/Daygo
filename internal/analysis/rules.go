package analysis

import "time"

// Scheduler and batching constants from docs/04 §4.3.1.
const (
	// TargetBatchDuration is the target span of one analysis batch. The
	// latest batch is left unbatched until its first-to-last span reaches it.
	TargetBatchDuration = 15 * time.Minute
	// MaxSplitGap closes a batch when two consecutive frames are farther
	// apart than this — the user stopped, slept, or the machine locked.
	MaxSplitGap = 2 * time.Minute
	// CardLookback is how much card context the card-generation stage reads
	// around the batch window (the Dayflow-style sliding window).
	CardLookback = 45 * time.Minute
	// UnbatchedLookback is how far back the scheduler scans for frames that
	// belong to no batch.
	UnbatchedLookback = 24 * time.Hour
	// MinAnalysisDuration is the minimum span of a closed batch for it to be
	// worth an LLM call; shorter closed batches become skipped_short.
	MinAnalysisDuration = 5 * time.Minute
	// FailureRetryCooldown is how long a failed batch stays failed before the
	// scheduler requeues it.
	FailureRetryCooldown = 10 * time.Minute
)

// IdleRules are the idle-detection parameters of docs/04 §4.4. They are a
// struct so tests can probe each boundary; DefaultIdleRules is the contract
// value the pipeline uses.
type IdleRules struct {
	// MinBatchDuration: batches shorter than this are never judged idle —
	// there is not enough evidence.
	MinBatchDuration time.Duration
	// CoverageRatio: the fraction of the batch span that idle coverage must
	// reach.
	CoverageRatio float64
	// QualifiedFrameRatio: the fraction of sampled frames whose idle reading
	// is at least FrameIdleThreshold.
	QualifiedFrameRatio float64
	// SampleAvailabilityRatio: the fraction of frames that must carry a
	// non-null idle sample at all.
	SampleAvailabilityRatio float64
	// FrameIdleThreshold: how many idle seconds make one frame count as idle.
	FrameIdleThreshold time.Duration
	// MaxUncoveredGap: the largest idle-time hole allowed inside the span.
	MaxUncoveredGap time.Duration
	// AdjacentIdleMergeGap: how close a preceding Idle card must be for a new
	// idle batch to merge into it instead of opening a new card.
	AdjacentIdleMergeGap time.Duration
}

// DefaultIdleRules pins the contract values of docs/04 §4.4.
func DefaultIdleRules() IdleRules {
	return IdleRules{
		MinBatchDuration:        12 * time.Minute,
		CoverageRatio:           0.95,
		QualifiedFrameRatio:     0.90,
		SampleAvailabilityRatio: 0.90,
		FrameIdleThreshold:      60 * time.Second,
		MaxUncoveredGap:         30 * time.Second,
		AdjacentIdleMergeGap:    5 * time.Minute,
	}
}
