//go:build !windows

package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenReportsErrorsFromUnwritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses directory permissions")
	}
	parent := newDir(t)
	dir := filepath.Join(parent, "readonly")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	_, err := Open(context.Background(), Options{Dir: dir})
	if err == nil {
		t.Skip("directory unexpectedly writable; permission semantics differ on this filesystem")
	}
	if IsCorrupt(err) {
		t.Fatalf("permission failure classified as corruption: %v", err)
	}
}
