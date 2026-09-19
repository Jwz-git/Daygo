//go:build windows

package windows

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/Jwz-git/Daygo/internal/platform"
	"golang.org/x/sys/windows/registry"
)

// launchRunValueName is this app's value under the per-user Run key. It is part
// of the Windows launch identity: once Windows ships, renaming it orphans a
// user's existing auto-start entry, so it belongs with the frozen identity set
// (AGENTS.md 身份标识, Windows values still TBD).
const (
	launchRunKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	launchRunValueName = "Daygo"
)

// LaunchAtLogin reports whether the current user's Run key auto-starts Daygo.
// A missing key or value is "disabled", not an error.
func (*System) LaunchAtLogin(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, launchRunKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("windows launch-at-login: open run key: %w", err)
	}
	defer key.Close()

	if _, _, err := key.GetStringValue(launchRunValueName); err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("windows launch-at-login: read value: %w", err)
	}
	return true, nil
}

// SetLaunchAtLogin adds or removes the per-user Run entry. Enabling records the
// current executable's quoted path; disabling deletes the value and treats an
// already-absent value as success.
func (*System) SetLaunchAtLogin(ctx context.Context, enabled bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, launchRunKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("windows launch-at-login: open run key: %w", err)
	}
	defer key.Close()

	if !enabled {
		if err := key.DeleteValue(launchRunValueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return fmt.Errorf("windows launch-at-login: clear value: %w", err)
		}
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("windows launch-at-login: resolve executable: %w", err)
	}
	// Quote the path so a Program Files install with spaces still launches.
	if err := key.SetStringValue(launchRunValueName, `"`+exe+`"`); err != nil {
		return fmt.Errorf("windows launch-at-login: write value: %w", err)
	}
	return nil
}

// OpenSystemSettings opens the closed set of Settings pages the app may deep-link
// to. explorer.exe resolves ms-settings: URIs; the process is started rather than
// waited on because explorer's exit code is unreliable and the Settings app runs
// independently of Daygo.
func (*System) OpenSystemSettings(ctx context.Context, pane platform.SettingsPane) error {
	uri, ok := settingsPaneURI(pane)
	if !ok {
		return errSystemCapabilityUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return exec.CommandContext(ctx, "explorer.exe", uri).Start()
}

// RevealPath opens a folder in Explorer. explorer.exe returns a nonzero exit
// code even on success, so the process is started rather than waited on, the
// same way OpenSystemSettings treats it.
func (*System) RevealPath(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return exec.CommandContext(ctx, "explorer.exe", path).Start()
}

// settingsPaneURI maps a cross-platform pane to its ms-settings: URI. Screen
// recording has no Windows equivalent (desktop capture is not TCC-gated, see
// ScreenRecordingPermission), so it reports unavailable rather than guessing a
// pane.
func settingsPaneURI(pane platform.SettingsPane) (string, bool) {
	switch pane {
	case platform.PaneNotifications:
		return "ms-settings:notifications", true
	case platform.PaneLoginItems:
		return "ms-settings:startupapps", true
	default:
		return "", false
	}
}
