package insight

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// WeeklySegment is one non-System card span as delivered to the weekly detail
// charts. Idle spans are kept: the focus heatmap needs them. AppPrimary /
// AppSecondary are the card's raw app/site pair (empty when absent); the
// client derives the display identity from them, the same way the timeline
// does for card icons. Distractions are the card's distraction intervals
// resolved to instants and clamped to this span.
type WeeklySegment struct {
	StartTs      int64
	EndTs        int64
	Category     string
	IsIdle       bool
	AppPrimary   string
	AppSecondary string
	Distractions []WeeklyInterval
}

// WeeklyInterval is a half-open [StartTs, EndTs) range in Unix seconds.
type WeeklyInterval struct {
	StartTs int64
	EndTs   int64
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
// excluded everywhere, matching AggregateWeekly; idle and Distraction spans
// count toward tracked but not focus. Spans whose day falls outside the week are ignored,
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
		primary, secondary, distractions := segmentMetadata(span, loc)
		accum.segments = append(accum.segments, WeeklySegment{
			StartTs:      span.StartTs,
			EndTs:        span.EndTs,
			Category:     span.Category,
			IsIdle:       span.IsIdle,
			AppPrimary:   primary,
			AppSecondary: secondary,
			Distractions: distractions,
		})

		if isWeeklyFocus(span.Category, span.IsIdle) {
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

// spanMetadata is the subset of a card's metadata JSON the weekly charts read
// (docs/05 §5.5.2). Fields that fail to decode are treated as absent.
type spanMetadata struct {
	AppSites *struct {
		Primary   *string `json:"primary"`
		Secondary *string `json:"secondary"`
	} `json:"appSites"`
	Distractions []struct {
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
	} `json:"distractions"`
}

// segmentMetadata extracts the app/site pair and the distraction intervals of
// one span. Each distraction's clock strings resolve against the span start
// with timeutil.ResolveClock (the docs/03 §3.5 three-day rule), the end
// against the resolved start so a 23:50–00:10 entry stays one interval. The
// interval is clamped to the span: a card split at the 4 AM boundary keeps
// each part's own share, and an entry that does not overlap the span, does
// not parse or is inverted is dropped — there is no instant to invent for it.
func segmentMetadata(span storage.CardSpan, loc *time.Location) (primary, secondary string, distractions []WeeklyInterval) {
	distractions = []WeeklyInterval{}
	if span.Metadata == "" {
		return "", "", distractions
	}
	var meta spanMetadata
	if err := json.Unmarshal([]byte(span.Metadata), &meta); err != nil {
		return "", "", distractions
	}
	if meta.AppSites != nil {
		if meta.AppSites.Primary != nil {
			primary = *meta.AppSites.Primary
		}
		if meta.AppSites.Secondary != nil {
			secondary = *meta.AppSites.Secondary
		}
	}
	anchor := time.Unix(span.StartTs, 0)
	for _, entry := range meta.Distractions {
		start, err := timeutil.ResolveClock(entry.StartTime, anchor, loc)
		if err != nil {
			continue
		}
		end, err := timeutil.ResolveClock(entry.EndTime, start, loc)
		if err != nil {
			continue
		}
		from := max(start.Unix(), span.StartTs)
		to := min(end.Unix(), span.EndTs)
		if to <= from {
			continue
		}
		distractions = append(distractions, WeeklyInterval{StartTs: from, EndTs: to})
	}
	return primary, secondary, distractions
}
