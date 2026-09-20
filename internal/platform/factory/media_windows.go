//go:build windows

package factory

import (
	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/windows"
)

func NewMedia(root string) platform.Media { return windows.NewMedia(root) }
