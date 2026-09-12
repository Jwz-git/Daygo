package timeutil

import (
	"testing"
	"time"
)

// mustLoc loads a named zone or fails; the five contract zones (docs/08 §8.3)
// exercise whole-hour, half-hour, 45-minute, and DST boundaries.
func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return loc
}

func TestResolveClockPicksNearestOfThreeDayCandidates(t *testing.T) {
	loc := mustLoc(t, "Asia/Shanghai")
	// Window centered just after midnight: 2026-09-12 00:30 local.
	anchor := time.Date(2026, 9, 12, 0, 30, 0, 0, loc)

	// "11:50 PM" is 30 minutes before the anchor, on the previous day.
	got, err := ResolveClock("11:50 PM", anchor, loc)
	if err != nil {
		t.Fatalf("ResolveClock: %v", err)
	}
	if want := time.Date(2026, 9, 11, 23, 50, 0, 0, loc); !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	// "12:10 AM" is 20 minutes after the anchor, same day.
	got, err = ResolveClock("12:10 AM", anchor, loc)
	if err != nil {
		t.Fatalf("ResolveClock: %v", err)
	}
	if want := time.Date(2026, 9, 12, 0, 10, 0, 0, loc); !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestResolveClockMeridiemAnd24HourForms(t *testing.T) {
	loc := mustLoc(t, "Asia/Shanghai")
	anchor := time.Date(2026, 9, 12, 12, 0, 0, 0, loc)

	cases := []struct {
		in   string
		want time.Time
	}{
		{"10:21 AM", time.Date(2026, 9, 12, 10, 21, 0, 0, loc)},
		{"10:21 am", time.Date(2026, 9, 12, 10, 21, 0, 0, loc)},
		{"10:21 a.m.", time.Date(2026, 9, 12, 10, 21, 0, 0, loc)},
		{"1:05 PM", time.Date(2026, 9, 12, 13, 5, 0, 0, loc)},
		{"12:00 AM", time.Date(2026, 9, 12, 0, 0, 0, 0, loc)},
		{"12:00 PM", time.Date(2026, 9, 12, 12, 0, 0, 0, loc)},
		{"12:30 PM", time.Date(2026, 9, 12, 12, 30, 0, 0, loc)},
		{"22:05", time.Date(2026, 9, 12, 22, 5, 0, 0, loc)},
		{"00:45", time.Date(2026, 9, 12, 0, 45, 0, 0, loc)},
	}
	for _, tc := range cases {
		got, err := ResolveClock(tc.in, anchor, loc)
		if err != nil {
			t.Fatalf("ResolveClock(%q): %v", tc.in, err)
		}
		if !got.Equal(tc.want) {
			t.Fatalf("ResolveClock(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestResolveClockAroundFourAMBoundary(t *testing.T) {
	loc := mustLoc(t, "Asia/Shanghai")
	// Anchor at 03:30 — before the 4 AM boundary, so "3:45 AM" belongs to the
	// previous logical day and "4:15 AM" to the current one.
	anchor := time.Date(2026, 9, 12, 3, 30, 0, 0, loc)

	got, _ := ResolveClock("3:45 AM", anchor, loc)
	if want := time.Date(2026, 9, 12, 3, 45, 0, 0, loc); !got.Equal(want) {
		t.Fatalf("3:45 AM -> %v, want %v", got, want)
	}
	if day := LogicalDay(got, loc); day != "2026-09-11" {
		t.Fatalf("3:45 AM day = %s, want 2026-09-11 (4 AM boundary)", day)
	}

	got, _ = ResolveClock("4:15 AM", anchor, loc)
	if want := time.Date(2026, 9, 12, 4, 15, 0, 0, loc); !got.Equal(want) {
		t.Fatalf("4:15 AM -> %v, want %v", got, want)
	}
	if day := LogicalDay(got, loc); day != "2026-09-12" {
		t.Fatalf("4:15 AM day = %s, want 2026-09-12", day)
	}
}

func TestResolveClockDSTAndFractionalZones(t *testing.T) {
	// Lord Howe Island: a 30-minute DST shift. On the spring-forward day the
	// local 2:00-2:30 wall time does not exist; parsing must still return a
	// defined instant rather than panicking, and the day-candidate rule must
	// not silently jump.
	loc := mustLoc(t, "Australia/Lord_Howe")
	anchor := time.Date(2026, 10, 4, 12, 0, 0, 0, loc) // after the 02:00 shift
	got, err := ResolveClock("11:30 PM", anchor, loc)
	if err != nil {
		t.Fatalf("ResolveClock: %v", err)
	}
	if want := time.Date(2026, 10, 4, 23, 30, 0, 0, loc); !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	// Kathmandu: UTC+05:45.
	kathmandu := mustLoc(t, "Asia/Kathmandu")
	anchor = time.Date(2026, 9, 12, 12, 0, 0, 0, kathmandu)
	got, err = ResolveClock("10:21 AM", anchor, kathmandu)
	if err != nil {
		t.Fatalf("ResolveClock: %v", err)
	}
	if _, offset := got.Zone(); offset != 45*60+5*3600 {
		t.Fatalf("offset = %d, want 5h45m", offset)
	}

	// Kolkata: UTC+05:30.
	kolkata := mustLoc(t, "Asia/Kolkata")
	anchor = time.Date(2026, 9, 12, 12, 0, 0, 0, kolkata)
	if _, err = ResolveClock("10:21 AM", anchor, kolkata); err != nil {
		t.Fatalf("ResolveClock: %v", err)
	}
}

func TestResolveClockRejectsMalformedInput(t *testing.T) {
	loc := mustLoc(t, "Asia/Shanghai")
	anchor := time.Date(2026, 9, 12, 12, 0, 0, 0, loc)

	for _, in := range []string{
		"", "   ", "10", "10:21:00", "10:61 AM", "13:00 PM", "0:00 PM",
		"ten o'clock", "10:2 AM", "10:21 AM extra", "-1:30",
	} {
		if _, err := ResolveClock(in, anchor, loc); err == nil {
			t.Fatalf("ResolveClock(%q) accepted malformed input", in)
		}
	}

	if _, err := ResolveClock("10:21 AM", anchor, nil); err == nil {
		t.Fatal("ResolveClock accepted a nil location")
	}
}

// FormatClock must round-trip through ResolveClock with a same-day anchor:
// resolving the formatted string against the original instant returns the
// same instant. Midnight and noon are the meridiem edge cases.
func TestFormatClockRoundTrips(t *testing.T) {
	loc := mustLoc(t, "Asia/Shanghai")
	for _, in := range []time.Time{
		time.Date(2026, 9, 12, 0, 0, 0, 0, loc),
		time.Date(2026, 9, 12, 0, 30, 0, 0, loc),
		time.Date(2026, 9, 12, 12, 0, 0, 0, loc),
		time.Date(2026, 9, 12, 12, 59, 0, 0, loc),
		time.Date(2026, 9, 12, 23, 59, 0, 0, loc),
		time.Date(2026, 9, 12, 10, 21, 0, 0, loc),
	} {
		clock := FormatClock(in, loc)
		got, err := ResolveClock(clock, in, loc)
		if err != nil {
			t.Fatalf("ResolveClock(FormatClock(%v)) = %q: %v", in, clock, err)
		}
		if !got.Equal(in) {
			t.Fatalf("round trip %v -> %q -> %v, want %v", in, clock, got, in)
		}
	}
}

func TestFormatClockShape(t *testing.T) {
	loc := mustLoc(t, "Asia/Shanghai")
	cases := []struct {
		in   time.Time
		want string
	}{
		{time.Date(2026, 9, 12, 10, 21, 0, 0, loc), "10:21 AM"},
		{time.Date(2026, 9, 12, 13, 5, 0, 0, loc), "1:05 PM"},
		{time.Date(2026, 9, 12, 0, 0, 0, 0, loc), "12:00 AM"},
		{time.Date(2026, 9, 12, 12, 0, 0, 0, loc), "12:00 PM"},
		{time.Date(2026, 9, 12, 23, 59, 0, 0, loc), "11:59 PM"},
	}
	for _, tc := range cases {
		if got := FormatClock(tc.in, loc); got != tc.want {
			t.Fatalf("FormatClock(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
