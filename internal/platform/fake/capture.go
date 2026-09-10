package fake

import (
	"context"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Jwz-git/Daygo/internal/platform"
)

var (
	_         platform.Capture = (*Capture)(nil)
	fakeEpoch                  = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
)

// Capture is a deterministic file-writing implementation of platform.Capture.
// Its controls exist only so contract tests can drive OS outcomes.
type Capture struct {
	mu                     sync.RWMutex
	permission             platform.PermissionState
	frontmostApplicationID string
	noDisplay              bool
}

func NewCapture() *Capture {
	return &Capture{permission: platform.PermissionGranted}
}

// SetPermission controls the authorization observed by the fake.
func (c *Capture) SetPermission(state platform.PermissionState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.permission = state
}

// SetFrontmostApplicationID controls the application inspected by the fake's
// capture-time privacy guard. An empty value means no identified application.
func (c *Capture) SetFrontmostApplicationID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.frontmostApplicationID = id
}

// SetNoDisplay controls whether the fake has a primary display.
func (c *Capture) SetNoDisplay(noDisplay bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.noDisplay = noDisplay
}

func (c *Capture) Capture(ctx context.Context, req platform.CaptureRequest) (platform.CaptureResult, error) {
	if err := ctx.Err(); err != nil {
		return platform.CaptureResult{}, err
	}
	if !validRequest(req) {
		return platform.CaptureResult{}, captureError(platform.CaptureInvalidArgument)
	}

	c.mu.RLock()
	permission := c.permission
	frontmostApplicationID := c.frontmostApplicationID
	noDisplay := c.noDisplay
	c.mu.RUnlock()

	if permission != platform.PermissionGranted {
		return platform.CaptureResult{}, captureError(platform.CapturePermissionDenied)
	}
	if noDisplay {
		return platform.CaptureResult{}, captureError(platform.CaptureNoDisplay)
	}
	if blocked(frontmostApplicationID, req.BlockedApplicationIDs) {
		return platform.CaptureResult{Outcome: platform.CaptureBlocked}, nil
	}
	if err := ctx.Err(); err != nil {
		return platform.CaptureResult{}, err
	}

	width := req.TargetHeight * 16 / 9
	if width == 0 {
		width = 1
	}
	fileSize, err := writeJPEG(ctx, req.OutputPath, width, req.TargetHeight, req.JPEGQuality)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return platform.CaptureResult{}, ctxErr
		}
		return platform.CaptureResult{}, captureError(platform.CaptureIO)
	}
	return platform.CaptureResult{
		Outcome:    platform.CaptureWritten,
		CapturedAt: fakeEpoch,
		Width:      width,
		Height:     req.TargetHeight,
		FileSize:   fileSize,
	}, nil
}

func validRequest(req platform.CaptureRequest) bool {
	if !filepath.IsAbs(req.OutputPath) || len(req.OutputPath) > 32768 || !utf8.ValidString(req.OutputPath) {
		return false
	}
	if !req.ImageFormat.Valid() || req.TargetHeight < 1 || req.TargetHeight > 16384 {
		return false
	}
	if req.JPEGQuality < 1 || req.JPEGQuality > 100 || len(req.BlockedApplicationIDs) > 4096 {
		return false
	}
	totalIDBytes := 0
	for _, id := range req.BlockedApplicationIDs {
		if id == "" || len(id) > 4096 || !utf8.ValidString(id) {
			return false
		}
		totalIDBytes += len(id)
		if totalIDBytes > 1<<20 {
			return false
		}
	}
	return true
}

func blocked(frontmostApplicationID string, blockedApplicationIDs []string) bool {
	if frontmostApplicationID == "" {
		return false
	}
	for _, id := range blockedApplicationIDs {
		if id == frontmostApplicationID {
			return true
		}
	}
	return false
}

func writeJPEG(ctx context.Context, outputPath string, width, height, quality int) (int64, error) {
	if _, err := os.Lstat(outputPath); err == nil || !os.IsNotExist(err) {
		return 0, os.ErrExist
	}

	directory := filepath.Dir(outputPath)
	file, err := os.CreateTemp(directory, "."+filepath.Base(outputPath)+".daygo-*.partial")
	if err != nil {
		return 0, err
	}
	temporaryPath := file.Name()
	defer os.Remove(temporaryPath)

	encoded := false
	defer func() {
		if !encoded {
			_ = file.Close()
		}
	}()

	if err := jpeg.Encode(file, image.NewRGBA(image.Rect(0, 0, width, height)), &jpeg.Options{Quality: quality}); err != nil {
		return 0, err
	}
	if err := file.Close(); err != nil {
		return 0, err
	}
	encoded = true
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	// Linking a completed sibling file publishes without replacing an existing
	// destination. Removing the temporary link leaves the final file intact.
	if err := os.Link(temporaryPath, outputPath); err != nil {
		return 0, err
	}
	if err := os.Remove(temporaryPath); err != nil {
		_ = os.Remove(outputPath)
		return 0, err
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		_ = os.Remove(outputPath)
		return 0, err
	}
	return info.Size(), nil
}

func captureError(code platform.CaptureErrorCode) error {
	return &platform.CaptureError{Code: code}
}
