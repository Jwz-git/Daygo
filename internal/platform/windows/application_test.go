//go:build windows && cgo

package windows

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestApplicationInspectorRoundTrip(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dll := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "build", "native", "windows", "amd64", "daygo_windows_native.dll"))
	if _, err := os.Stat(dll); err != nil {
		if os.IsNotExist(err) {
			t.Skip("native privacy helper is unavailable; install Windows SDK 26100 and run native/windows/build.ps1 -RequirePrivacyAdapter")
		}
		t.Fatalf("stat native helper: %v", err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	testDLL := filepath.Join(filepath.Dir(executable), "daygo_windows_native.dll")
	copyFile(t, dll, testDLL)
	t.Cleanup(func() { _ = os.Remove(testDLL) })
	inspector := NewApplicationInspector()
	identity, err := inspector.InspectApplication(context.Background(), executable)
	if err != nil {
		t.Fatalf("InspectApplication: %v", err)
	}
	if !strings.HasPrefix(identity.ID, "win32.exe.sha256:") || identity.Name == "" {
		t.Fatalf("identity = %#v", identity)
	}
	if len(identity.IconPNG) > 0 && !strings.HasPrefix(string(identity.IconPNG), "\x89PNG\r\n\x1a\n") {
		t.Fatalf("icon is not PNG: %x", identity.IconPNG[:min(8, len(identity.IconPNG))])
	}

	described, err := inspector.DescribeApplications(context.Background(), []string{identity.ID})
	if err != nil {
		t.Fatalf("DescribeApplications: %v", err)
	}
	if len(described) != 1 || described[0].ID != identity.ID || described[0].Name != identity.Name {
		t.Fatalf("described = %#v, want %#v", described, identity)
	}
}

func copyFile(t *testing.T, source, destination string) {
	t.Helper()
	input, err := os.Open(source)
	if err != nil {
		t.Fatalf("open native helper: %v", err)
	}
	defer input.Close()
	output, err := os.Create(destination)
	if err != nil {
		t.Fatalf("create native helper copy: %v", err)
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		t.Fatalf("copy native helper: %v", err)
	}
	if err := output.Close(); err != nil {
		t.Fatalf("close native helper copy: %v", err)
	}
}
