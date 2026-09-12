// Package insight derives read-only views (weekly aggregates) from cards.
// It contains no I/O: database queries live in internal/storage, and this
// package only combines their results (docs/modules/weekly.md).
package insight

import (
	"sort"

	"github.com/Jwz-git/Daygo/internal/storage"
)

// CategoryTotal is one row of the weekly category breakdown.
type CategoryTotal struct {
	Name    string
	Minutes float64
	Share   float64
}

// WeeklyTotals is the aggregated weekly dashboard content. TrackedMinutes
// excludes the System category; FocusMinutes additionally excludes isIdle
// categories; Shares are 0 when the tracked denominator is 0.
type WeeklyTotals struct {
	TrackedMinutes float64
	FocusMinutes   float64
	Categories     []CategoryTotal
}

// AggregateWeekly folds per-category minutes into weekly totals. The System
// category is excluded everywhere (it never counts toward tracked time and
// would break the share denominator); isIdle categories count toward tracked
// but not focus. Categories are ordered by minutes descending with a name
// tie-break so output is deterministic.
func AggregateWeekly(minutes []storage.CategoryMinutes) WeeklyTotals {
	totals := WeeklyTotals{Categories: []CategoryTotal{}}
	for _, row := range minutes {
		if row.Name == "System" {
			continue
		}
		totals.TrackedMinutes += row.Minutes
		if !row.IsIdle {
			totals.FocusMinutes += row.Minutes
		}
		totals.Categories = append(totals.Categories, CategoryTotal{
			Name:    row.Name,
			Minutes: row.Minutes,
		})
	}
	for i := range totals.Categories {
		if totals.TrackedMinutes > 0 {
			totals.Categories[i].Share = totals.Categories[i].Minutes / totals.TrackedMinutes
		}
	}
	sort.SliceStable(totals.Categories, func(i, j int) bool {
		if totals.Categories[i].Minutes != totals.Categories[j].Minutes {
			return totals.Categories[i].Minutes > totals.Categories[j].Minutes
		}
		return totals.Categories[i].Name < totals.Categories[j].Name
	})
	return totals
}
