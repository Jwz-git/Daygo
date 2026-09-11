//go:build darwin

package factory

import (
	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/darwin"
)

func NewCapture() platform.Capture {
	return darwin.NewCapture()
}
