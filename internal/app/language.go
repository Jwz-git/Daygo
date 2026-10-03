package app

import "github.com/Jwz-git/Daygo/internal/settings"

// resolveInterfaceLanguage returns the BCP 47 tag to hand to the model layer.
//
// Model output language follows the interface language, with no separate
// setting (docs/03 §3.3.5). The stored appearance.language keeps "" as its
// "follow the system" sentinel, but the chat prompt and the analysis prompt
// render an empty tag as a weak "match the user's message" instruction — and
// since the prompt skeleton is single-language English, the model then
// defaults to English on a Chinese interface. Resolve the sentinel here, at
// the adapter edge, so the prompt always carries a concrete BCP 47 tag.
func resolveInterfaceLanguage(snapshot settings.Snapshot) string {
	if snapshot.Language != "" {
		return snapshot.Language
	}
	// normalizeLanguage preserves "" as the "follow the system" sentinel, so
	// the empty branch is reachable; Load's own fallback (DefaultLanguage)
	// covers an unreadable store, and this keeps the prompt non-empty either
	// way.
	return settings.DefaultLanguage
}
