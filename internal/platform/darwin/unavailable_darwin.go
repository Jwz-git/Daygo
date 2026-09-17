//go:build darwin && !cgo

package darwin

import (
	"context"

	"github.com/Jwz-git/Daygo/internal/platform"
)

func captureOnce(context.Context, platform.CaptureRequest) (platform.CaptureResult, error) {
	return platform.CaptureResult{}, &platform.CaptureError{Code: platform.CaptureUnsupported}
}

func frameAppend(context.Context, platform.CaptureRequest) (platform.CaptureResult, error) {
	return platform.CaptureResult{}, &platform.CaptureError{Code: platform.CaptureUnsupported}
}

func frameDecode(context.Context, string, string, int, int) ([]byte, error) {
	return nil, &platform.CaptureError{Code: platform.CaptureUnsupported}
}

func segmentProbe(context.Context, string, string) (platform.SegmentInfo, error) {
	return platform.SegmentInfo{}, &platform.CaptureError{Code: platform.CaptureUnsupported}
}

func segmentCloseActive() error {
	return nil
}

func testFrameAppendSynthetic(string, int, int, int) (platform.CaptureResult, error) {
	return platform.CaptureResult{}, &platform.CaptureError{Code: platform.CaptureUnsupported}
}
