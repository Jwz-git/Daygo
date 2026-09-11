//go:build darwin && cgo

package darwin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestApplicationInspectorSmoke exercises Go -> cgo -> Swift -> Foundation
// against an application selected by the operator. It is opt-in so portable
// and headless gates never depend on a particular macOS installation.
func TestApplicationInspectorSmoke(t *testing.T) {
	path := os.Getenv("DAYGO_APPLICATION_SMOKE_PATH")
	if path == "" {
		t.Skip("set DAYGO_APPLICATION_SMOKE_PATH to a .app bundle")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	info, err := NewApplicationInspector().InspectApplication(ctx, path)
	if err != nil {
		t.Fatalf("InspectApplication: %v", err)
	}
	if info.ID == "" || info.Name == "" {
		t.Fatalf("incomplete application identity: %+v", info)
	}
	t.Logf("resolved application name=%q id=%q", info.Name, info.ID)
}

func TestApplicationInspectorUsesBundleIdentifierWithoutSignatureGate(t *testing.T) {
	bundlePath := filepath.Join(t.TempDir(), "Unsigned Fixture.app")
	contentsPath := filepath.Join(bundlePath, "Contents")
	if err := os.MkdirAll(contentsPath, 0o755); err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	const infoPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>com.example.daygo.unsigned-fixture</string>
	<key>CFBundleName</key>
	<string>Unsigned Fixture</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
</dict>
</plist>`
	if err := os.WriteFile(filepath.Join(contentsPath, "Info.plist"), []byte(infoPlist), 0o644); err != nil {
		t.Fatalf("write Info.plist: %v", err)
	}

	info, err := NewApplicationInspector().InspectApplication(context.Background(), bundlePath)
	if err != nil {
		t.Fatalf("InspectApplication: %v", err)
	}
	if info.ID != "com.example.daygo.unsigned-fixture" || info.Name != "Unsigned Fixture" {
		t.Fatalf("application identity = %+v", info)
	}
}
