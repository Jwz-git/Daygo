#!/usr/bin/env bash

# Prepares the generated frontend artifacts that both `wails dev` and the
# headless gate in docs/08 §8.8 depend on.
#
# Two generated directories are involved, and they depend on each other:
#
#   frontend/dist/     go:embed all:dist — the Go module does not compile when
#                      the pattern matches no files, and `npm run build`
#                      (= vue-tsc + vite) cannot run until dist exists
#   frontend/wailsjs/  imported by src/api/*.ts — vue-tsc and vite both fail
#                      without it, and generating it compiles the Go module
#
# Order: placeholder dist → bindings → real bundle. Each step is skipped when
# its output already exists, so this is safe to run repeatedly.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WAILS_VERSION="${WAILS_VERSION:-v2.15.0}"

# frontend/wailsjs/ is generated and not committed, but src/api/*.ts imports it,
# so vue-tsc and vite both fail without it. It drifts as soon as a binding is
# added, renamed or removed, and a stale copy produces errors that name the
# missing member rather than the stale file — so regenerate unconditionally
# instead of testing for existence.
printf 'Generating Wails bindings...\n'
mkdir -p "$ROOT_DIR/frontend/dist"
printf '<!doctype html>\n' > "$ROOT_DIR/frontend/dist/index.html"
(cd "$ROOT_DIR/cmd/daygo" \
  && go run "github.com/wailsapp/wails/v2/cmd/wails@$WAILS_VERSION" build -s -m -nopackage)

# Build the real bundle, replacing the placeholder if one was written above.
# The Go binary compiles dist via go:embed, so this must run before any Go
# command that imports the frontend package.
if [[ ! -d "$ROOT_DIR/frontend/dist/assets" ]]; then
  printf 'Building frontend bundle for go:embed...\n'
  npm --prefix "$ROOT_DIR/frontend" run build
fi
