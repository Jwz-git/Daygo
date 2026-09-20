//go:build windows

package windows

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Jwz-git/Daygo/internal/platform"
	"golang.org/x/sys/windows/registry"
)

// installedApplicationRoots lists the registry sources the enumeration reads,
// in the order their entries are preferred. App Paths comes before Uninstall
// on purpose: it registers the executable the shell launches, while an
// Uninstall DisplayIcon is a display hint that vendors frequently pin to a
// version-stamped directory that changes on every update.
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
	paths, err := installedApplicationPaths(ctx)
	if err != nil {
		return nil, err
	}
	applications := resolveInstalledApplications(ctx, paths, sameExecutable, inspectApplication)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return applications, nil
}

// installedApplicationPaths collects the executable paths the registry
// advertises, in the order their sources are preferred. A path is carried as
// the registry spelled it; whether it resolves to an application, and whether
// it repeats one already listed, is decided by the resolver.
func installedApplicationPaths(ctx context.Context) ([]string, error) {
	var paths []string
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
				if path := installedExecutablePath(source.root, source.path+`\`+name, view); path != "" {
					paths = append(paths, path)
				}
			}
		}
	}
	return paths, nil
}

// listedApplication is an accepted entry plus the path it was resolved from,
// which the duplicate check needs in order to compare executables.
type listedApplication struct {
	id   string
	name string
	path string
}

// resolveInstalledApplications turns candidate executable paths into the
// identifier/name pairs the privacy grid renders. Paths are visited in
// enumeration order and the first path to register a program wins, so an
// application stays a single entry however many times the registry names it.
//
// The privacy identity is a hash of the executable's path, so two paths to one
// program are two identities and would otherwise render as two tiles — where
// only the one a process actually runs from can ever match the filter, leaving
// the other a tile that blocks nothing. Identity cannot detect that on its own;
// the executable's name and bytes can.
func resolveInstalledApplications(
	ctx context.Context,
	paths []string,
	sameProgram func(listed, candidate string) bool,
	inspect func(context.Context, string) (platform.ApplicationIdentity, error),
) []platform.AppInfo {
	var accepted []listedApplication
	seen := make(map[string]struct{})
	for _, path := range paths {
		if ctx.Err() != nil {
			break
		}
		app, err := inspect(ctx, path)
		if err != nil || app.ID == "" || app.Name == "" {
			continue
		}
		if _, listed := seen[app.ID]; listed {
			continue
		}
		duplicate := false
		for _, entry := range accepted {
			if entry.name == app.Name && sameProgram(entry.path, path) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		seen[app.ID] = struct{}{}
		accepted = append(accepted, listedApplication{id: app.ID, name: app.Name, path: path})
	}
	result := make([]platform.AppInfo, 0, len(accepted))
	for _, entry := range accepted {
		result = append(result, platform.AppInfo{ID: entry.id, Name: entry.name})
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := strings.ToLower(result[i].Name), strings.ToLower(result[j].Name)
		if left == right {
			return result[i].ID < result[j].ID
		}
		return left < right
	})
	return result
}

// sameExecutable reports whether a candidate path holds the same program as one
// already listed, which is how the enumeration recognises a program installed
// twice: Microsoft Edge ships msedge.exe both directly under
// `Edge\Application\` and as a second copy under `Edge\Application\<version>\`,
// registering the first through App Paths and the second through an Uninstall
// DisplayIcon. The running browser uses the first, so keeping the second would
// list an application whose tile can never match.
//
// The display name already has to match before this is consulted; the file name
// is checked here. Comparing bytes and not just names is what separates Edge —
// one program installed twice — from Edge Stable beside Edge Beta, which
// register the same msedge.exe name and must stay separately blockable.
// Comparing bytes and not just file names is what separates it from Python's
// launchers, which ship one image under several names (idle3.11.exe,
// pythonw3.11.exe) and are different applications to whoever is blocking them.
//
// The size settles most pairs without reading anything, and the digest is
// computed only for the remainder.
func sameExecutable(listed, candidate string) bool {
	if !strings.EqualFold(filepath.Base(listed), filepath.Base(candidate)) {
		return false
	}
	listedInfo, err := os.Stat(listed)
	if err != nil {
		return false
	}
	candidateInfo, err := os.Stat(candidate)
	if err != nil || listedInfo.Size() != candidateInfo.Size() {
		return false
	}
	listedDigest, ok := fileDigest(listed)
	if !ok {
		return false
	}
	candidateDigest, ok := fileDigest(candidate)
	return ok && bytes.Equal(listedDigest, candidateDigest)
}

func fileDigest(path string) ([]byte, bool) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, false
	}
	return hash.Sum(nil), true
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
