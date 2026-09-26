#!/usr/bin/env bash
set -euo pipefail
task_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
task_output="$(mktemp -d)"
trap 'rm -rf "$task_output"' EXIT
xcrun swiftc -swift-version 6 -parse-as-library \
  -import-objc-header "$task_root/native/include/daygo_native.h" \
  -framework AppKit -framework Foundation -framework ServiceManagement -framework CoreGraphics \
  "$task_root/native/darwin/Sources/SystemABI.swift" \
  "$task_root/native/darwin/Sources/StatusItemABI.swift" \
  "$task_root/native/darwin/system_smoke.swift" -o "$task_output/system-smoke"
"$task_output/system-smoke"
