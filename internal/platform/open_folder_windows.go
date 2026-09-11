//go:build windows

package platform

import (
	"context"
	"os/exec"
)

func OpenFolder(ctx context.Context, path string) error {
	return exec.CommandContext(ctx, "explorer.exe", path).Run()
}
