package ai

import (
	"net/url"
	"strings"
)

// OpenAIBaseURL normalizes Chat Completions, Responses and model listing to
// the same base. Version and gateway prefixes belong to the configured URL:
// do not guess /v1, since compatible providers can expose unversioned routes.
// Normalize at send time too so previously stored pasted URLs keep working.
func OpenAIBaseURL(endpoint string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") {
		return "", NewError(ErrorInvalidRequest, "provider endpoint must be an absolute HTTP URL", 0, err)
	}
	base.RawQuery, base.Fragment, base.RawFragment = "", "", ""
	base.ForceQuery = false
	base.Path = strings.TrimRight(base.Path, "/")
	base.RawPath = strings.TrimRight(base.RawPath, "/")
	for _, suffix := range []string{"/chat/completions", "/responses", "/completions", "/models"} {
		if strings.HasSuffix(base.Path, suffix) {
			base.Path = strings.TrimSuffix(base.Path, suffix)
			base.RawPath = strings.TrimSuffix(base.RawPath, suffix)
			break
		}
	}
	return strings.TrimRight(base.String(), "/") + "/", nil
}
