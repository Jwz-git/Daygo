package app

import "github.com/Jwz-git/Daygo/internal/settings"

// resolveOutputLanguage returns the language tag to hand to the model layer.
//
// The stored setting uses "" as a sentinel for "follow the interface language"
// (docs/05 §5.3.3). That sentinel is meaningful to the UI, but the chat prompt
// and the analysis prompt render it as a weak "match the user's message"
// instruction — and since the prompt skeleton is single-language English, the
// model defaults to English output. Resolve the sentinel here, at the
// adapter edge, so the prompt always carries a concrete BCP 47 tag.
//
// The interface language is guaranteed non-empty by settings.normalizeLanguage
// (it folds anything unrecognised onto DefaultLanguage, "zh-CN"). Even an
// unreadable settings store yields the snapshot default rather than "".
func resolveOutputLanguage(snapshot settings.Snapshot) string {
	if snapshot.OutputLanguage != "" {
		return snapshot.OutputLanguage
	}
	if snapshot.Language != "" {
		return snapshot.Language
	}
	// settings.normalizeLanguage prevents an empty Language for any value
	// it sees, and Load's default is "zh-CN" (also non-empty). The empty
	// branch is unreachable in production; fall back to the system default
	// so the prompt never carries the weak "match the user" instruction.
	return settings.DefaultLanguage
}

// interfaceLanguage is the same resolution for the error path: the settings
// store could not be loaded, so we don't have a snapshot, only the caller
// knows the configured default ("zh-CN"). Exposed as a Backend method so the
// error fallback can stay in one place if the default ever changes.
func (b *Backend) interfaceLanguage(_ settings.Snapshot) string {
	// The snapshot is intentionally ignored: when Load fails we have no
	// authoritative source for the user's pick, and the system default is
	// the right guess for the prompt. Keep the parameter so the call site
	// stays symmetric with resolveOutputLanguage.
	return settings.DefaultLanguage
}
