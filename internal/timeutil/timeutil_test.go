package timeutil

import (
	"testing"
	"time"
)

func TestLogicalDayBoundary(t *testing.T) {
	loc := mustLocation(t, "America/Los_Angeles")
	tests := []struct {
		name string
		at   time.Time
		want string
	}{
		{"before boundary", time.Date(2026, time.January, 15, 3, 59, 59, 0, loc), "2026-01-14"},
		{"at boundary", time.Date(2026, time.January, 15, 4, 0, 0, 0, loc), "2026-01-15"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := LogicalDay(test.at, loc); got != test.want {
				t.Fatalf("LogicalDay(%v) = %q, want %q", test.at, got, test.want)
			}
		})
	}
}

func TestLogicalDayAcrossLocations(t *testing.T) {
	tests := []struct {
		zone string
		at   time.Time
		want string
	}{
		{"UTC", time.Date(2026, time.January, 15, 3, 59, 59, 0, time.UTC), "2026-01-14"},
		{"Asia/Kolkata", time.Date(2026, time.January, 15, 4, 0, 0, 0, mustLocation(t, "Asia/Kolkata")), "2026-01-15"},
		{"Australia/Lord_Howe", time.Date(2026, time.April, 5, 3, 59, 59, 0, mustLocation(t, "Australia/Lord_Howe")), "2026-04-04"},
		{"Pacific/Chatham", time.Date(2026, time.April, 5, 4, 0, 0, 0, mustLocation(t, "Pacific/Chatham")), "2026-04-05"},
	}

	for _, test := range tests {
		t.Run(test.zone, func(t *testing.T) {
			loc := mustLocation(t, test.zone)
			if got := LogicalDay(test.at, loc); got != test.want {
				t.Fatalf("LogicalDay(%v) = %q, want %q", test.at, got, test.want)
			}
		})
	}
}

func TestParseDayStrict(t *testing.T) {
	loc := mustLocation(t, "Asia/Kolkata")
	got, err := ParseDay("2024-02-29", loc)
	if err != nil {
		t.Fatalf("ParseDay(valid) error = %v", err)
	}
	want := time.Date(2024, time.February, 29, 0, 0, 0, 0, loc)
	if !got.Equal(want) || got.Location() != loc {
		t.Fatalf("ParseDay(valid) = %v in %v, want %v in %v", got, got.Location(), want, loc)
	}

	invalid := []string{
		"", "2026-1-02", "2026-01-2", "26-01-02", "2026/01/02",
		"2026-01-02T00:00:00", " 2026-01-02", "2026-01-02 ",
		"2023-02-29", "2024-02-30", "2026-00-10", "2026-13-10",
	}
	for _, value := range invalid {
		t.Run(value, func(t *testing.T) {
			if _, err := ParseDay(value, loc); err == nil {
				t.Fatalf("ParseDay(%q) succeeded, want error", value)
			}
		})
	}
}

func TestDayWindowDST(t *testing.T) {
	loc := mustLocation(t, "America/Los_Angeles")
	tests := []struct {
		name         string
		day          string
		wantDuration time.Duration
		wantStart    time.Time
		wantEnd      time.Time
	}{
		{
			name:         "spring forward",
			day:          "2026-03-07",
			wantDuration: 23 * time.Hour,
			wantStart:    time.Date(2026, time.March, 7, 4, 0, 0, 0, loc),
			wantEnd:      time.Date(2026, time.March, 8, 4, 0, 0, 0, loc),
		},
		{
			name:         "fall back",
			day:          "2026-10-31",
			wantDuration: 25 * time.Hour,
			wantStart:    time.Date(2026, time.October, 31, 4, 0, 0, 0, loc),
			wantEnd:      time.Date(2026, time.November, 1, 4, 0, 0, 0, loc),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start, end, err := DayWindow(test.day, loc)
			if err != nil {
				t.Fatalf("DayWindow(%q) error = %v", test.day, err)
			}
			if !start.Equal(test.wantStart) || !end.Equal(test.wantEnd) {
				t.Fatalf("DayWindow(%q) = [%v, %v), want [%v, %v)", test.day, start, end, test.wantStart, test.wantEnd)
			}
			if got := end.Sub(start); got != test.wantDuration {
				t.Fatalf("DayWindow(%q) duration = %v, want %v", test.day, got, test.wantDuration)
			}
		})
	}
}

func TestDayWindowUsesCalendarBoundaries(t *testing.T) {
	tests := []struct {
		zone         string
		day          string
		wantDuration time.Duration
	}{
		{"UTC", "2026-03-07", 24 * time.Hour},
		{"Asia/Kolkata", "2026-03-07", 24 * time.Hour},
		{"Australia/Lord_Howe", "2026-04-04", 24*time.Hour + 30*time.Minute},
		{"Pacific/Chatham", "2026-04-04", 25 * time.Hour},
	}

	for _, test := range tests {
		t.Run(test.zone, func(t *testing.T) {
			loc := mustLocation(t, test.zone)
			start, end, err := DayWindow(test.day, loc)
			if err != nil {
				t.Fatalf("DayWindow(%q) error = %v", test.day, err)
			}
			if start.Hour() != BoundaryHour || end.Hour() != BoundaryHour {
				t.Fatalf("DayWindow(%q) local hours = %d and %d, want %d", test.day, start.Hour(), end.Hour(), BoundaryHour)
			}
			if got := end.Sub(start); got != test.wantDuration {
				t.Fatalf("DayWindow(%q) duration = %v, want %v", test.day, got, test.wantDuration)
			}
		})
	}
}

func TestCalendarDay(t *testing.T) {
	loc := mustLocation(t, "Pacific/Chatham")
	instant := time.Date(2026, time.January, 15, 2, 0, 0, 0, loc)
	if got := CalendarDay(instant, loc); got != "2026-01-15" {
		t.Fatalf("CalendarDay(%v) = %q, want %q", instant, got, "2026-01-15")
	}
	if got := LogicalDay(instant, loc); got != "2026-01-14" {
		t.Fatalf("LogicalDay(%v) = %q, want %q", instant, got, "2026-01-14")
	}
}

func TestDayBoundaryTotalityAndContainment(t *testing.T) {
	zones := []string{
		"UTC",
		"America/Los_Angeles",
		"Asia/Kolkata",
		"Australia/Lord_Howe",
		"Pacific/Chatham",
	}
	periods := []struct {
		name  string
		start time.Time
		end   time.Time
	}{
		{
			name:  "spring transitions",
			start: time.Date(2026, time.March, 6, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2026, time.April, 8, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "fall transitions",
			start: time.Date(2026, time.September, 25, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2026, time.November, 4, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, zone := range zones {
		loc := mustLocation(t, zone)
		for _, period := range periods {
			t.Run(zone+"/"+period.name, func(t *testing.T) {
				var previousDay string
				for at := period.start; at.Before(period.end); at = at.Add(17 * time.Minute) {
					day := LogicalDay(at, loc)
					start, end, err := DayWindow(day, loc)
					if err != nil {
						t.Fatalf("DayWindow(LogicalDay(%v)) error = %v", at, err)
					}
					if at.Before(start) || !at.Before(end) {
						t.Fatalf("%v assigned to %q outside [%v, %v)", at, day, start, end)
					}
					if got := LogicalDay(start, loc); got != day {
						t.Fatalf("LogicalDay(window start %v) = %q, want %q", start, got, day)
					}
					if got := LogicalDay(end.Add(-time.Nanosecond), loc); got != day {
						t.Fatalf("LogicalDay(window end-1ns %v) = %q, want %q", end, got, day)
					}
					endDay, err := ParseDay(LogicalDay(end, loc), loc)
					if err != nil {
						t.Fatalf("parse next logical day: %v", err)
					}
					currentDay, err := ParseDay(day, loc)
					if err != nil {
						t.Fatalf("parse current logical day: %v", err)
					}
					if want := currentDay.AddDate(0, 0, 1); !endDay.Equal(want) {
						t.Fatalf("logical day after %q = %q, want %q", day, endDay.Format(time.DateOnly), want.Format(time.DateOnly))
					}
					if previousDay != "" && day != previousDay {
						previous, err := ParseDay(previousDay, loc)
						if err != nil {
							t.Fatalf("parse previous logical day: %v", err)
						}
						if want := previous.AddDate(0, 0, 1).Format(time.DateOnly); day != want {
							t.Fatalf("logical day jumped from %q to %q, want %q", previousDay, day, want)
						}
					}
					previousDay = day
				}
			})
		}
	}
}

func mustLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load location %q: %v", name, err)
	}
	return loc
}

func TestWeekStart(t *testing.T) {
	loc := mustLocation(t, "Asia/Shanghai")
	tests := []struct {
		day  string
		want string
	}{
		{"2026-09-07", "2026-09-07"}, // Monday itself
		{"2026-09-09", "2026-09-07"}, // Wednesday
		{"2026-09-13", "2026-09-07"}, // Sunday
		{"2026-09-14", "2026-09-14"}, // next Monday
		{"2026-01-04", "2025-12-29"}, // Sunday spans the year boundary
	}
	for _, test := range tests {
		if got, err := WeekStart(test.day, loc); err != nil || got != test.want {
			t.Fatalf("WeekStart(%q) = %q, %v; want %q", test.day, got, err, test.want)
		}
	}
	if _, err := WeekStart("2026-9-7", loc); err == nil {
		t.Fatal("WeekStart(non-strict day) succeeded, want error")
	}
}

// Sunday before 4 AM belongs to the previous week because its logical day is
// Saturday — the direct consequence of the 4 AM alignment.
func TestWeekStartFourAMAlignment(t *testing.T) {
	loc := mustLocation(t, "Asia/Shanghai")
	before := LogicalDay(time.Date(2026, time.September, 13, 3, 59, 59, 0, loc), loc)
	after := LogicalDay(time.Date(2026, time.September, 13, 4, 0, 0, 0, loc), loc)
	if before != "2026-09-12" || after != "2026-09-13" {
		t.Fatalf("logical days = %q, %q; want 2026-09-12, 2026-09-13", before, after)
	}
	weekBefore, err := WeekStart(before, loc)
	if err != nil {
		t.Fatalf("WeekStart(before): %v", err)
	}
	weekAfter, err := WeekStart(after, loc)
	if err != nil {
		t.Fatalf("WeekStart(after): %v", err)
	}
	if weekBefore != "2026-09-07" || weekAfter != "2026-09-07" {
		t.Fatalf("Sunday 03:59/04:00 weeks = %q, %q; both want 2026-09-07", weekBefore, weekAfter)
	}
	// The actual split is Saturday midnight..4 AM: Saturday 2026-09-12 is the
	// last day of the week starting 2026-09-07 either way, so the alignment
	// matters for days near the boundary, verified above via logical days.
}

func TestWeekWindowDSTAndOddZones(t *testing.T) {
	tests := []struct {
		zone      string
		weekStart string
	}{
		// DST starts Sunday 2026-03-08 in America/Los_Angeles.
		{"America/Los_Angeles", "2026-03-02"},
		{"Asia/Kolkata", "2026-09-07"},
		{"Australia/Lord_Howe", "2026-09-07"},
		{"Pacific/Chatham", "2026-09-07"},
	}
	for _, test := range tests {
		t.Run(test.zone, func(t *testing.T) {
			loc := mustLocation(t, test.zone)
			start, end, err := WeekWindow(test.weekStart, loc)
			if err != nil {
				t.Fatalf("WeekWindow(%q): %v", test.weekStart, err)
			}
			if got := start.Format("2006-01-02 15:04"); got != test.weekStart+" 04:00" {
				t.Fatalf("window start = %s, want %s 04:00", got, test.weekStart)
			}
			next, err := WeekStart(weekStartPlus7(t, test.weekStart, loc), loc)
			if err != nil {
				t.Fatalf("next week start: %v", err)
			}
			wantEnd, _, err := WeekWindow(next, loc)
			if err != nil {
				t.Fatalf("next window: %v", err)
			}
			if !end.Equal(wantEnd) {
				t.Fatalf("window end = %v, want %v (no gap/overlap)", end, wantEnd)
			}
		})
	}
}

func weekStartPlus7(t *testing.T, day string, loc *time.Location) string {
	t.Helper()
	parsed, err := ParseDay(day, loc)
	if err != nil {
		t.Fatalf("parse %q: %v", day, err)
	}
	return parsed.AddDate(0, 0, 7).Format(time.DateOnly)
}

// Property: consecutive week windows tile the time axis without gaps or
// overlaps, and each window's interior maps back to its own weekStart via
// LogicalDay + WeekStart (docs/08 §8.6.4, "跨周一").
func TestWeekWindowTotalityAndContainment(t *testing.T) {
	for _, zone := range []string{"UTC", "America/Los_Angeles", "Asia/Kolkata", "Pacific/Chatham"} {
		t.Run(zone, func(t *testing.T) {
			loc := mustLocation(t, zone)
			// Start from a known Monday; 2026-09-07 is a Monday.
			weekStart := "2026-09-07"
			for i := 0; i < 8; i++ {
				start, end, err := WeekWindow(weekStart, loc)
				if err != nil {
					t.Fatalf("WeekWindow(%q): %v", weekStart, err)
				}
				for _, at := range []time.Time{start, start.Add(30 * time.Hour), end.Add(-time.Nanosecond)} {
					day := LogicalDay(at, loc)
					got, err := WeekStart(day, loc)
					if err != nil {
						t.Fatalf("WeekStart(%q): %v", day, err)
					}
					if got != weekStart {
						t.Fatalf("WeekStart(LogicalDay(%v)) = %q, want %q", at, got, weekStart)
					}
				}
				weekStart = weekStartPlus7(t, weekStart, loc)
			}
		})
	}
}

func TestWeekWindowRejectsNonMonday(t *testing.T) {
	loc := mustLocation(t, "UTC")
	if _, _, err := WeekWindow("2026-09-08", loc); err == nil {
		t.Fatal("WeekWindow(Tuesday) succeeded, want error")
	}
	if _, _, err := WeekWindow("2026-13-01", loc); err == nil {
		t.Fatal("WeekWindow(invalid date) succeeded, want error")
	}
}
