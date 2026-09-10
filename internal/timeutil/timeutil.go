// Package timeutil defines Daygo's logical and calendar day calculations.
package timeutil

import (
	"fmt"
	"time"
)

const (
	// BoundaryHour is the local hour at which a logical day begins.
	BoundaryHour = 4
	dayLayout    = "2006-01-02"
)

// ParseDay parses a yyyy-MM-dd calendar date in loc without normalization.
func ParseDay(day string, loc *time.Location) (time.Time, error) {
	if loc == nil {
		return time.Time{}, fmt.Errorf("parse day %q: location is nil", day)
	}
	parsed, err := time.ParseInLocation(dayLayout, day, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse day %q: %w", day, err)
	}
	if parsed.Format(dayLayout) != day {
		return time.Time{}, fmt.Errorf("parse day %q: date is not in yyyy-MM-dd form", day)
	}
	return parsed, nil
}

// LogicalDay returns the yyyy-MM-dd logical day containing at in loc.
func LogicalDay(at time.Time, loc *time.Location) string {
	local := at.In(loc)
	boundary := time.Date(local.Year(), local.Month(), local.Day(), BoundaryHour, 0, 0, 0, loc)
	if local.Before(boundary) {
		local = local.AddDate(0, 0, -1)
	}
	return local.Format(dayLayout)
}

// DayWindow returns the left-closed, right-open local boundary window for day.
func DayWindow(day string, loc *time.Location) (start, end time.Time, err error) {
	date, err := ParseDay(day, loc)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	start = time.Date(date.Year(), date.Month(), date.Day(), BoundaryHour, 0, 0, 0, loc)
	next := date.AddDate(0, 0, 1)
	end = time.Date(next.Year(), next.Month(), next.Day(), BoundaryHour, 0, 0, 0, loc)
	return start, end, nil
}

// CalendarDay returns the yyyy-MM-dd calendar day containing at in loc.
func CalendarDay(at time.Time, loc *time.Location) string {
	return at.In(loc).Format(dayLayout)
}
