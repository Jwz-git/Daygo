//go:build !windows && !(darwin && cgo && daygo_updater)

package factory

import "github.com/Jwz-git/Daygo/internal/platform"

func NewUpdater() platform.Updater { return nil }
