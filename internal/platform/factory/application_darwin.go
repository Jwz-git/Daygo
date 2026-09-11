//go:build darwin

package factory

import (
	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/darwin"
)

func NewApplicationInspector() platform.ApplicationInspector {
	return darwin.NewApplicationInspector()
}
