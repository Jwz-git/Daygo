package app

import (
	"strings"
	"unicode/utf8"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
)

type dockShortcutPresenter interface {
	DockShortcutAvailable() bool
	ShowDockShortcut(title, hint string) error
}

// ShowDockShortcut opens the native drag source for Daygo's own app bundle.
func (b *Backend) ShowDockShortcut(title, hint string) error {
	presenter, ok := b.system.(dockShortcutPresenter)
	if !ok || !presenter.DockShortcutAvailable() {
		return apperr.E(apperr.NativeUnavailable, "dock shortcut unavailable", nil)
	}
	for _, label := range []string{title, hint} {
		if len(label) > 512 || !utf8.ValidString(label) || strings.ContainsRune(label, 0) || strings.TrimSpace(label) == "" {
			return apperr.E(apperr.InvalidArgument, "invalid dock shortcut copy", nil)
		}
	}
	if err := presenter.ShowDockShortcut(title, hint); err != nil {
		return apperr.E(apperr.NativeUnavailable, "dock shortcut could not open", err)
	}
	return nil
}
