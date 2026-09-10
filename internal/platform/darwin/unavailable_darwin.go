//go:build darwin && !cgo

package darwin

import (
	"context"

	"github.com/Jwz-git/Daygo/internal/platform"
)

func captureOnce(context.Context, platform.CaptureRequest) (platform.CaptureResult, error) {
	return platform.CaptureResult{}, &platform.CaptureError{Code: platform.CaptureUnsupported}
}
