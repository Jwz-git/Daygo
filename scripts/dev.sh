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

# frontend/wailsjs/ is generated and not committed, but src/api/*.ts imports it,
# so vue-tsc and vite both fail without it. Generating it needs a compilable Go
# tree, and go:embed all:dist needs dist to exist — hence the placeholder.
if [[ ! -f "$ROOT_DIR/frontend/wailsjs/go/app/Backend.d.ts" ]]; then
  printf 'Generating Wails bindings for frontend typecheck...\n'
  mkdir -p "$ROOT_DIR/frontend/dist"
  printf '<!doctype html>\n' > "$ROOT_DIR/frontend/dist/index.html"
  (cd "$ROOT_DIR/cmd/daygo" \
    && go run "github.com/wailsapp/wails/v2/cmd/wails@$WAILS_VERSION" build -s -m -nopackage)
fi

# Build the real bundle, replacing the placeholder if one was written above.
# The Go binary compiles dist via go:embed, so this must run before wails dev.
if [[ ! -d "$ROOT_DIR/frontend/dist/assets" ]]; then
  printf 'Building frontend bundle for go:embed...\n'
  npm --prefix "$ROOT_DIR/frontend" run build
fi

cd "$ROOT_DIR/cmd/daygo"

printf 'Starting wails dev (the first run may take a while to download the Wails CLI)...\n'
exec go run "github.com/wailsapp/wails/v2/cmd/wails@$WAILS_VERSION" dev -s "$@"
