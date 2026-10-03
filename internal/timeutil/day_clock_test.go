package timeutil

import (
	"testing"
	"time"
)

// A plan's wall-clock time belongs to its logical day: 04:00–23:59 fall on
// the day's own calendar date, 00:00–03:59 on the next one.
func TestResolveDayClock(t *testing.T) {
	shanghai := mustLocation(t, "Asia/Shanghai")
	kolkata := mustLocation(t, "Asia/Kolkata")
	newYork := mustLocation(t, "America/New_York")

	tests := []struct {
		name  string
		day   string
		clock string
		loc   *time.Location
		want  time.Time
	}{
		{"morning on the same date", "2026-09-12", "09:30", shanghai, time.Date(2026, 9, 12, 9, 30, 0, 0, shanghai)},
		{"single-digit hour", "2026-09-12", "9:05", shanghai, time.Date(2026, 9, 12, 9, 5, 0, 0, shanghai)},
		{"day boundary itself", "2026-09-12", "04:00", shanghai, time.Date(2026, 9, 12, 4, 0, 0, 0, shanghai)},
		{"late evening", "2026-09-12", "23:59", shanghai, time.Date(2026, 9, 12, 23, 59, 0, 0, shanghai)},
		{"after midnight is the next date", "2026-09-12", "01:30", shanghai, time.Date(2026, 9, 13, 1, 30, 0, 0, shanghai)},
		{"last minute of the logical day", "2026-09-12", "03:59", shanghai, time.Date(2026, 9, 13, 3, 59, 0, 0, shanghai)},
		{"half-hour zone", "2026-09-12", "10:15", kolkata, time.Date(2026, 9, 12, 10, 15, 0, 0, kolkata)},
		// 2026-03-08 02:30 does not exist in New York (clocks jump 02:00→03:00);
		// time.Date normalizes it forward to 03:30 EDT, still inside the day.
		{"DST gap normalizes forward", "2026-03-07", "02:30", newYork, time.Date(2026, 3, 8, 3, 30, 0, 0, newYork)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ResolveDayClock(test.day, test.clock, test.loc)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(test.want) {
				t.Fatalf("ResolveDayClock(%q, %q) = %v, want %v", test.day, test.clock, got, test.want)
			}
		})
	}

	for _, bad := range []struct{ day, clock string }{
		{"2026-09-12", "24:00"},
		{"2026-09-12", "12:60"},
		{"2026-09-12", "9am"},
		{"2026-09-12", "09:30:00"},
		{"2026-09-12", ""},
		{"2026-9-12", "09:30"},
	} {
		if _, err := ResolveDayClock(bad.day, bad.clock, shanghai); err == nil {
			t.Fatalf("ResolveDayClock(%q, %q) accepted malformed input", bad.day, bad.clock)
		}
	}
}

// A range's end of "04:00" closes the day rather than pointing back to its
// start, and an end before its start is refused rather than wrapped.
func TestResolveDayRange(t *testing.T) {
	loc := mustLocation(t, "Asia/Shanghai")
	start, end, err := ResolveDayRange("2026-09-12", "22:00", "04:00", loc)
	if err != nil {
		t.Fatal(err)
	}
	if !start.Equal(time.Date(2026, 9, 12, 22, 0, 0, 0, loc)) || !end.Equal(time.Date(2026, 9, 13, 4, 0, 0, 0, loc)) {
		t.Fatalf("22:00–04:00 = %v–%v, want it to run to the day's end", start, end)
	}
	start, end, err = ResolveDayRange("2026-09-12", "23:30", "00:45", loc)
	if err != nil || !end.After(start) || end.Sub(start) != 75*time.Minute {
		t.Fatalf("23:30–00:45 = %v–%v, %v; want 75 minutes across midnight", start, end, err)
	}
	for _, bad := range [][2]string{{"10:00", "09:00"}, {"10:00", "10:00"}, {"04:00", "04:00"}} {
		if _, _, err := ResolveDayRange("2026-09-12", bad[0], bad[1], loc); err == nil {
			t.Fatalf("range %s–%s accepted; want end after start", bad[0], bad[1])
		}
	}
}
