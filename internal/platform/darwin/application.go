//go:build darwin

package darwin

import (
	"context"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/Jwz-git/Daygo/internal/platform"
)

var _ platform.ApplicationInspector = (*ApplicationInspector)(nil)

// ApplicationInspector resolves a user-selected macOS application bundle to
// the exact identifier consumed by the capture privacy filter.
type ApplicationInspector struct{}

func NewApplicationInspector() *ApplicationInspector {
	return &ApplicationInspector{}
}

func (i *ApplicationInspector) InspectApplication(ctx context.Context, path string) (platform.AppInfo, error) {
	if err := ctx.Err(); err != nil {
		return platform.AppInfo{}, err
	}
	if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Ext(path), ".app") || len(path) > 32768 || !utf8.ValidString(path) {
		return platform.AppInfo{}, &platform.ApplicationError{Code: platform.ApplicationInvalidArgument}
	}
	return inspectApplication(ctx, path)
}
