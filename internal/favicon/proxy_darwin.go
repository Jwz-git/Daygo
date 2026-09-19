//go:build darwin

package favicon

import (
	"net/url"
	"os/exec"
	"strings"
)

// systemProxyURL reads the macOS system HTTPS proxy via `scutil --proxy`, so
// favicon fetches use the same proxy the OS (and Dayflow's URLSession) would.
// Go's net/http reads only the *_PROXY environment variables, not the system
// configuration, so this bridges that gap. It runs once at startup; a nil
// result means no system proxy is configured.
//
// This is a read-only configuration query, not a capture/keychain/host system
// capability, so it stays local to the favicon network concern rather than
// going through internal/platform. It degrades to nil off macOS.
func systemProxyURL() *url.URL {
	out, err := exec.Command("scutil", "--proxy").Output()
	if err != nil {
		return nil
	}
	fields := parseScutilProxy(string(out))
	if fields["HTTPSEnable"] != "1" {
		return nil
	}
	host := fields["HTTPSProxy"]
	if host == "" {
		return nil
	}
	u := &url.URL{Scheme: "http", Host: host}
	if port := fields["HTTPSPort"]; port != "" {
		u.Host = host + ":" + port
	}
	return u
}

// parseScutilProxy turns the `scutil --proxy` dictionary dump into a flat map.
// Lines look like "  HTTPSProxy : 127.0.0.1"; values are taken verbatim.
func parseScutilProxy(out string) map[string]string {
	fields := make(map[string]string)
	for line := range strings.SplitSeq(out, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return fields
}
