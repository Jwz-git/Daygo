//go:build windows && !cgo

package windows

import (
	"context"
	"fmt"

	"github.com/Jwz-git/Daygo/internal/platform"
)

func captureOnce(context.Context, platform.CaptureRequest) (platform.CaptureResult, error) {
	return platform.CaptureResult{}, &platform.CaptureError{Code: platform.CaptureUnsupported}
}

func frameAppend(context.Context, platform.CaptureRequest) (platform.CaptureResult, error) {
	return platform.CaptureResult{}, &platform.CaptureError{Code: platform.CaptureUnsupported}
}

func segmentCloseActive() error {
	return &platform.CaptureError{Code: platform.CaptureUnsupported}
}

func frameDecode(context.Context, string, string, int, int) ([]byte, error) {
	return nil, fmt.Errorf("windows media unavailable without cgo")
}

func segmentProbe(context.Context, string, string) (platform.SegmentInfo, error) {
	return platform.SegmentInfo{}, fmt.Errorf("windows media unavailable without cgo")
}
