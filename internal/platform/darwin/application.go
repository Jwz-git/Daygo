//go:build darwin

package darwin

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/Jwz-git/Daygo/internal/platform"
)

var _ platform.ApplicationInspector = (*ApplicationInspector)(nil)

// ApplicationInspector resolves installed macOS application bundles to the
// identity consumed by the capture privacy filter, plus display-only name and
// icon data.
type ApplicationInspector struct{}

func NewApplicationInspector() *ApplicationInspector {
	return &ApplicationInspector{}
}

func (i *ApplicationInspector) InspectApplication(ctx context.Context, path string) (platform.ApplicationIdentity, error) {
	if err := ctx.Err(); err != nil {
		return platform.ApplicationIdentity{}, err
	}
	if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Ext(path), ".app") || len(path) > 32768 || !utf8.ValidString(path) {
		return platform.ApplicationIdentity{}, &platform.ApplicationError{Code: platform.ApplicationInvalidArgument}
	}
	return inspectApplication(ctx, path)
}

func (i *ApplicationInspector) DescribeApplications(ctx context.Context, ids []string) ([]platform.ApplicationIdentity, error) {
	identities := make([]platform.ApplicationIdentity, 0, len(ids))
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		identity, err := lookupApplication(ctx, id)
		if err != nil {
			if identityUnavailable(err) {
				// Keep one entry per configured identifier so the caller can
				// still render and remove it.
				identities = append(identities, platform.ApplicationIdentity{ID: id})
				continue
			}
			return nil, err
		}
		identities = append(identities, identity)
	}
	return identities, nil
}

// identityUnavailable reports whether the platform simply cannot name this
// application: it does not exist, the identifier is not well formed, or the
// adapter has no resolver at all. Those cases degrade to an ID-only entry;
// adapter failures (ABI mismatch, native error) propagate instead.
func identityUnavailable(err error) bool {
	var applicationError *platform.ApplicationError
	if !errors.As(err, &applicationError) {
		return false
	}
	switch applicationError.Code {
	case platform.ApplicationNotFound,
		platform.ApplicationInvalidArgument,
		platform.ApplicationUnsupported:
		return true
	default:
		return false
	}
}
