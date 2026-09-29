package agentread

import (
	"context"

	"github.com/Jwz-git/Daygo/internal/insight"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// Daily returns one logical day's journal and goal — the same journal+goal view
// the chat "daily" tool serves (docs/05 §5.12), so the three faces stay
// same-source.
func (r *Reader) Daily(ctx context.Context, dayArg string) (DailyResult, error) {
	day, err := r.resolveDay(dayArg)
	if err != nil {
		return DailyResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	result := DailyResult{
		SchemaVersion: SchemaVersion,
		Day:           day,
		Goal: DailyGoal{
			FocusCategories:       []GoalCategoryRef{},
			DistractionCategories: []GoalCategoryRef{},
		},
	}

	entry, found, err := r.store.Journal().Get(ctx, day)
	if err != nil {
		return DailyResult{}, faultf(CodeInternal, "read daily journal failed")
	}
	if found {
		updatedAt := formatTime(entry.UpdatedAt.Unix(), r.loc)
		result.Journal = DailyJournal{
			Intentions:  entry.Intentions,
			Notes:       entry.Notes,
			Goals:       entry.Goals,
			Reflections: entry.Reflections,
			Status:      entry.Status,
			UpdatedAt:   &updatedAt,
		}
	}

	goal, refs, goalFound, err := r.store.Goals().Get(ctx, day)
	if err != nil {
		return DailyResult{}, faultf(CodeInternal, "read daily goal failed")
	}
	if goalFound {
		result.Goal.Exists = true
		result.Goal.FocusTargetMinutes = goal.FocusTargetMinutes
		result.Goal.DistractionLimitMinutes = goal.DistractionLimitMinutes
		result.Goal.IsSkipped = goal.IsSkipped
		for _, ref := range refs {
			out := GoalCategoryRef{
				CategoryID: ref.CategoryID,
				Name:       ref.Name,
				ColorHex:   ref.ColorHex,
				SortOrder:  ref.SortOrder,
			}
			switch ref.Role {
			case storage.GoalRoleFocus:
				result.Goal.FocusCategories = append(result.Goal.FocusCategories, out)
			case storage.GoalRoleDistraction:
				result.Goal.DistractionCategories = append(result.Goal.DistractionCategories, out)
			}
		}
	}
	return result, nil
}

// resolveWeekStart maps "" to the Monday of the current logical week and passes
// an explicit value through unchanged (WeekWindow rejects a non-Monday).
func (r *Reader) resolveWeekStart(arg string) (string, error) {
	if arg != "" {
		return arg, nil
	}
	today := timeutil.LogicalDay(r.now(), r.loc)
	weekStart, err := timeutil.WeekStart(today, r.loc)
	if err != nil {
		return "", faultf(CodeInternal, "resolve current week failed")
	}
	return weekStart, nil
}

// Weekly aggregates one week's per-category minutes and per-day detail. The
// same storage queries and insight aggregation the weekly binding uses, so the
// totals (System excluded, focus excluding idle/Distraction) match.
func (r *Reader) Weekly(ctx context.Context, weekStartArg string) (WeeklyResult, error) {
	weekStart, err := r.resolveWeekStart(weekStartArg)
	if err != nil {
		return WeeklyResult{}, err
	}
	start, end, err := timeutil.WeekWindow(weekStart, r.loc)
	if err != nil {
		return WeeklyResult{}, faultf(CodeInvalidArgument, "weekStart must be a Monday in yyyy-MM-dd form")
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	rows, err := r.store.Cards().CategoryMinutesInRange(ctx, start, end)
	if err != nil {
		return WeeklyResult{}, faultf(CodeInternal, "read weekly totals failed")
	}
	totals := insight.AggregateWeekly(rows)

	result := WeeklyResult{
		SchemaVersion:  SchemaVersion,
		WeekStart:      weekStart,
		WeekStartTs:    start.Unix(),
		WeekEndTs:      end.Unix(),
		TrackedMinutes: totals.TrackedMinutes,
		FocusMinutes:   totals.FocusMinutes,
		Categories:     make([]WeeklyCategory, 0, len(totals.Categories)),
		Days:           make([]WeeklyDay, 0, 7),
	}
	for _, c := range totals.Categories {
		result.Categories = append(result.Categories, WeeklyCategory{
			Name: c.Name, Minutes: c.Minutes, Share: c.Share, ColorHex: c.ColorHex,
		})
	}

	spans, err := r.store.Cards().CardSpansInRange(ctx, start, end)
	if err != nil {
		return WeeklyResult{}, faultf(CodeInternal, "read weekly detail failed")
	}
	detail := insight.BuildWeeklyDetail(spans, weekStart, r.loc)
	for _, day := range detail.Days {
		out := WeeklyDay{
			Day:            day.Day,
			TrackedMinutes: day.TrackedMinutes,
			FocusMinutes:   day.FocusMinutes,
			Categories:     make([]WeeklyCategory, 0, len(day.Categories)),
		}
		for _, c := range day.Categories {
			out.Categories = append(out.Categories, WeeklyCategory{
				Name: c.Name, Minutes: c.Minutes, Share: c.Share, ColorHex: c.ColorHex,
			})
		}
		result.Days = append(result.Days, out)
	}
	ins := detail.Insights
	result.Insights = WeeklyInsights{
		LongestFocusMinutes:  ins.LongestFocusMinutes,
		LongestFocusDay:      ins.LongestFocusDay,
		PeakHour:             ins.PeakHour,
		PeakHourMinutes:      ins.PeakHourMinutes,
		MostActiveDay:        ins.MostActiveDay,
		MostActiveDayMinutes: ins.MostActiveDayMinutes,
		ActiveDays:           ins.ActiveDays,
		AvgDailyFocusMinutes: ins.AvgDailyFocusMinutes,
	}
	return result, nil
}

// Categories lists every category, built-ins included, for inspection. The
// chat/MCP tool face hides built-ins because they cannot be assigned; the CLI
// command shows them so the full set is visible.
func (r *Reader) Categories(ctx context.Context) (CategoriesResult, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	list, err := r.store.Categories().List(ctx)
	if err != nil {
		return CategoriesResult{}, faultf(CodeInternal, "read categories failed")
	}
	result := CategoriesResult{SchemaVersion: SchemaVersion, Categories: make([]Category, 0, len(list))}
	for _, c := range list {
		result.Categories = append(result.Categories, Category{
			ID: c.ID, Name: c.Name, ColorHex: c.ColorHex, Details: c.Details,
			SortOrder: c.SortOrder, IsSystem: c.IsSystem, IsIdle: c.IsIdle,
		})
	}
	return result, nil
}
