package app

import (
	"context"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/insight"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// WeeklyDashboardDTO and CategoryTotalDTO mirror docs/05 §5.5.2.
type WeeklyDashboardDTO struct {
	WeekStart      string             `json:"weekStart"`
	WeekStartTs    int64              `json:"weekStartTs"`
	WeekEndTs      int64              `json:"weekEndTs"`
	TrackedMinutes float64            `json:"trackedMinutes"` // excludes "System"
	FocusMinutes   float64            `json:"focusMinutes"`   // additionally excludes isIdle categories
	Categories     []CategoryTotalDTO `json:"categories"`     // minutes DESC
}

type CategoryTotalDTO struct {
	Name    string  `json:"name"`
	Minutes float64 `json:"minutes"`
	Share   float64 `json:"share"` // 0 when tracked is 0
}

// GetWeeklyDashboard aggregates one week's card minutes. weekStart must be a
// Monday (the value GetDayContext returns as weekStart); an empty or non-Monday
// value is invalid — the frontend must never derive week boundaries itself.
func (b *Backend) GetWeeklyDashboard(weekStart string) (WeeklyDashboardDTO, error) {
	store := b.store()
	if store == nil {
		if err := b.storageFailure(); err != nil {
			return WeeklyDashboardDTO{}, mapStorageError("get weekly dashboard", err)
		}
		return WeeklyDashboardDTO{}, apperr.E(apperr.DatabaseError, "weekly requires a database", nil)
	}
	loc := b.clock.Now().Location()
	start, end, err := timeutil.WeekWindow(weekStart, loc)
	if err != nil {
		return WeeklyDashboardDTO{}, apperr.E(apperr.InvalidArgument, "weekStart must be a Monday in yyyy-MM-dd form", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()

	rows, err := store.Cards().CategoryMinutesInRange(ctx, start, end)
	if err != nil {
		return WeeklyDashboardDTO{}, mapStorageError("get weekly dashboard", err)
	}
	totals := insight.AggregateWeekly(rows)
	dto := WeeklyDashboardDTO{
		WeekStart:      weekStart,
		WeekStartTs:    start.Unix(),
		WeekEndTs:      end.Unix(),
		TrackedMinutes: totals.TrackedMinutes,
		FocusMinutes:   totals.FocusMinutes,
		Categories:     make([]CategoryTotalDTO, 0, len(totals.Categories)),
	}
	for _, c := range totals.Categories {
		dto.Categories = append(dto.Categories, CategoryTotalDTO{
			Name:    c.Name,
			Minutes: c.Minutes,
			Share:   c.Share,
		})
	}
	return dto, nil
}
