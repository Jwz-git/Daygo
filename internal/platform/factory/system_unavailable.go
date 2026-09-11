//go:build !darwin

package factory

import "github.com/Jwz-git/Daygo/internal/platform"

func NewSystem() platform.System { return nil }
