//go:build darwin && cgo

package darwin

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestInstalledApplicationsSmoke exercises the Go -> cgo -> Foundation
// enumeration against the real machine's installed applications. It is opt-in
// so portable and headless gates never depend on a particular macOS
// installation; run it with DAYGO_APPLICATION_LIST_SMOKE=1 after building the
// native library.
func TestInstalledApplicationsSmoke(t *testing.T) {
	if os.Getenv("DAYGO_APPLICATION_LIST_SMOKE") == "" {
		t.Skip("set DAYGO_APPLICATION_LIST_SMOKE=1 to enumerate the real machine's applications")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	apps, err := listApplications(ctx, "zh-CN")
	if err != nil {
		t.Fatalf("listApplications: %v", err)
	}
	if len(apps) == 0 {
		t.Fatal("no applications enumerated")
	}
	t.Logf("enumerated %d applications (zh-CN)", len(apps))

	// The grid must follow Daygo's UI language, not the host's. Notes ships a
	// zh_CN localization whose display name is 备忘录.
	names := make(map[string]string, len(apps))
	for _, app := range apps {
		names[app.ID] = app.Name
	}
	if name, ok := names["com.apple.Notes"]; !ok {
		t.Log("Notes not installed; skipping the localized-name assertion")
	} else if name != "备忘录" {
		t.Fatalf("Notes name = %q, want 备忘录", name)
	} else {
		t.Log("com.apple.Notes → 备忘录 ✓")
	}

	seen := make(map[string]bool, len(apps))
	for _, app := range apps {
		if app.ID == "" || app.Name == "" {
			t.Fatalf("incomplete entry: %+v", app)
		}
		if seen[app.ID] {
			t.Fatalf("duplicate identifier: %s", app.ID)
		}
		seen[app.ID] = true
	}
	for i, app := range apps {
		if i >= 5 {
			break
		}
		t.Logf("  %s → %s", app.ID, app.Name)
	}
}
