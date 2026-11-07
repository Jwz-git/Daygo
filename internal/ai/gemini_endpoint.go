package ai

import (
	"net/url"
	"strings"
)

// GeminiBaseURL accepts a Gemini API base including its version, or its models URL.
func GeminiBaseURL(endpoint string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || base.Host == "" || base.User != nil || (base.Scheme != "http" && base.Scheme != "https") {
		return "", NewError(ErrorInvalidRequest, "Gemini endpoint must be an absolute HTTP URL", 0, nil)
	}
	base.RawQuery, base.Fragment, base.RawFragment = "", "", ""
	base.ForceQuery = false
	base.Path = strings.TrimRight(base.Path, "/")
	if strings.HasSuffix(base.Path, "/models") {
		base.Path = strings.TrimSuffix(base.Path, "/models")
		base.RawPath = strings.TrimSuffix(strings.TrimRight(base.RawPath, "/"), "/models")
	}
	if base.Path == "" {
		base.Path = "/v1beta"
	}
	return strings.TrimRight(base.String(), "/") + "/", nil
}
