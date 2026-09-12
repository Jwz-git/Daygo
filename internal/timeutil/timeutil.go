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

// WeekStart returns the yyyy-MM-dd Monday of the week containing day, per
// decisions/weekly-boundary-monday: weeks start Monday and align to the 4 AM
// logical-day boundary, so day is interpreted as a logical day.
func WeekStart(day string, loc *time.Location) (string, error) {
	date, err := ParseDay(day, loc)
	if err != nil {
		return "", err
	}
	offset := (int(date.Weekday()) - int(time.Monday) + 7) % 7
	monday := date.AddDate(0, 0, -offset)
	return monday.Format(dayLayout), nil
}

// WeekWindow returns the left-closed, right-open week window for weekStart:
// [weekStart 04:00, weekStart+7d 04:00) in loc. weekStart must be a Monday
// (the output of WeekStart); other weekdays are rejected.
func WeekWindow(weekStart string, loc *time.Location) (start, end time.Time, err error) {
	monday, err := ParseDay(weekStart, loc)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if monday.Weekday() != time.Monday {
		return time.Time{}, time.Time{}, fmt.Errorf("parse week start %q: not a Monday", weekStart)
	}
	start = time.Date(monday.Year(), monday.Month(), monday.Day(), BoundaryHour, 0, 0, 0, loc)
	next := monday.AddDate(0, 0, 7)
	end = time.Date(next.Year(), next.Month(), next.Day(), BoundaryHour, 0, 0, 0, loc)
	return start, end, nil
}
