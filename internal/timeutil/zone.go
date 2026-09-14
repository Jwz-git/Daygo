// Package timeutil defines Daygo's logical and calendar day calculations.
package timeutil

import (
	"os"
	"strings"
	"time"
)

// ZoneName returns an IANA time-zone identifier for loc (e.g. "Asia/Shanghai"),
// suitable for the frontend's Intl.DateTimeFormat.
//
// Go names the host zone "Local", which Intl rejects with a RangeError that
// blanks the timeline. So when loc carries no usable IANA name this recovers the
// host's real zone from $TZ, the Windows time-zone registry key, or the
// /etc/localtime symlink, and only falls back to "UTC" — a value Intl always
// accepts — when nothing resolves.
func ZoneName(loc *time.Location) string {
	if loc == nil {
		loc = time.Local
	}
	if name := loc.String(); isIANAName(name) {
		return name
	}
	if name, ok := hostZoneName(os.Getenv("TZ"), readLocaltimeLink); ok {
		return name
	}
	if name, ok := windowsZoneName(); ok {
		return name
	}
	return "UTC"
}

// readLocaltimeLink reads the /etc/localtime symlink target, present on macOS and
// Linux. A regular file or a missing link is a valid "unknown" answer.
func readLocaltimeLink() (string, bool) {
	target, err := os.Readlink("/etc/localtime")
	if err != nil {
		return "", false
	}
	return target, true
}

// hostZoneName is the pure decision behind ZoneName: prefer an explicit $TZ,
// otherwise the zone embedded in the /etc/localtime symlink target. readLink is
// injected so tests can exercise it without touching the host's clock config.
func hostZoneName(tz string, readLink func() (string, bool)) (string, bool) {
	if isIANAName(tz) {
		return tz, true
	}
	if target, ok := readLink(); ok {
		if name, ok := zoneFromLocaltimeTarget(target); ok {
			return name, true
		}
	}
	return "", false
}

// zoneFromLocaltimeTarget extracts the IANA id from a symlink target such as
// "/var/db/timezone/zoneinfo/Asia/Shanghai" (macOS) or
// "/usr/share/zoneinfo/America/Argentina/Buenos_Aires" (Linux, nested id).
func zoneFromLocaltimeTarget(target string) (string, bool) {
	const marker = "zoneinfo/"
	idx := strings.LastIndex(target, marker)
	if idx < 0 {
		return "", false
	}
	name := strings.TrimPrefix(target[idx+len(marker):], "posix/")
	if !isIANAName(name) {
		return "", false
	}
	return name, true
}

// isIANAName accepts region/city ids (Asia/Shanghai) and "UTC", the values Intl
// can format with, while rejecting Go's "Local", the empty string, and bare
// abbreviations such as "CST" that Intl would throw on.
func isIANAName(name string) bool {
	switch name {
	case "", "Local":
		return false
	case "UTC":
		return true
	}
	return strings.Contains(name, "/")
}

// windowsZoneToIANA covers the Windows identifiers that Daygo can encounter
// in its supported desktop environments. Windows stores its local zone under a
// Windows-specific key (for example, "China Standard Time"), while the
// frontend's Intl API requires an IANA identifier.
//
// This is deliberately a mapping of stable Windows keys, never translated UI
// display names. Unknown keys retain the existing UTC fallback rather than
// guessing from a UTC offset, which would get daylight-saving transitions
// wrong.
var windowsZoneToIANA = map[string]string{
	"China Standard Time":             "Asia/Shanghai",
	"Tokyo Standard Time":             "Asia/Tokyo",
	"Korea Standard Time":             "Asia/Seoul",
	"Singapore Standard Time":         "Asia/Singapore",
	"Taipei Standard Time":            "Asia/Taipei",
	"India Standard Time":             "Asia/Kolkata",
	"SE Asia Standard Time":           "Asia/Bangkok",
	"W. Australia Standard Time":      "Australia/Perth",
	"AUS Eastern Standard Time":       "Australia/Sydney",
	"New Zealand Standard Time":       "Pacific/Auckland",
	"GMT Standard Time":               "Europe/London",
	"W. Europe Standard Time":         "Europe/Berlin",
	"Central Europe Standard Time":    "Europe/Budapest",
	"Romance Standard Time":           "Europe/Paris",
	"E. Europe Standard Time":         "Europe/Chisinau",
	"Russian Standard Time":           "Europe/Moscow",
	"Turkey Standard Time":            "Europe/Istanbul",
	"Arabian Standard Time":           "Asia/Dubai",
	"Israel Standard Time":            "Asia/Jerusalem",
	"South Africa Standard Time":      "Africa/Johannesburg",
	"Egypt Standard Time":             "Africa/Cairo",
	"Eastern Standard Time":           "America/New_York",
	"Central Standard Time":           "America/Chicago",
	"Mountain Standard Time":          "America/Denver",
	"US Mountain Standard Time":       "America/Phoenix",
	"Pacific Standard Time":           "America/Los_Angeles",
	"Alaskan Standard Time":           "America/Anchorage",
	"Hawaiian Standard Time":          "Pacific/Honolulu",
	"Atlantic Standard Time":          "America/Halifax",
	"SA Eastern Standard Time":        "America/Argentina/Buenos_Aires",
	"E. South America Standard Time":  "America/Sao_Paulo",
	"Central Brazilian Standard Time": "America/Cuiaba",
}
