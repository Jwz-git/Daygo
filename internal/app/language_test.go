package app

import (
	"testing"

	"github.com/Jwz-git/Daygo/internal/settings"
)

// resolveInterfaceLanguage is the adapter's single source of truth for what
// BCP 47 tag to hand the chat / analysis prompts. Model output language
// follows the interface language, so the interface tag is what the model
// gets. The prompts render an empty string as a weak "match the user's
// message" instruction; since the skeleton is single-language English, that
// means English output on a Chinese interface.
func TestResolveInterfaceLanguage_FollowsInterfaceLanguage(t *testing.T) {
	for _, language := range []string{"zh-CN", "zh-Hant", "ja", "ko", "en", "de", "fr", "es", "pt-BR"} {
		got := resolveInterfaceLanguage(settings.Snapshot{Language: language})
		if got != language {
			t.Errorf("language=%q: got %q, want %q", language, got, language)
		}
	}
}

// The "follow the system" sentinel ("") must never escape to the prompt
// layer: an empty tag renders as the weak "match the user's message" fallback
// and the LLM defaults to English on the single-language skeleton. This is a
// regression guard for the bug where chat replies and timeline cards came out
// in English on a Chinese interface even though the UI was Chinese.
func TestResolveInterfaceLanguage_NeverEmpty(t *testing.T) {
	for _, language := range []string{"", settings.DefaultLanguage} {
		if got := resolveInterfaceLanguage(settings.Snapshot{Language: language}); got == "" {
			t.Fatalf("resolveInterfaceLanguage(%q) returned empty", language)
		}
	}
}
