//go:build !darwin && !windows

package factory

import (
	"context"

	"github.com/Jwz-git/Daygo/internal/platform"
)

func NewCapture() platform.Capture {
	return unavailableCapture{}
}

type unavailableCapture struct{}

func (unavailableCapture) Capture(ctx context.Context, req platform.CaptureRequest) (platform.CaptureResult, error) {
	return platform.CaptureResult{}, &platform.CaptureError{Code: platform.CaptureUnsupported}
}
