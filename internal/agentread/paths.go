package agentread

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Jwz-git/Daygo/internal/storage"
)

// applicationSupportDirName is the directory under the user's config dir that
// holds every Daygo file. It is part of the published identity and must match
// app.ApplicationSupportDirName and AGENTS.md "身份标识"; it is duplicated here
// rather than imported so this package never depends on internal/app (Wails).
const applicationSupportDirName = "Daygo"

// databasePathEnv overrides the resolved database path (docs/05 §5.9.1). It
// points at the database file itself, so a fixture at any path can be read.
const databasePathEnv = "DAYGO_DB"

// socketFileName is the agent.sock file name in the support dir (AGENTS.md
// identity, docs/03 §3.1).
const socketFileName = "agent.sock"

// SupportDir returns the application support directory
// (~/Library/Application Support/Daygo on macOS), the same path the daemon
// resolves in app.supportDir.
func SupportDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate application support directory: %w", err)
	}
	return filepath.Join(base, applicationSupportDirName), nil
}

// DatabasePath resolves the database file: the DAYGO_DB override when set,
// otherwise daygo.sqlite in the support directory.
func DatabasePath() (string, error) {
	if override := os.Getenv(databasePathEnv); override != "" {
		return override, nil
	}
	dir, err := SupportDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, storage.DatabaseFileName), nil
}

// SocketPath resolves the agent.sock path in the support directory. The
// DAYGO_DB override moves the socket next to the overridden database so a
// fixture-backed CLI and its bridge agree on one location.
func SocketPath() (string, error) {
	if override := os.Getenv(databasePathEnv); override != "" {
		return filepath.Join(filepath.Dir(override), socketFileName), nil
	}
	dir, err := SupportDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, socketFileName), nil
}
