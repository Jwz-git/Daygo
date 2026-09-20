//go:build !darwin && !windows

package factory

import (
	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/mediafile"
)

func NewMedia(root string) platform.Media {
	return mediafile.New(root)
}
