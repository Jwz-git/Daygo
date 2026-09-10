package platformtest

import (
	"context"
	"errors"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/Jwz-git/Daygo/internal/platform"
)

// Suite exercises behavior shared by every platform.Capture implementation.
func Suite(t *testing.T, newCapture func(t *testing.T) platform.Capture) {
	t.Helper()

	t.Run("writes one complete JPEG", func(t *testing.T) {
		capture := newCapture(t)
		req := request(t)
		result, err := capture.Capture(context.Background(), req)
		if err != nil {
			t.Fatalf("Capture: %v", err)
		}
		if result.Outcome != platform.CaptureWritten {
			t.Fatalf("Outcome = %q, want %q", result.Outcome, platform.CaptureWritten)
		}
		if result.Width <= 0 || result.Height != req.TargetHeight || result.FileSize <= 0 || result.CapturedAt.IsZero() {
			t.Fatalf("invalid written result: %#v", result)
		}
		file, err := os.Open(req.OutputPath)
		if err != nil {
			t.Fatalf("open output: %v", err)
		}
		config, decodeErr := jpeg.DecodeConfig(file)
		closeErr := file.Close()
		if decodeErr != nil {
			t.Fatalf("decode JPEG config: %v", decodeErr)
		}
		if closeErr != nil {
			t.Fatalf("close output: %v", closeErr)
		}
		if config.Width != result.Width || config.Height != result.Height {
			t.Fatalf("JPEG size = %dx%d, result = %dx%d", config.Width, config.Height, result.Width, result.Height)
		}
		info, err := os.Stat(req.OutputPath)
		if err != nil {
			t.Fatalf("stat output: %v", err)
		}
		if info.Size() != result.FileSize {
			t.Fatalf("file size = %d, result = %d", info.Size(), result.FileSize)
		}
	})

	t.Run("does not overwrite output", func(t *testing.T) {
		capture := newCapture(t)
		req := request(t)
		const original = "existing"
		if err := os.WriteFile(req.OutputPath, []byte(original), 0o600); err != nil {
			t.Fatalf("seed output: %v", err)
		}
		_, err := capture.Capture(context.Background(), req)
		requireCaptureError(t, err, platform.CaptureIO)
		contents, readErr := os.ReadFile(req.OutputPath)
		if readErr != nil {
			t.Fatalf("read output: %v", readErr)
		}
		if string(contents) != original {
			t.Fatalf("existing output changed to %q", contents)
		}
	})

	t.Run("honors cancellation before capture", func(t *testing.T) {
		capture := newCapture(t)
		req := request(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := capture.Capture(ctx, req)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Capture error = %v, want context.Canceled", err)
		}
		if _, statErr := os.Stat(req.OutputPath); !os.IsNotExist(statErr) {
			t.Fatalf("canceled capture created output: %v", statErr)
		}
	})

	t.Run("rejects invalid request", func(t *testing.T) {
		capture := newCapture(t)
		req := request(t)
		req.TargetHeight = 0
		_, err := capture.Capture(context.Background(), req)
		requireCaptureError(t, err, platform.CaptureInvalidArgument)
	})
}

// AuthorizedCapture lets a test control the permission observed by a fake.
type AuthorizedCapture interface {
	platform.Capture
	SetPermission(platform.PermissionState)
}

func SuitePermission(t *testing.T, newCapture func(t *testing.T) AuthorizedCapture) {
	t.Helper()
	capture := newCapture(t)
	capture.SetPermission(platform.PermissionDenied)
	req := request(t)
	_, err := capture.Capture(context.Background(), req)
	requireCaptureError(t, err, platform.CapturePermissionDenied)
	if _, statErr := os.Stat(req.OutputPath); !os.IsNotExist(statErr) {
		t.Fatalf("permission-denied capture created output: %v", statErr)
	}

	capture.SetPermission(platform.PermissionGranted)
	result, err := capture.Capture(context.Background(), req)
	if err != nil {
		t.Fatalf("Capture after grant: %v", err)
	}
	if result.Outcome != platform.CaptureWritten {
		t.Fatalf("Outcome after grant = %q", result.Outcome)
	}
}

// PrivacyCapture lets a test control the application inspected immediately
// before the fake would read screen pixels.
type PrivacyCapture interface {
	platform.Capture
	SetFrontmostApplicationID(string)
}

func SuitePrivacy(t *testing.T, newCapture func(t *testing.T) PrivacyCapture) {
	t.Helper()
	capture := newCapture(t)
	capture.SetFrontmostApplicationID("app.blocked")
	req := request(t)
	req.BlockedApplicationIDs = []string{"app.other", "app.blocked"}
	result, err := capture.Capture(context.Background(), req)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if result != (platform.CaptureResult{Outcome: platform.CaptureBlocked}) {
		t.Fatalf("blocked result = %#v", result)
	}
	if _, statErr := os.Stat(req.OutputPath); !os.IsNotExist(statErr) {
		t.Fatalf("blocked capture created output: %v", statErr)
	}
}

// DisplayCapture lets a test model a host without a primary display.
type DisplayCapture interface {
	platform.Capture
	SetNoDisplay(bool)
}

func SuiteNoDisplay(t *testing.T, newCapture func(t *testing.T) DisplayCapture) {
	t.Helper()
	capture := newCapture(t)
	capture.SetNoDisplay(true)
	req := request(t)
	_, err := capture.Capture(context.Background(), req)
	requireCaptureError(t, err, platform.CaptureNoDisplay)
	if _, statErr := os.Stat(req.OutputPath); !os.IsNotExist(statErr) {
		t.Fatalf("no-display capture created output: %v", statErr)
	}
}

func request(t *testing.T) platform.CaptureRequest {
	t.Helper()
	return platform.CaptureRequest{
		OutputPath:            filepath.Join(t.TempDir(), "capture.jpg"),
		ImageFormat:           platform.CaptureImageJPEG,
		TargetHeight:          72,
		JPEGQuality:           80,
		ShowsCursor:           true,
		BlockedApplicationIDs: []string{"app.blocked"},
	}
}

func requireCaptureError(t *testing.T, err error, code platform.CaptureErrorCode) {
	t.Helper()
	var captureErr *platform.CaptureError
	if !errors.As(err, &captureErr) {
		t.Fatalf("error = %v, want *platform.CaptureError", err)
	}
	if captureErr.Code != code {
		t.Fatalf("error code = %q, want %q", captureErr.Code, code)
	}
}
