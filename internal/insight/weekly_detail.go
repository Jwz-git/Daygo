package insight

import (
	"sort"
	"time"

	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// WeeklySegment is one non-System card span as delivered to the weekly detail
// charts. Idle spans are kept: the focus heatmap needs them.
type WeeklySegment struct {
	StartTs  int64
	EndTs    int64
	Category string
	IsIdle   bool
}

// WeeklyDay is one logical day of the week: totals, per-category minutes and
// the raw segments the rhythm / workflow / heatmap charts bucket on the client.
type WeeklyDay struct {
	Day            string
	TrackedMinutes float64
	FocusMinutes   float64
	Categories     []CategoryTotal
	Segments       []WeeklySegment
}

// WeeklyInsights are the derived week facts shown as stat cards. Zero values
// (empty strings, PeakHour -1) mean "no data", not "unknown".
type WeeklyInsights struct {
	LongestFocusMinutes  float64
	LongestFocusDay      string
	PeakHour             int
	PeakHourMinutes      float64
	MostActiveDay        string
	MostActiveDayMinutes float64
	ActiveDays           int
	AvgDailyFocusMinutes float64
}

// WeeklyDetail is the per-day and insight breakdown of one week.
type WeeklyDetail struct {
	Days     []WeeklyDay
	Insights WeeklyInsights
}

// BuildWeeklyDetail folds the week's card spans into per-day rows (Mon..Sun of
// the week containing weekStart) plus derived insights. The System category is
// excluded everywhere, matching AggregateWeekly; idle spans count toward
// tracked but not focus. Spans whose day falls outside the week are ignored,
// keeping output stable against clock skew at the boundaries.
func BuildWeeklyDetail(spans []storage.CardSpan, weekStart string, loc *time.Location) WeeklyDetail {
	start, _, err := timeutil.WeekWindow(weekStart, loc)
	if err != nil {
		// The binding validates weekStart before calling; this path only keeps
		// the package total on bad input.
		return WeeklyDetail{Days: []WeeklyDay{}, Insights: WeeklyInsights{PeakHour: -1}}
	}

	dayIndex := make(map[string]int, 7)
	days := make([]WeeklyDay, 7)
	for i := range days {
		day := timeutil.LogicalDay(start.AddDate(0, 0, i), loc)
		days[i] = WeeklyDay{
			Day:        day,
			Categories: []CategoryTotal{},
			Segments:   []WeeklySegment{},
		}
		dayIndex[day] = i
	}

	colors := make(map[string]string, 8)
	for _, span := range spans {
		if span.Category != "System" && span.ColorHex != "" {
			colors[span.Category] = span.ColorHex
		}
	}

	type dayAccumulator struct {
		tracked  float64
		focus    float64
		byName   map[string]float64
		segments []WeeklySegment
	}
	accums := make([]dayAccumulator, 7)
	for i := range accums {
		accums[i].byName = map[string]float64{}
	}

	insights := WeeklyInsights{PeakHour: -1}
	hours := [24]float64{}
	var focusTotal float64

	for _, span := range spans {
		if span.Category == "System" {
			continue
		}
		index, ok := dayIndex[span.Day]
		if !ok {
			continue
		}
		minutes := float64(span.EndTs-span.StartTs) / 60
		accum := &accums[index]
		accum.tracked += minutes
		accum.byName[span.Category] += minutes
		accum.segments = append(accum.segments, WeeklySegment{
			StartTs:  span.StartTs,
			EndTs:    span.EndTs,
			Category: span.Category,
			IsIdle:   span.IsIdle,
		})

		if !span.IsIdle {
			accum.focus += minutes
			focusTotal += minutes
			addSpanHours(span.StartTs, span.EndTs, loc, &hours)
			if minutes > insights.LongestFocusMinutes {
				insights.LongestFocusMinutes = minutes
				insights.LongestFocusDay = days[index].Day
			}
		}
	}

	for i := range days {
		accum := &accums[i]
		days[i].TrackedMinutes = accum.tracked
		days[i].FocusMinutes = accum.focus
		days[i].Segments = accum.segments
		days[i].Categories = make([]CategoryTotal, 0, len(accum.byName))
		for name, minutes := range accum.byName {
			share := 0.0
			if accum.tracked > 0 {
				share = minutes / accum.tracked
			}
			days[i].Categories = append(days[i].Categories, CategoryTotal{
				Name:     name,
				Minutes:  minutes,
				Share:    share,
				ColorHex: colors[name],
			})
		}
		sort.SliceStable(days[i].Categories, func(a, b int) bool {
			if days[i].Categories[a].Minutes != days[i].Categories[b].Minutes {
				return days[i].Categories[a].Minutes > days[i].Categories[b].Minutes
			}
			return days[i].Categories[a].Name < days[i].Categories[b].Name
		})

		if accum.tracked > 0 {
			insights.ActiveDays++
			if accum.tracked > insights.MostActiveDayMinutes {
				insights.MostActiveDayMinutes = accum.tracked
				insights.MostActiveDay = days[i].Day
			}
		}
	}

	for hour, minutes := range hours {
		if minutes > insights.PeakHourMinutes {
			insights.PeakHourMinutes = minutes
			insights.PeakHour = hour
		}
	}
	if insights.ActiveDays > 0 {
		insights.AvgDailyFocusMinutes = focusTotal / float64(insights.ActiveDays)
	}

	return WeeklyDetail{Days: days, Insights: insights}
}

// addSpanHours distributes a span's minutes across local clock hours
// (DST and half-hour zones are safe: hour boundaries come from time.Date in
// the zone, not from UTC truncation).
func addSpanHours(startTs, endTs int64, loc *time.Location, hours *[24]float64) {
	for at := startTs; at < endTs; {
		current := time.Unix(at, 0).In(loc)
		hourEnd := time.Date(current.Year(), current.Month(), current.Day(), current.Hour()+1, 0, 0, 0, loc)
		segmentEnd := min(hourEnd.Unix(), endTs)
		hours[current.Hour()] += float64(segmentEnd-at) / 60
		at = segmentEnd
	}
}
