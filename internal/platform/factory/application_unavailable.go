//go:build !darwin && !windows

package factory

import (
	"context"

	"github.com/Jwz-git/Daygo/internal/platform"
)

func NewApplicationInspector() platform.ApplicationInspector {
	return unavailableApplicationInspector{}
}

type unavailableApplicationInspector struct{}

func (unavailableApplicationInspector) InspectApplication(context.Context, string) (platform.ApplicationIdentity, error) {
	return platform.ApplicationIdentity{}, &platform.ApplicationError{Code: platform.ApplicationUnsupported}
}

func (unavailableApplicationInspector) DescribeApplications(_ context.Context, ids []string) ([]platform.ApplicationIdentity, error) {
	// No resolver on this platform: every configured identifier stays visible
	// as an ID-only entry instead of failing the whole list.
	identities := make([]platform.ApplicationIdentity, 0, len(ids))
	for _, id := range ids {
		identities = append(identities, platform.ApplicationIdentity{ID: id})
	}
	return identities, nil
}
