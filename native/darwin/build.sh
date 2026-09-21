#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT_DIR="$ROOT_DIR/build/native/darwin"
HEADER="$ROOT_DIR/native/include/daygo_native.h"
SDK_PATH="$(xcrun --sdk macosx --show-sdk-path)"
SOURCES=("$ROOT_DIR"/native/darwin/Sources/*.swift)
ARCHIVE="$OUT_DIR/arm64/libdaygo_capture.a"

mkdir -p "$OUT_DIR/arm64"

xcrun swiftc \
  -swift-version 6 \
  -parse-as-library \
  -O \
  -whole-module-optimization \
  -target "arm64-apple-macos14.0" \
  -sdk "$SDK_PATH" \
  -module-name DaygoCapture \
  -module-cache-path "$OUT_DIR/arm64/module-cache" \
  -import-objc-header "$HEADER" \
  -emit-library \
  -static \
  -framework CoreGraphics \
  -framework Foundation \
  -framework AppKit \
  -framework ImageIO \
  -framework ScreenCaptureKit \
  -framework UniformTypeIdentifiers \
  -framework AVFoundation \
  -framework CoreMedia \
  -framework VideoToolbox \
  "${SOURCES[@]}" \
  -o "$ARCHIVE"

xcrun lipo -info "$ARCHIVE"
