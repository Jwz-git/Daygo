package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/fake"
)

func TestCaptureTestWritesConfiguredJPEG(t *testing.T) {
	dir := t.TempDir()
	capture := fake.NewCapture()
	backend := newBackend(systemClock{}, nil, nil, false, false)
	backend.setCapture(capture)

	result, err := backend.CaptureTest(CaptureTestRequestDTO{
		OutputDirectory: dir,
		FilenamePrefix:  "abi-smoke",
		TargetHeight:    18,
		JPEGQuality:     82,
	})
	if err != nil {
		t.Fatalf("CaptureTest: %v", err)
	}
	if result.Outcome != string(platform.CaptureWritten) {
		t.Fatalf("outcome = %q", result.Outcome)
	}
	if filepath.Dir(result.OutputPath) != dir || filepath.Ext(result.OutputPath) != ".jpg" {
		t.Fatalf("output path = %q", result.OutputPath)
	}
	info, err := os.Stat(result.OutputPath)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if info.Size() != result.FileSize || result.Width != 32 || result.Height != 18 {
		t.Fatalf("result = %+v, file size = %d", result, info.Size())
	}
}

func TestCaptureTestReportsPrivacyBlockWithoutFile(t *testing.T) {
	dir := t.TempDir()
	capture := fake.NewCapture()
	capture.SetFrontmostApplicationID("com.example.private")
	backend := newBackend(systemClock{}, nil, nil, false, false)
	backend.setCapture(capture)

	result, err := backend.CaptureTest(CaptureTestRequestDTO{
		OutputDirectory:       dir,
		FilenamePrefix:        "blocked",
		TargetHeight:          18,
		JPEGQuality:           82,
		BlockedApplicationIDs: []string{"com.example.private"},
	})
	if err != nil {
		t.Fatalf("CaptureTest: %v", err)
	}
	if result.Outcome != string(platform.CaptureBlocked) {
		t.Fatalf("outcome = %q", result.Outcome)
	}
	if _, err := os.Stat(result.OutputPath); !os.IsNotExist(err) {
		t.Fatalf("blocked output stat error = %v, want not-exist", err)
	}
}
