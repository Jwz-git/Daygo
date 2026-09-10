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
