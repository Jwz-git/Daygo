//go:build darwin

package factory

import "github.com/Jwz-git/Daygo/internal/platform"
import "github.com/Jwz-git/Daygo/internal/platform/darwin"

func NewSystem() platform.System { return darwin.NewSystem() }
