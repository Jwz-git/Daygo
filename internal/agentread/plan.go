package agentread

import (
	"context"
	"time"

	"github.com/Jwz-git/Daygo/internal/insight"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// Plan returns one logical day's plan blocks with their coverage up to now —
// the same insight.PlanReviews the in-app panel and chat "plan" tool use.
func (r *Reader) Plan(ctx context.Context, dayArg string) (PlanResult, error) {
	day, err := r.resolveDay(dayArg)
	if err != nil {
		return PlanResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	start, end, _ := timeutil.DayWindow(day, r.loc)

	result := PlanResult{SchemaVersion: SchemaVersion, Day: day, Blocks: []PlanBlock{}}
	blocks, err := r.store.Plans().ForDay(ctx, day)
	if err != nil {
		return PlanResult{}, faultf(CodeInternal, "read plan failed")
	}
	if len(blocks) == 0 {
		return result, nil
	}
	spans, err := r.store.Cards().CardSpansInRange(ctx, start, end)
	if err != nil {
		return PlanResult{}, faultf(CodeInternal, "read plan failed")
	}
	_, refs, _, err := r.store.Goals().Get(ctx, day)
	if err != nil {
		return PlanResult{}, faultf(CodeInternal, "read plan failed")
	}
	var distraction []string
	for _, ref := range refs {
		if ref.Role == storage.GoalRoleDistraction {
			distraction = append(distraction, ref.Name)
		}
	}
	for _, review := range insight.PlanReviews(blocks, spans, distraction, r.now().Unix(), r.loc) {
		block := review.Block
		out := PlanBlock{
			ID:                 block.ID,
			Start:              clockOf(block.StartTs, end, r.loc),
			End:                clockOf(block.EndTs, end, r.loc),
			StartTs:            block.StartTs,
			EndTs:              block.EndTs,
			Title:              block.Title,
			Notes:              block.Notes,
			Category:           block.CategoryName,
			CategoryID:         block.CategoryID,
			Status:             block.Status,
			Remind:             block.Remind,
			MatchedMinutes:     review.Coverage.MatchedMinutes,
			DistractionMinutes: review.Coverage.DistractionMinutes,
		}
		if block.CompletedAt != nil {
			at := formatTime(block.CompletedAt.Unix(), r.loc)
			out.CompletedAt = &at
		}
		result.Blocks = append(result.Blocks, out)
	}
	return result, nil
}

// clockOf renders the HH:mm the plan_add / plan_update inputs accept; the
// day's own end renders as "04:00".
func clockOf(ts int64, dayEnd time.Time, loc *time.Location) string {
	if ts == dayEnd.Unix() {
		return dayEnd.In(loc).Format("15:04")
	}
	return time.Unix(ts, 0).In(loc).Format("15:04")
}
