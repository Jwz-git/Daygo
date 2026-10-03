package timeutil

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// dayClockPattern is the strict 24-hour wall clock a planner or an agent
// types: "9:05" or "09:05". Unlike ResolveClock (which tolerates LLM output),
// user and agent input is either well-formed or refused.
var dayClockPattern = regexp.MustCompile(`^([01]?\d|2[0-3]):([0-5]\d)$`)

// ResolveDayClock places an "H:mm" wall clock inside a logical day: hours from
// the boundary (04:00) to 23:59 fall on day's own calendar date, 00:00–03:59 on
// the next one, so every result lies in DayWindow(day). A wall time skipped by
// a DST jump (02:30 on a spring-forward night) moves forward by the gap, as
// calendars do; time.Date alone would resolve it backward.
func ResolveDayClock(day, clock string, loc *time.Location) (time.Time, error) {
	date, err := ParseDay(day, loc)
	if err != nil {
		return time.Time{}, err
	}
	match := dayClockPattern.FindStringSubmatch(clock)
	if match == nil {
		return time.Time{}, fmt.Errorf("resolve day clock %q: want H:mm in 24-hour time", clock)
	}
	hour, _ := strconv.Atoi(match[1])
	minute, _ := strconv.Atoi(match[2])
	if hour < BoundaryHour {
		date = date.AddDate(0, 0, 1)
	}
	at := time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, loc)
	if wall := at.Hour()*60 + at.Minute(); wall != hour*60+minute {
		at = at.Add(time.Duration(hour*60+minute-wall) * time.Minute)
	}
	return at, nil
}

// ResolveDayRange resolves a start and end wall clock inside one logical day.
// An end at the day's own boundary ("04:00") means the end of the day. The
// result is refused unless end is strictly after start; nothing wraps, and the
// same clock typed twice is a zero-length range, not a whole day.
func ResolveDayRange(day, startClock, endClock string, loc *time.Location) (start, end time.Time, err error) {
	start, err = ResolveDayClock(day, startClock, loc)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err = ResolveDayClock(day, endClock, loc)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	dayStart, dayEnd, _ := DayWindow(day, loc)
	if end.Equal(dayStart) && !start.Equal(dayStart) {
		end = dayEnd
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("resolve day range %s–%s: end must be after start", startClock, endClock)
	}
	return start, end, nil
}
