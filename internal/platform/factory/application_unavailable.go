//go:build !darwin

package factory

import (
	"context"

	"github.com/Jwz-git/Daygo/internal/platform"
)

func NewApplicationInspector() platform.ApplicationInspector {
	return unavailableApplicationInspector{}
}

type unavailableApplicationInspector struct{}

func (unavailableApplicationInspector) InspectApplication(context.Context, string) (platform.AppInfo, error) {
	return platform.AppInfo{}, &platform.ApplicationError{Code: platform.ApplicationUnsupported}
}
