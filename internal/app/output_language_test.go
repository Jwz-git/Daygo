package app

import (
	"testing"

	"github.com/Jwz-git/Daygo/internal/settings"
)

// resolveOutputLanguage is the adapter's single source of truth for what
// BCP 47 tag to hand the chat / analysis prompts. The chat prompt and the
// analysis prompt both render an empty string as a weak "match the user's
// message" instruction; since the prompt skeleton is single-language English,
// that falls back to English output on a Chinese interface. Resolving the
// "follow the interface language" sentinel here, at the adapter edge, fixes
// that without changing the stored setting's documented semantics.
func TestResolveOutputLanguage_PinnedHonoured(t *testing.T) {
	for _, pinned := range []string{"zh-CN", "en", "zh-CN"} {
		got := resolveOutputLanguage(settings.Snapshot{
			OutputLanguage: pinned,
			Language:       "en",
		})
		if got != pinned {
			t.Errorf("pinned %q: got %q, want %q", pinned, got, pinned)
		}
	}
}

func TestResolveOutputLanguage_FollowsInterfaceLanguage(t *testing.T) {
	cases := []struct {
		interfaceLanguage string
		want              string
	}{
		{"zh-CN", "zh-CN"},
		{"zh-Hant", "zh-Hant"},
		{"ja", "ja"},
		{"ko", "ko"},
		{"en", "en"},
		{"de", "de"},
		{"fr", "fr"},
		{"es", "es"},
		{"pt-BR", "pt-BR"},
	}
	for _, c := range cases {
		got := resolveOutputLanguage(settings.Snapshot{
			OutputLanguage: "", // sentinel: follow the interface language
			Language:       c.interfaceLanguage,
		})
		if got != c.want {
			t.Errorf("interface=%q, empty output: got %q, want %q",
				c.interfaceLanguage, got, c.want)
		}
	}
}

// Empty string must never escape to the prompt layer: an empty tag renders
// as the weak "match the user's message" fallback and the LLM defaults to
// English on the single-language skeleton. This is a regression guard for
// the bug where chat replies and timeline cards came out in English on a
// Chinese interface even though the UI was Chinese.
func TestResolveOutputLanguage_NeverEmpty(t *testing.T) {
	// Even the most degenerate snapshot (both fields empty, which
	// normalizeLanguage prevents in practice) must not yield "".
	if got := resolveOutputLanguage(settings.Snapshot{}); got == "" {
		t.Fatal("resolveOutputLanguage returned empty for empty snapshot")
	}
}
