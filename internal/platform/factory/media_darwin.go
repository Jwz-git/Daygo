//go:build darwin

package factory

import (
	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/darwin"
)

func NewMedia(root string) platform.Media {
	return darwin.NewMedia(root)
}
