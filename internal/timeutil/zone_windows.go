//go:build windows

package timeutil

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

const windowsTimeZoneRegistryPath = `SYSTEM\CurrentControlSet\Control\TimeZoneInformation`

// windowsZoneName reads the stable, untranslated Windows time-zone key. Go
// exposes the active zone as "Local" on Windows, which cannot be passed to
// Intl; TimeZoneKeyName is the system source needed to translate it to IANA.
func windowsZoneName() (string, bool) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, windowsTimeZoneRegistryPath, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer key.Close()

	name, _, err := key.GetStringValue("TimeZoneKeyName")
	if err != nil {
		return "", false
	}
	iana, ok := windowsZoneToIANA[strings.TrimSpace(name)]
	return iana, ok
}
