#!/usr/bin/env bash

# macOS development entry. Sources scripts/bootstrap-frontend.sh for the
# shared host checks and wails invocation; the xcode-select check below is
# the only piece that stays here because Linux/Windows do not need it.
#
# Pass extra arguments through to `wails dev` (e.g. -platform darwin/arm64).

set -euo pipefail

# shellcheck source=bootstrap-frontend.sh
source "$(dirname "${BASH_SOURCE[0]}")/bootstrap-frontend.sh"

require_tool go 'Install Go from https://go.dev/dl/ or via Homebrew (brew install go).'
require_tool npm 'Install Node.js 20.19+ (or 22.12+) — Homebrew (brew install node) ships npm.'

if [[ "$(uname -s)" == "Darwin" ]] && ! command -v clang >/dev/null 2>&1; then
  printf 'error: Xcode Command Line Tools are required for the Wails build.\n' >&2
  printf '       Install them with: xcode-select --install\n' >&2
  exit 1
fi

printf 'Syncing frontend dependencies...\n'
npm --prefix "$ROOT_DIR/frontend" install --no-audit --no-fund

printf 'Running bootstrap (placeholder dist → bindings → bundle)...\n'
daygo_bootstrap

printf 'Starting wails dev (the first run may take a while to download the Wails CLI)...\n'
run_wails dev "$@"