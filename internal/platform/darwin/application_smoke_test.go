//go:build darwin && cgo

package darwin

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestApplicationInspectorSmoke exercises Go -> cgo -> Swift -> AppKit/Foundation
// against an application selected by the operator. It is opt-in so portable and
// headless gates never depend on a particular macOS installation.
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
	if len(info.IconPNG) > 0 && !bytes.HasPrefix(info.IconPNG, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("icon is not a PNG: %d bytes", len(info.IconPNG))
	}
	t.Logf("resolved application name=%q id=%q icon=%d bytes", info.Name, info.ID, len(info.IconPNG))

	// The identifier just resolved must also resolve without a path: that is
	// the lookup the privacy list uses for already-configured entries.
	identities, err := NewApplicationInspector().DescribeApplications(ctx, []string{info.ID})
	if err != nil {
		t.Fatalf("DescribeApplications: %v", err)
	}
	if len(identities) != 1 {
		t.Fatalf("identities = %+v", identities)
	}
	if identities[0].ID != info.ID || identities[0].Name == "" {
		t.Fatalf("looked-up identity = %+v", identities[0])
	}
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

// A configured identifier the system does not know keeps its place in the list
// instead of failing the whole lookup.
func TestApplicationLookupReportsUnknownIdentifier(t *testing.T) {
	ids := []string{"com.example.daygo.not-installed", "com.example.daygo.also-missing"}

	identities, err := NewApplicationInspector().DescribeApplications(context.Background(), ids)
	if err != nil {
		t.Fatalf("DescribeApplications: %v", err)
	}
	if len(identities) != len(ids) {
		t.Fatalf("identities = %+v", identities)
	}
	for index, identity := range identities {
		if identity.ID != ids[index] || identity.Name != "" || len(identity.IconPNG) != 0 {
			t.Fatalf("identity %d = %+v, want an ID-only entry", index, identity)
		}
	}
}
