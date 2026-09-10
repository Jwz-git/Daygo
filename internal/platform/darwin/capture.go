//go:build darwin

package darwin

import (
	"context"

	"github.com/Jwz-git/Daygo/internal/platform"
)

var _ platform.Capture = (*Capture)(nil)

// Capture is the stateless macOS adapter for one primary-display screenshot.
type Capture struct{}

func NewCapture() *Capture {
	return &Capture{}
}

func (c *Capture) Capture(ctx context.Context, req platform.CaptureRequest) (platform.CaptureResult, error) {
	if err := ctx.Err(); err != nil {
		return platform.CaptureResult{}, err
	}
	if err := req.Validate(); err != nil {
		return platform.CaptureResult{}, err
	}
	return captureOnce(ctx, req)
}
