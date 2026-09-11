package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform"
)

const captureTestDefaultTimeout = 15 * time.Second

// CaptureTestRequestDTO is temporary test-surface configuration. It is not
// persisted and does not participate in recorder settings.
type CaptureTestRequestDTO struct {
	OutputDirectory       string   `json:"outputDirectory"`
	FilenamePrefix        string   `json:"filenamePrefix"`
	TargetHeight          int      `json:"targetHeight"`
	JPEGQuality           int      `json:"jpegQuality"`
	ShowsCursor           bool     `json:"showsCursor"`
	BlockedApplicationIDs []string `json:"blockedApplicationIds"`
}

// CaptureTestResultDTO reports the file produced by one direct Capture call.
type CaptureTestResultDTO struct {
	Outcome      string `json:"outcome"`
	OutputPath   string `json:"outputPath"`
	CapturedAtTs int64  `json:"capturedAtTs"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	FileSize     int64  `json:"fileSize"`
}

// CaptureTest invokes the platform Capture port once. It deliberately bypasses
// recorder and persistence so native ABI work can be exercised from the UI.
func (b *Backend) CaptureTest(request CaptureTestRequestDTO) (CaptureTestResultDTO, error) {
	if b.capture == nil {
		return CaptureTestResultDTO{}, apperr.E(apperr.NativeUnavailable, "capture services are unavailable", nil)
	}
	if err := validateCaptureTestRequest(request); err != nil {
		return CaptureTestResultDTO{}, err
	}

	b.captureMu.Lock()
	defer b.captureMu.Unlock()

	if err := os.MkdirAll(request.OutputDirectory, 0o700); err != nil {
		return CaptureTestResultDTO{}, apperr.E(apperr.InvalidArgument, "output directory is unavailable", err)
	}
	outputPath := nextCaptureTestPath(request.OutputDirectory, request.FilenamePrefix)

	ctx, cancel := context.WithTimeout(context.Background(), captureTestDefaultTimeout)
	defer cancel()
	result, err := b.capture.Capture(ctx, platform.CaptureRequest{
		OutputPath:            outputPath,
		ImageFormat:           platform.CaptureImageJPEG,
		TargetHeight:          request.TargetHeight,
		JPEGQuality:           request.JPEGQuality,
		ShowsCursor:           request.ShowsCursor,
		BlockedApplicationIDs: append([]string(nil), request.BlockedApplicationIDs...),
	})
	if err != nil {
		return CaptureTestResultDTO{}, captureTestError(err)
	}

	response := CaptureTestResultDTO{Outcome: string(result.Outcome), OutputPath: outputPath}
	if result.Outcome == platform.CaptureWritten {
		response.CapturedAtTs = result.CapturedAt.Unix()
		response.Width = result.Width
		response.Height = result.Height
		response.FileSize = result.FileSize
	}
	return response, nil
}

// OpenCaptureTestFolder opens the parent directory of a path returned by
// CaptureTest. It is kept separate from arbitrary URL opening for safety.
func (b *Backend) OpenCaptureTestFolder(path string) error {
	if !filepath.IsAbs(path) || !utf8.ValidString(path) {
		return apperr.E(apperr.InvalidArgument, "capture path must be absolute", nil)
	}
	directory := filepath.Dir(path)
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return apperr.E(apperr.NotFound, "capture directory is unavailable", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := platform.OpenFolder(ctx, directory); err != nil {
		return apperr.E(apperr.NativeUnavailable, "could not open capture directory", err)
	}
	return nil
}

func validateCaptureTestRequest(request CaptureTestRequestDTO) error {
	if !filepath.IsAbs(request.OutputDirectory) || !utf8.ValidString(request.OutputDirectory) || len(request.OutputDirectory) > 32768 {
		return apperr.E(apperr.InvalidArgument, "output directory must be an absolute path", nil)
	}
	if request.TargetHeight < 1 || request.TargetHeight > 16384 || request.JPEGQuality < 1 || request.JPEGQuality > 100 {
		return apperr.E(apperr.InvalidArgument, "image settings are outside supported bounds", nil)
	}
	if request.FilenamePrefix == "" {
		return apperr.E(apperr.InvalidArgument, "filename prefix is required", nil)
	}
	if len(request.FilenamePrefix) > 64 || filepath.Base(request.FilenamePrefix) != request.FilenamePrefix || strings.ContainsAny(request.FilenamePrefix, `/\\`) || !utf8.ValidString(request.FilenamePrefix) {
		return apperr.E(apperr.InvalidArgument, "filename prefix is invalid", nil)
	}
	if len(request.BlockedApplicationIDs) > 4096 {
		return apperr.E(apperr.InvalidArgument, "blocked application list is too large", nil)
	}
	return nil
}

func nextCaptureTestPath(directory, prefix string) string {
	base := prefix + "-" + time.Now().UTC().Format("20060102-150405.000000000")
	for suffix := 0; ; suffix++ {
		name := base + ".jpg"
		if suffix > 0 {
			name = base + "-" + strconv.Itoa(suffix) + ".jpg"
		}
		candidate := filepath.Join(directory, name)
		if _, err := os.Stat(candidate); err != nil {
			return candidate
		}
	}
}

func captureTestError(err error) error {
	if captureErr, ok := err.(*platform.CaptureError); ok {
		code := apperr.NativeUnavailable
		switch captureErr.Code {
		case platform.CaptureInvalidArgument:
			code = apperr.InvalidArgument
		case platform.CapturePermissionDenied:
			code = apperr.PermissionDenied
		}
		return apperr.E(code, "screenshot capture failed", err)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return apperr.E(apperr.Canceled, "screenshot capture was canceled", err)
	}
	return apperr.E(apperr.NativeUnavailable, "screenshot capture failed", err)
}
