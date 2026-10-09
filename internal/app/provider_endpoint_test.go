package app

import "testing"

func TestNormalizeTestEndpointStripsRequestPathSuffix(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"base only", "https://example.com/v1", "https://example.com/v1"},
		{"pasted chat completions url", "https://example.com/v1/chat/completions", "https://example.com/v1"},
		{"pasted chat completions url with slash", "https://example.com/v1/chat/completions/", "https://example.com/v1"},
		{"gateway request url with slashes", "https://example.com/gateway/v1/chat/completions///?x=1#frag", "https://example.com/gateway/v1"},
		{"pasted responses url with slash", "https://example.com/v1/responses/", "https://example.com/v1"},
		{"pasted responses url", "https://example.com/v1/responses", "https://example.com/v1"},
		{"pasted messages url", "https://example.com/v1/messages", "https://example.com/v1"},
		{"pasted models url", "https://example.com/v1/models", "https://example.com/v1"},
		{"trailing slash kept out", "https://example.com/v1/", "https://example.com/v1"},
		{"query and fragment stripped", "https://example.com/v1/chat/completions?x=1#frag", "https://example.com/v1"},
		{"unrelated path untouched", "https://example.com/gateway/openai", "https://example.com/gateway/openai"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := normalizeTestEndpoint(testCase.raw)
			if err != nil {
				t.Fatalf("normalizeTestEndpoint(%q): %v", testCase.raw, err)
			}
			if got != testCase.want {
				t.Fatalf("normalizeTestEndpoint(%q) = %q, want %q", testCase.raw, got, testCase.want)
			}
		})
	}
}
