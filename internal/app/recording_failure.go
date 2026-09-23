package app

import (
	"errors"
	"fmt"
	"os"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// recordingFailureReason exposes a stable, non-identifying diagnostic. Never
// use Error() here: native and storage errors can include local file paths.
func recordingFailureReason(err error) string {
	var captureErr *platform.CaptureError
	if errors.As(err, &captureErr) {
		if captureErr.NativeCode != 0 {
			return fmt.Sprintf("capture_%s:0x%x", captureErr.Code, uint32(captureErr.NativeCode))
		}
		return "capture_" + string(captureErr.Code)
	}
	if kind, ok := storage.KindOf(err); ok {
		return "storage_" + string(kind)
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return "filesystem"
	}
	return "unknown"
}
