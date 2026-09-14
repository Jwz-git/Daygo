//go:build darwin && !cgo

package darwin

import (
	"context"

	"github.com/Jwz-git/Daygo/internal/platform"
)

func inspectApplication(context.Context, string) (platform.ApplicationIdentity, error) {
	return platform.ApplicationIdentity{}, &platform.ApplicationError{Code: platform.ApplicationUnsupported}
}

func lookupApplication(context.Context, string) (platform.ApplicationIdentity, error) {
	return platform.ApplicationIdentity{}, &platform.ApplicationError{Code: platform.ApplicationUnsupported}
}

func listApplications(context.Context, string) ([]platform.AppInfo, error) {
	return nil, &platform.ApplicationError{Code: platform.ApplicationUnsupported}
}
