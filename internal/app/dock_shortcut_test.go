package app

import (
	"errors"
	"slices"
	"testing"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform/fake"
)

type dockShortcutFixture struct {
	*fake.System
	available bool
	title     string
	hint      string
}

func (s *dockShortcutFixture) DockShortcutAvailable() bool { return s.available }
func (s *dockShortcutFixture) ShowDockShortcut(title, hint string) error {
	s.title, s.hint = title, hint
	return nil
}

func TestShowDockShortcutUsesOnlyAvailableNativeHost(t *testing.T) {
	system := &dockShortcutFixture{System: fake.NewSystem()}
	b := NewBackend(system, nil)
	if slices.Contains(b.features(), "dock_shortcut") {
		t.Fatal("an unbundled host must not advertise a Dock shortcut")
	}
	var err *apperr.Error
	if callErr := b.ShowDockShortcut("Fixture", "Drag the icon"); !errors.As(callErr, &err) || err.Code != apperr.NativeUnavailable {
		t.Fatalf("unavailable shortcut = %v", callErr)
	}

	system.available = true
	if !slices.Contains(b.features(), "dock_shortcut") {
		t.Fatal("bundled host must advertise the native Dock shortcut")
	}
	if callErr := b.ShowDockShortcut("Fixture", ""); !errors.As(callErr, &err) || err.Code != apperr.InvalidArgument {
		t.Fatalf("invalid shortcut copy = %v", callErr)
	}
	if system.title != "" {
		t.Fatal("invalid copy must not reach the presenter")
	}
	if callErr := b.ShowDockShortcut("Fixture", "Drag the icon"); callErr != nil {
		t.Fatal(callErr)
	}
	if system.title != "Fixture" || system.hint != "Drag the icon" {
		t.Fatalf("presented %q, %q", system.title, system.hint)
	}
}
