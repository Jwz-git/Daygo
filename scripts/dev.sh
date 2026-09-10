#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WAILS_VERSION="v2.15.0"

if ! command -v go >/dev/null 2>&1; then
  printf 'error: Go is required but was not found in PATH.\n' >&2
  exit 1
fi

if ! command -v npm >/dev/null 2>&1; then
  printf 'error: npm is required but was not found in PATH.\n' >&2
  exit 1
fi

if [[ "$(uname -s)" == "Darwin" ]] && ! command -v clang >/dev/null 2>&1; then
  printf 'error: Xcode Command Line Tools are required for the Wails build.\n' >&2
  printf '       Install them with: xcode-select --install\n' >&2
  exit 1
fi

printf 'Syncing frontend dependencies...\n'
npm --prefix "$ROOT_DIR/frontend" install --no-audit --no-fund

# `wails dev -s` below skips the production frontend build; the Go binary still
# compiles frontend/dist via go:embed, so make sure it exists at least once
# (it is gitignored, so fresh clones need this first build).
if [[ ! -f "$ROOT_DIR/frontend/dist/index.html" ]]; then
  printf 'Building frontend bundle once for go:embed...\n'
  npm --prefix "$ROOT_DIR/frontend" run build
fi

cd "$ROOT_DIR/cmd/daygo"

printf 'Starting wails dev (the first run may take a while to download the Wails CLI)...\n'
exec go run "github.com/wailsapp/wails/v2/cmd/wails@$WAILS_VERSION" dev -s "$@"
