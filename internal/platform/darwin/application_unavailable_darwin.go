//go:build darwin && !cgo

package darwin

import (
	"context"

	"github.com/Jwz-git/Daygo/internal/platform"
)

func inspectApplication(context.Context, string) (platform.AppInfo, error) {
	return platform.AppInfo{}, &platform.ApplicationError{Code: platform.ApplicationUnsupported}
}
