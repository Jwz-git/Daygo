//go:build windows

package windows

import (
	"context"
	"fmt"

	"github.com/Jwz-git/Daygo/internal/platform"
	xwindows "golang.org/x/sys/windows"
)

var _ platform.Capture = (*Capture)(nil)
var _ platform.CapturePrivacyReporter = (*Capture)(nil)
var _ platform.SegmentCloser = (*Capture)(nil)

const minimumPrivacyBuild = 26100

// Capture is the stateless Windows adapter for one primary-display screenshot.
type Capture struct{}

func NewCapture() *Capture { return &Capture{} }

func (c *Capture) Capture(ctx context.Context, req platform.CaptureRequest) (platform.CaptureResult, error) {
	if err := ctx.Err(); err != nil {
		return platform.CaptureResult{}, err
	}
	if err := req.Validate(); err != nil {
		return platform.CaptureResult{}, err
	}
	if req.SegmentDirectory != "" {
		return frameAppend(ctx, req)
	}
	return captureOnce(ctx, req)
}

func (c *Capture) CloseActiveSegment(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return segmentCloseActive()
}

func (c *Capture) CapturePrivacyCompatibility(ctx context.Context) (platform.CapturePrivacyCompatibility, error) {
	if err := ctx.Err(); err != nil {
		return platform.CapturePrivacyCompatibility{}, err
	}
	version := xwindows.RtlGetVersion()
	build := version.BuildNumber
	product := "Windows"
	if build >= 22000 {
		product = "Windows 11"
	}
	return platform.CapturePrivacyCompatibility{
		Platform:     "windows",
		Version:      fmt.Sprintf("%s %d.%d (build %d)", product, version.MajorVersion, version.MinorVersion, build),
		Build:        build,
		MinimumBuild: minimumPrivacyBuild,
		Supported:    build >= minimumPrivacyBuild,
	}, nil
}
