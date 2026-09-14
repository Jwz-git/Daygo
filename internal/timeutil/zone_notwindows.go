//go:build !windows

package timeutil

// windowsZoneName keeps ZoneName platform-neutral. Non-Windows hosts use the
// $TZ and /etc/localtime resolution paths in zone.go.
func windowsZoneName() (string, bool) {
	return "", false
}
