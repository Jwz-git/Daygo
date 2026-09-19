//go:build !darwin

package favicon

import "net/url"

// systemProxyURL has no portable non-macOS implementation; favicon fetches fall
// back to the environment proxy only. Keeps the core buildable on Linux (the
// CGO_ENABLED=0 / Linux test gate).
func systemProxyURL() *url.URL {
	return nil
}
