#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT_DIR="$ROOT_DIR/build/native/darwin"
HEADER="$ROOT_DIR/native/include/daygo_capture.h"
SDK_PATH="$(xcrun --sdk macosx --show-sdk-path)"
SOURCES=("$ROOT_DIR"/native/darwin/Sources/*.swift)
ARCHIVES=()
UNIVERSAL_ARCHIVE="$OUT_DIR/universal/libdaygo_capture.a"


mkdir -p "$OUT_DIR/arm64" "$OUT_DIR/x86_64" "$OUT_DIR/universal"

for arch in arm64 x86_64; do
  archive="$OUT_DIR/$arch/libdaygo_capture.a"
  xcrun swiftc \
    -swift-version 6 \
    -parse-as-library \
    -O \
    -whole-module-optimization \
    -target "$arch-apple-macos14.0" \
    -sdk "$SDK_PATH" \
    -module-name DaygoCapture \
    -module-cache-path "$OUT_DIR/$arch/module-cache" \
    -import-objc-header "$HEADER" \
    -emit-library \
    -static \
    -framework CoreGraphics \
    -framework Foundation \
    -framework ImageIO \
    -framework ScreenCaptureKit \
    -framework Security \
    -framework UniformTypeIdentifiers \
    "${SOURCES[@]}" \
    -o "$archive"
  ARCHIVES+=("$archive")
done

xcrun lipo -create "${ARCHIVES[@]}" -output "$UNIVERSAL_ARCHIVE"
xcrun lipo -info "$UNIVERSAL_ARCHIVE"
