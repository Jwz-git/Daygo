//go:build !darwin && !windows

package factory

import "github.com/Jwz-git/Daygo/internal/platform"

func NewSystem() platform.System { return nil }
