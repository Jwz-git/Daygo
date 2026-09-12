//go:build windows

package windows

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/Jwz-git/Daygo/internal/platform"
)

var _ platform.ApplicationInspector = (*ApplicationInspector)(nil)

// ApplicationInspector resolves executable identities used by the Windows
// capture privacy filter. Paths are accepted only from the native picker and
// are never returned through the Go or Wails boundary.
type ApplicationInspector struct{}

func NewApplicationInspector() *ApplicationInspector { return &ApplicationInspector{} }

func (i *ApplicationInspector) InspectApplication(ctx context.Context, path string) (platform.ApplicationIdentity, error) {
	if err := ctx.Err(); err != nil {
		return platform.ApplicationIdentity{}, err
	}
	if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Ext(path), ".exe") || len(path) > 32768 || !utf8.ValidString(path) {
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
			var applicationError *platform.ApplicationError
			if errors.As(err, &applicationError) && (applicationError.Code == platform.ApplicationNotFound ||
				applicationError.Code == platform.ApplicationInvalidArgument || applicationError.Code == platform.ApplicationUnsupported) {
				identities = append(identities, platform.ApplicationIdentity{ID: id})
				continue
			}
			return nil, err
		}
		identities = append(identities, identity)
	}
	return identities, nil
}
