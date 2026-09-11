//go:build darwin

package platform

import (
	"context"
	"os/exec"
)

func OpenFolder(ctx context.Context, path string) error {
	return exec.CommandContext(ctx, "open", path).Run()
}
