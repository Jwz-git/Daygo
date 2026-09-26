#!/usr/bin/env bash
set -euo pipefail
task_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
task_output="$(mktemp -d)"
trap 'rm -rf "$task_output"' EXIT
xcrun swiftc -swift-version 6 -parse-as-library -D DAYGO_NATIVE_TESTS \
  -import-objc-header "$task_root/native/include/daygo_native.h" \
  -framework AppKit -framework Foundation -framework ServiceManagement -framework CoreGraphics \
  "$task_root/native/darwin/Sources/SystemABI.swift" \
  "$task_root/native/darwin/Sources/StatusItemABI.swift" \
  "$task_root/native/darwin/Sources/ApplicationMenuABI.swift" \
  "$task_root/native/darwin/Sources/ResidentMessageABI.swift" \
  "$task_root/native/darwin/system_smoke.swift" -o "$task_output/system-smoke"
"$task_output/system-smoke"
# Repeat in an anonymous app bundle, without Daygo's identity or user data.
task_bundle="$task_output/Fixture.app/Contents"
mkdir -p "$task_bundle/MacOS"
cp "$task_output/system-smoke" "$task_bundle/MacOS/system-smoke"
cat > "$task_bundle/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>system-smoke</string>
<key>CFBundleIdentifier</key><string>invalid.daygo.anonymous-system-fixture</string>
<key>CFBundleName</key><string>Anonymous System Fixture</string>
<key>CFBundlePackageType</key><string>APPL</string>
</dict></plist>
PLIST
"$task_bundle/MacOS/system-smoke"
