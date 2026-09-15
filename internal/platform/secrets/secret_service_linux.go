//go:build linux

package secrets

import (
	"os/exec"

	"github.com/Jwz-git/Daygo/internal/platform"
)

// New returns the freedesktop Secret Service implementation used on Linux.
// A missing secret-tool binary is represented by the same object and reported
// as SecretUnsupported on use, so application startup remains available.
func New() platform.Secrets {
	path, _ := exec.LookPath("secret-tool")
	return newSecretService(path, execSecretTool{})
}
