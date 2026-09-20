//go:build windows

package windows

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/Jwz-git/Daygo/internal/platform"
	"golang.org/x/sys/windows/registry"
)

var installedApplicationRoots = []struct {
	root registry.Key
	path string
}{
	{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\App Paths`},
	{registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\App Paths`},
	{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Uninstall`},
}

// installedApplications enumerates the same registry sources used by the
// native identity lookup. Entries are resolved through ApplicationInspector so
// the ID exactly matches the privacy filter's canonical-path SHA-256 identity.
func installedApplications(ctx context.Context) ([]platform.AppInfo, error) {
	byID := make(map[string]platform.AppInfo)
	for _, source := range installedApplicationRoots {
		for _, view := range []uint32{registry.WOW64_64KEY, registry.WOW64_32KEY} {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			key, err := registry.OpenKey(source.root, source.path, registry.READ|view)
			if err != nil {
				if errors.Is(err, registry.ErrNotExist) {
					continue
				}
				return nil, err
			}
			names, err := key.ReadSubKeyNames(-1)
			key.Close()
			if err != nil {
				return nil, err
			}
			for _, name := range names {
				path := installedExecutablePath(source.root, source.path+`\`+name, view)
				if path == "" {
					continue
				}
				identity, err := inspectApplication(ctx, path)
				if err != nil || identity.ID == "" || identity.Name == "" {
					continue
				}
				byID[identity.ID] = platform.AppInfo{ID: identity.ID, Name: identity.Name}
			}
		}
	}
	result := make([]platform.AppInfo, 0, len(byID))
	for _, app := range byID {
		result = append(result, app)
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := strings.ToLower(result[i].Name), strings.ToLower(result[j].Name)
		if left == right {
			return result[i].ID < result[j].ID
		}
		return left < right
	})
	return result, nil
}

func installedExecutablePath(root registry.Key, subkey string, view uint32) string {
	key, err := registry.OpenKey(root, subkey, registry.READ|view)
	if err != nil {
		return ""
	}
	defer key.Close()
	value, _, err := key.GetStringValue("")
	if err != nil {
		value, _, err = key.GetStringValue("DisplayIcon")
	}
	if err != nil {
		return ""
	}
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, `"`) {
		if end := strings.Index(value[1:], `"`); end >= 0 {
			return value[1 : end+1]
		}
	}
	if comma := strings.LastIndex(value, ","); comma >= 0 {
		value = value[:comma]
	}
	return strings.TrimSpace(value)
}
