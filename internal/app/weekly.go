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
	FocusMinutes   float64            `json:"focusMinutes"`   // additionally excludes idle and Distraction categories
	Categories     []CategoryTotalDTO `json:"categories"`     // minutes DESC
	Days           []WeeklyDayDTO     `json:"days"`           // Mon..Sun of this week
	Insights       WeeklyInsightsDTO  `json:"insights"`
}

type CategoryTotalDTO struct {
	Name     string  `json:"name"`
	Minutes  float64 `json:"minutes"`
	Share    float64 `json:"share"` // 0 when tracked is 0
	ColorHex string  `json:"colorHex"`
}

type WeeklyDayDTO struct {
	Day            string             `json:"day"`
	TrackedMinutes float64            `json:"trackedMinutes"`
	FocusMinutes   float64            `json:"focusMinutes"`
	Categories     []CategoryTotalDTO `json:"categories"`
	Segments       []WeeklySegmentDTO `json:"segments"`
}

type WeeklySegmentDTO struct {
	StartTs  int64  `json:"startTs"`
	EndTs    int64  `json:"endTs"`
	Category string `json:"category"`
	IsIdle   bool   `json:"isIdle"`
	// The card's raw app/site pair, as on TimelineCardDTO; nil when the card
	// carries neither. The client derives the display identity from it.
	AppSites *AppSitesDTO `json:"appSites"`
	// The card's distraction intervals resolved to instants and clamped to
	// this segment; empty when there are none.
	Distractions []WeeklyIntervalDTO `json:"distractions"`
}

// WeeklyIntervalDTO is a half-open [startTs, endTs) range in Unix seconds.
type WeeklyIntervalDTO struct {
	StartTs int64 `json:"startTs"`
	EndTs   int64 `json:"endTs"`
}

type WeeklyInsightsDTO struct {
	LongestFocusMinutes  float64 `json:"longestFocusMinutes"`
	LongestFocusDay      string  `json:"longestFocusDay"`
	PeakHour             int     `json:"peakHour"` // -1 when no focus minutes exist
	PeakHourMinutes      float64 `json:"peakHourMinutes"`
	MostActiveDay        string  `json:"mostActiveDay"`
	MostActiveDayMinutes float64 `json:"mostActiveDayMinutes"`
	ActiveDays           int     `json:"activeDays"`
	AvgDailyFocusMinutes float64 `json:"avgDailyFocusMinutes"`
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
	loc := store.Location()
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
		Days:           make([]WeeklyDayDTO, 0, 7),
	}
	for _, c := range totals.Categories {
		dto.Categories = append(dto.Categories, CategoryTotalDTO{
			Name:     c.Name,
			Minutes:  c.Minutes,
			Share:    c.Share,
			ColorHex: c.ColorHex,
		})
	}

	spans, err := store.Cards().CardSpansInRange(ctx, start, end)
	if err != nil {
		return WeeklyDashboardDTO{}, mapStorageError("get weekly dashboard", err)
	}
	detail := insight.BuildWeeklyDetail(spans, weekStart, loc)
	for _, day := range detail.Days {
		dayDTO := WeeklyDayDTO{
			Day:            day.Day,
			TrackedMinutes: day.TrackedMinutes,
			FocusMinutes:   day.FocusMinutes,
			Categories:     make([]CategoryTotalDTO, 0, len(day.Categories)),
			Segments:       make([]WeeklySegmentDTO, 0, len(day.Segments)),
		}
		for _, c := range day.Categories {
			dayDTO.Categories = append(dayDTO.Categories, CategoryTotalDTO{
				Name:     c.Name,
				Minutes:  c.Minutes,
				Share:    c.Share,
				ColorHex: c.ColorHex,
			})
		}
		for _, segment := range day.Segments {
			segmentDTO := WeeklySegmentDTO{
				StartTs:      segment.StartTs,
				EndTs:        segment.EndTs,
				Category:     segment.Category,
				IsIdle:       segment.IsIdle,
				AppSites:     weeklyAppSites(segment.AppPrimary, segment.AppSecondary),
				Distractions: make([]WeeklyIntervalDTO, 0, len(segment.Distractions)),
			}
			for _, interval := range segment.Distractions {
				segmentDTO.Distractions = append(segmentDTO.Distractions, WeeklyIntervalDTO{
					StartTs: interval.StartTs,
					EndTs:   interval.EndTs,
				})
			}
			dayDTO.Segments = append(dayDTO.Segments, segmentDTO)
		}
		dto.Days = append(dto.Days, dayDTO)
	}
	ins := detail.Insights
	dto.Insights = WeeklyInsightsDTO{
		LongestFocusMinutes:  ins.LongestFocusMinutes,
		LongestFocusDay:      ins.LongestFocusDay,
		PeakHour:             ins.PeakHour,
		PeakHourMinutes:      ins.PeakHourMinutes,
		MostActiveDay:        ins.MostActiveDay,
		MostActiveDayMinutes: ins.MostActiveDayMinutes,
		ActiveDays:           ins.ActiveDays,
		AvgDailyFocusMinutes: ins.AvgDailyFocusMinutes,
	}
	return dto, nil
}

// weeklyAppSites maps the insight segment's raw pair onto the timeline's
// AppSitesDTO shape: an empty side is nil, and a card with neither is nil.
func weeklyAppSites(primary, secondary string) *AppSitesDTO {
	if primary == "" && secondary == "" {
		return nil
	}
	sites := &AppSitesDTO{}
	if primary != "" {
		sites.Primary = &primary
	}
	if secondary != "" {
		sites.Secondary = &secondary
	}
	return sites
}
