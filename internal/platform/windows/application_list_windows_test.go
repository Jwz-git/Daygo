//go:build windows

package windows

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Jwz-git/Daygo/internal/platform"
)

// Paths as the registry enumeration hands them to the resolver: two of them can
// name one program and still hash differently, because the ID is a hash of the
// path string.
const (
	appPathsEdge      = `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`
	uninstallEdge     = `C:\Program Files (x86)\Microsoft\Edge\Application\151.0.4129.86\msedge.exe`
	appPathsTerminal  = `C:\Program Files\WindowsApps\terminal.exe`
	appPathsPython39  = `C:\Python39\python.exe`
	appPathsPython312 = `C:\Python312\python.exe`
)

const edgeName = "Microsoft Edge"

// inspectorFor resolves the paths a test declared and rejects the rest, the way
// the real inspector rejects a path it cannot identify. The ID is a hash of the
// path, so two entries for one program keep the distinct IDs that made them two
// tiles in the first place.
func inspectorFor(names map[string]string) func(context.Context, string) (platform.ApplicationIdentity, error) {
	return func(_ context.Context, path string) (platform.ApplicationIdentity, error) {
		name, ok := names[path]
		if !ok {
			return platform.ApplicationIdentity{}, errors.New("inspector rejected the path")
		}
		return platform.ApplicationIdentity{ID: "win32.exe.sha256:" + path, Name: name}, nil
	}
}

// sameExecutablePairs answers for the unordered path pairs a test declared as
// one program installed twice.
func sameExecutablePairs(pairs ...[2]string) func(string, string) bool {
	return func(left, right string) bool {
		for _, pair := range pairs {
			if (pair[0] == left && pair[1] == right) || (pair[0] == right && pair[1] == left) {
				return true
			}
		}
		return false
	}
}

func TestResolveInstalledApplicationsMergesIdenticalExecutables(t *testing.T) {
	// Edge registers the same program twice: App Paths names the copy the
	// browser runs from, the Uninstall DisplayIcon names a byte-identical copy
	// one directory down.
	names := map[string]string{appPathsEdge: edgeName, uninstallEdge: edgeName}

	got := resolveInstalledApplications(context.Background(),
		[]string{appPathsEdge, uninstallEdge},
		sameExecutablePairs([2]string{appPathsEdge, uninstallEdge}),
		inspectorFor(names))

	if len(got) != 1 {
		t.Fatalf("applications = %#v, want the two registrations merged into one entry", got)
	}
	// App Paths is enumerated first and must win: it is the path the running
	// process reports, so it is the one the privacy filter can match.
	if want := "win32.exe.sha256:" + appPathsEdge; got[0].ID != want {
		t.Fatalf("application ID = %q, want the App Paths entry %q", got[0].ID, want)
	}
}

func TestResolveInstalledApplicationsKeepsDistinctExecutables(t *testing.T) {
	// Same display name, different programs: two Python installations are two
	// applications and must both stay listed.
	names := map[string]string{appPathsPython39: "Python", appPathsPython312: "Python"}

	got := resolveInstalledApplications(context.Background(),
		[]string{appPathsPython39, appPathsPython312},
		sameExecutablePairs(),
		inspectorFor(names))

	if len(got) != 2 {
		t.Fatalf("applications = %#v, want both installations listed", got)
	}
	// Same display name on both entries, so the tie-break is the ID.
	if got[0].ID >= got[1].ID {
		t.Fatalf("applications are not ordered deterministically: %#v", got)
	}
}

func TestResolveInstalledApplicationsDoesNotCompareAcrossNames(t *testing.T) {
	// Byte-identical candidates under different display names are never read:
	// the name has to match before the executables are compared at all.
	names := map[string]string{appPathsEdge: edgeName, uninstallEdge: "Microsoft Edge Beta"}

	got := resolveInstalledApplications(context.Background(),
		[]string{appPathsEdge, uninstallEdge},
		func(string, string) bool { t.Fatal("executables compared across display names"); return false },
		inspectorFor(names))

	if len(got) != 2 {
		t.Fatalf("applications = %#v, want both names listed", got)
	}
}

func TestResolveInstalledApplicationsCollapsesRepeatedPaths(t *testing.T) {
	names := map[string]string{appPathsEdge: edgeName}

	got := resolveInstalledApplications(context.Background(),
		[]string{appPathsEdge, appPathsEdge}, sameExecutablePairs(), inspectorFor(names))

	if len(got) != 1 {
		t.Fatalf("applications = %#v, want the repeated path listed once", got)
	}
}

func TestResolveInstalledApplicationsLeavesUnresolvedPathsUnclaimed(t *testing.T) {
	// The first path for the executable cannot be resolved, so it must not
	// reserve the program against the second one.
	names := map[string]string{uninstallEdge: edgeName}

	got := resolveInstalledApplications(context.Background(),
		[]string{appPathsEdge, uninstallEdge},
		sameExecutablePairs([2]string{appPathsEdge, uninstallEdge}),
		inspectorFor(names))

	if len(got) != 1 || got[0].ID != "win32.exe.sha256:"+uninstallEdge {
		t.Fatalf("applications = %#v, want the resolvable path", got)
	}
}

func TestSameExecutable(t *testing.T) {
	directory := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		return path
	}
	original := write("msedge.exe", "browser")
	secondCopy := write("msedge.exe", "browser")
	if err := os.MkdirAll(filepath.Join(directory, "151.0.4129.86"), 0o700); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	versioned := write(filepath.Join("151.0.4129.86", "msedge.exe"), "browser")
	// Same length, different bytes: size alone must not be taken as equality.
	sameLength := write("same-length.exe", "BROWSER")
	// Byte-identical under a different file name: Python ships idle3.11.exe and
	// pythonw3.11.exe as one image, and they stay separate applications.
	otherName := write("pythonw.exe", "browser")
	if err := os.MkdirAll(filepath.Join(directory, "short"), 0o700); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	shorter := write(filepath.Join("short", "msedge.exe"), "brows")

	if !sameExecutable(original, versioned) {
		t.Fatal("the same executable name with identical bytes was not recognised")
	}
	if !sameExecutable(original, secondCopy) {
		t.Fatal("two copies of one file were not recognised")
	}
	if sameExecutable(original, sameLength) {
		t.Fatal("files of equal length but different content were treated as one program")
	}
	if sameExecutable(original, otherName) {
		t.Fatal("identical bytes under a different file name were treated as one program")
	}
	if sameExecutable(original, shorter) {
		t.Fatal("files of different length were treated as one program")
	}
	if sameExecutable(original, filepath.Join(directory, "missing.exe")) {
		t.Fatal("a missing path was treated as the same program")
	}
}
