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
// host's real zone from $TZ or the /etc/localtime symlink, and only falls back
// to "UTC" — a value Intl always accepts — when nothing resolves.
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
