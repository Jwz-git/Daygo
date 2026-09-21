#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEPS_DIR="$ROOT_DIR/build/deps"
SPARKLE_VERSION="2.10.0"
SPARKLE_SHA256="c2bf58aa8387266ac179357b1415d6f2635f044da8be41042af32425dae6da0c"
ARCHIVE="$DEPS_DIR/Sparkle-$SPARKLE_VERSION.tar.xz"
DEST="$DEPS_DIR/Sparkle-$SPARKLE_VERSION"

mkdir -p "$DEPS_DIR"
if [[ ! -f "$ARCHIVE" ]] || ! printf '%s  %s\n' "$SPARKLE_SHA256" "$ARCHIVE" | shasum -a 256 -c - >/dev/null 2>&1; then
  curl -fL --retry 3 -C - \
    "https://github.com/sparkle-project/Sparkle/releases/download/$SPARKLE_VERSION/Sparkle-$SPARKLE_VERSION.tar.xz" \
    -o "$ARCHIVE"
  printf '%s  %s\n' "$SPARKLE_SHA256" "$ARCHIVE" | shasum -a 256 -c - >&2
fi
if [[ ! -d "$DEST/Sparkle.framework" ]]; then
  mkdir -p "$DEST"
  tar -xf "$ARCHIVE" -C "$DEST"
fi
printf '%s\n' "$DEST"
