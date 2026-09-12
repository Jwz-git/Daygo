package timeutil

import (
	"testing"
	"time"
)

func TestZoneFromLocaltimeTarget(t *testing.T) {
	cases := []struct {
		name   string
		target string
		want   string
		ok     bool
	}{
		{"macOS var db", "/var/db/timezone/zoneinfo/Asia/Shanghai", "Asia/Shanghai", true},
		{"linux usr share", "/usr/share/zoneinfo/Europe/Paris", "Europe/Paris", true},
		{"nested id", "/usr/share/zoneinfo/America/Argentina/Buenos_Aires", "America/Argentina/Buenos_Aires", true},
		{"relative symlink", "../usr/share/zoneinfo/Asia/Kolkata", "Asia/Kolkata", true},
		{"posix prefix", "/usr/share/zoneinfo/posix/Europe/Berlin", "Europe/Berlin", true},
		{"utc", "/usr/share/zoneinfo/UTC", "UTC", true},
		{"no marker", "/etc/localtime", "", false},
		{"bare abbreviation rejected", "/usr/share/zoneinfo/CST", "", false},
		{"empty after marker", "/usr/share/zoneinfo/", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := zoneFromLocaltimeTarget(tc.target)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("zoneFromLocaltimeTarget(%q) = (%q, %v), want (%q, %v)", tc.target, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestHostZoneName(t *testing.T) {
	noLink := func() (string, bool) { return "", false }
	linkTo := func(target string) func() (string, bool) {
		return func() (string, bool) { return target, true }
	}

	cases := []struct {
		name string
		tz   string
		link func() (string, bool)
		want string
		ok   bool
	}{
		{"tz wins", "Asia/Tokyo", linkTo("/usr/share/zoneinfo/Europe/Paris"), "Asia/Tokyo", true},
		{"tz local ignored, link used", "Local", linkTo("/var/db/timezone/zoneinfo/Asia/Shanghai"), "Asia/Shanghai", true},
		{"empty tz, link used", "", linkTo("/usr/share/zoneinfo/UTC"), "UTC", true},
		{"empty tz, no link", "", noLink, "", false},
		{"empty tz, unusable link", "", linkTo("/etc/localtime"), "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := hostZoneName(tc.tz, tc.link)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("hostZoneName(%q) = (%q, %v), want (%q, %v)", tc.tz, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestIsIANAName(t *testing.T) {
	cases := map[string]bool{
		"Asia/Shanghai":    true,
		"UTC":              true,
		"America/New_York": true,
		"Local":            false,
		"":                 false,
		"CST":              false,
	}
	for name, want := range cases {
		if got := isIANAName(name); got != want {
			t.Errorf("isIANAName(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestZoneNameNeverReturnsLocal is the regression guard for the timeline blank
// screen: whatever the host looks like, ZoneName must hand the frontend a value
// Intl accepts, never Go's "Local".
func TestZoneNameNeverReturnsLocal(t *testing.T) {
	if got := ZoneName(time.UTC); got != "UTC" {
		t.Fatalf("ZoneName(time.UTC) = %q, want \"UTC\"", got)
	}
	// A named location keeps its id without any host lookup.
	named := time.FixedZone("Asia/Shanghai", 8*60*60)
	if got := ZoneName(named); got != "Asia/Shanghai" {
		t.Fatalf("ZoneName(named) = %q, want \"Asia/Shanghai\"", got)
	}
	// The host zone must resolve to something Intl can format with.
	if got := ZoneName(time.Local); got == "" || got == "Local" {
		t.Fatalf("ZoneName(time.Local) = %q, want a valid IANA id", got)
	}
}
