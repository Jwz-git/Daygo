//go:build darwin

package darwin

import (
	"context"

	"github.com/Jwz-git/Daygo/internal/platform"
)

var _ platform.Capture = (*Capture)(nil)
var _ platform.CapturePrivacyReporter = (*Capture)(nil)

// Capture is the stateless macOS adapter for one primary-display screenshot.
type Capture struct{}

func NewCapture() *Capture {
	return &Capture{}
}

func (c *Capture) CapturePrivacyCompatibility(ctx context.Context) (platform.CapturePrivacyCompatibility, error) {
	if err := ctx.Err(); err != nil {
		return platform.CapturePrivacyCompatibility{}, err
	}
	return platform.CapturePrivacyCompatibility{Platform: "darwin", Supported: true}, nil
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
