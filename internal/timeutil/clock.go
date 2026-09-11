package timeutil

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ResolveClock resolves a localized clock string — the LLM's verbatim output
// such as "10:21 AM" — to an instant near anchor, following docs/03 §3.5:
//
//	candidates = [anchor day - 1, anchor day, anchor day + 1] each at h:m
//	return the candidate closest to anchor
//
// The three-day candidate set is what keeps "11:50 PM" correct for a window
// that straddles midnight: parsing it onto the anchor's own date alone would
// be off by a day. Whether the resolved end precedes the resolved start —
// meaning the activity crossed midnight — is the caller's rule, not this
// function's.
//
// Accepted forms are the contract format "h:mm a" ("10:21 AM", case-insensitive,
// optional periods) plus bare 24-hour "H:mm" ("22:05") so a provider that
// ignores the prompt's format still parses. Anything else is an error; callers
// must surface it as a skipped card, never a silent drop.
func ResolveClock(clock string, anchor time.Time, loc *time.Location) (time.Time, error) {
	if loc == nil {
		return time.Time{}, fmt.Errorf("resolve clock %q: location is nil", clock)
	}
	hour, minute, err := parseClockString(clock)
	if err != nil {
		return time.Time{}, err
	}

	local := anchor.In(loc)
	best := time.Time{}
	bestDiff := time.Duration(1<<62 - 1)
	for dayOffset := -1; dayOffset <= 1; dayOffset++ {
		day := local.AddDate(0, 0, dayOffset)
		candidate := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, loc)
		diff := candidate.Sub(local)
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			best = candidate
			bestDiff = diff
		}
	}
	return best, nil
}

// parseClockString extracts hour and minute from a clock string. It uses
// manual scanning rather than time.Parse because time.Parse layouts are
// locale-sensitive in exactly the ways LLM output is not.
func parseClockString(clock string) (hour, minute int, err error) {
	s := strings.TrimSpace(clock)
	if s == "" {
		return 0, 0, fmt.Errorf("parse clock %q: empty", clock)
	}

	// Split a trailing meridiem, if any. Fields collapses runs of whitespace,
	// so "10:21  AM" parses the same as "10:21 AM".
	meridiem := ""
	if fields := strings.Fields(s); len(fields) == 2 {
		s = fields[0]
		meridiem = strings.ToUpper(strings.ReplaceAll(fields[1], ".", ""))
	}

	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("parse clock %q: want h:mm", clock)
	}
	hour, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parse clock %q: hour: %w", clock, err)
	}
	minute, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parse clock %q: minute: %w", clock, err)
	}
	if len(parts[1]) != 2 || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("parse clock %q: minute out of range", clock)
	}

	switch meridiem {
	case "":
		if hour < 0 || hour > 23 {
			return 0, 0, fmt.Errorf("parse clock %q: hour out of range", clock)
		}
	case "AM", "PM":
		if hour < 1 || hour > 12 {
			return 0, 0, fmt.Errorf("parse clock %q: hour out of range for %s", clock, meridiem)
		}
		if meridiem == "PM" && hour != 12 {
			hour += 12
		}
		if meridiem == "AM" && hour == 12 {
			hour = 0
		}
	default:
		return 0, 0, fmt.Errorf("parse clock %q: unknown meridiem %q", clock, meridiem)
	}
	return hour, minute, nil
}
