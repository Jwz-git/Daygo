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
# go:embed all:dist fails to compile when the pattern matches no files, so the
# directory must hold something. Write the placeholder ONLY when there is no
# index.html: overwriting one unconditionally destroys a real bundle's entry
# point while leaving its assets/ in place, and the app then serves a blank page.
if [[ ! -f "$ROOT_DIR/frontend/dist/index.html" ]]; then
  printf '<!doctype html>\n' > "$ROOT_DIR/frontend/dist/index.html"
fi
(cd "$ROOT_DIR/cmd/daygo" \
  && go run "github.com/wailsapp/wails/v2/cmd/wails@$WAILS_VERSION" build -s -m -nopackage)

# Build the real bundle. The test is the entry point's content, not the presence
# of assets/: a real index.html references its hashed asset files and a
# placeholder does not, whereas assets/ can survive a placeholder write and
# would make this skip the rebuild that is needed.
if ! grep -q 'assets/' "$ROOT_DIR/frontend/dist/index.html" 2>/dev/null; then
  printf 'Building frontend bundle for go:embed...\n'
  npm --prefix "$ROOT_DIR/frontend" run build
fi
