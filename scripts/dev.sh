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

if [[ ! -d "$ROOT_DIR/frontend/node_modules" ]]; then
  printf 'Installing frontend dependencies...\n'
  npm --prefix "$ROOT_DIR/frontend" ci
fi

cd "$ROOT_DIR/cmd/daygo"

exec go run "github.com/wailsapp/wails/v2/cmd/wails@$WAILS_VERSION" dev
