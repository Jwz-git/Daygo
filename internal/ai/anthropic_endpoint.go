package ai

import (
	"net/url"
	"strings"
)

// AnthropicBaseURL returns the unversioned base shared by Messages and model
// listing. Accept existing /v1 bases and pasted request URLs without appending
// a second version prefix; preserve gateway paths before that prefix.
func AnthropicBaseURL(endpoint string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") {
		return "", NewError(ErrorInvalidRequest, "provider endpoint must be an absolute HTTP URL", 0, err)
	}
	base.RawQuery, base.Fragment, base.RawFragment = "", "", ""
	base.ForceQuery = false
	base.Path = strings.TrimRight(base.Path, "/")
	base.RawPath = strings.TrimRight(base.RawPath, "/")
	for _, suffix := range []string{"/messages", "/models"} {
		if strings.HasSuffix(base.Path, suffix) {
			base.Path = strings.TrimSuffix(base.Path, suffix)
			base.RawPath = strings.TrimSuffix(base.RawPath, suffix)
			break
		}
	}
	base.Path = strings.TrimSuffix(base.Path, "/v1")
	base.RawPath = strings.TrimSuffix(base.RawPath, "/v1")
	return strings.TrimRight(base.String(), "/") + "/", nil
}
