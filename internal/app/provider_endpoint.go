package app

import (
	"net/url"
	"strings"

	daygoai "github.com/Jwz-git/Daygo/internal/ai"
)

// normalizeTestEndpoint validates a configured http(s) URL and strips query,
// fragment, trailing slashes and a pasted request-path suffix before saving
// providers or listing models from a draft.
//
// Users paste full request URLs from provider docs (".../v1/chat/completions");
// appending the request path to those would double it, so every suffix Daygo
// itself appends is stripped here. base-only endpoints pass through untouched.
func normalizeTestEndpoint(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", daygoai.NewError(daygoai.ErrorInvalidRequest, "endpoint is required", 0, nil)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return "", daygoai.NewError(daygoai.ErrorInvalidRequest, "endpoint is not an absolute URL", 0, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", daygoai.NewError(daygoai.ErrorInvalidRequest, "endpoint scheme must be http or https", 0, nil)
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.RawFragment = ""
	parsed.ForceQuery = false
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = strings.TrimRight(parsed.RawPath, "/")
	for _, suffix := range endpointPathSuffixes {
		if strings.HasSuffix(parsed.Path, suffix) {
			parsed.Path = strings.TrimSuffix(parsed.Path, suffix)
			parsed.RawPath = strings.TrimSuffix(parsed.RawPath, suffix)
			break
		}
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

// endpointPathSuffixes are the request paths Daygo appends to a provider base
// endpoint. normalizeTestEndpoint strips one of them when the user pasted a
// full request URL instead of the base.
var endpointPathSuffixes = []string{
	"/chat/completions",
	"/responses",
	"/completions",
	"/messages",
	"/models",
}
