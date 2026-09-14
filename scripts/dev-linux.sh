#!/usr/bin/env bash

# Linux counterpart to scripts/dev.sh (mac) and scripts/dev.ps1 (Windows).
#
# Pulls host checks, Wails tag selection, and the wails dev invocation from
# scripts/bootstrap-frontend.sh — the same script that runs the placeholder
# dist → bindings → real bundle pipeline before wails dev starts. Anything
# specific to Linux (which package manager the host uses, etc.) stays out:
# `wails doctor` already reports missing WebKit2GTK headers, and a wrong
# package-manager guess in a cross-distribution script is more harmful than
# a missing dependency the user already knows how to install.
#
# Two environment overrides are honoured:
#   DAYGO_LINUX_WEBKIT_TAG=webkit2_40|webkit2_41  force a specific ABI tag.
#   DAYGO_WAILS_TAG=...                          pass through to `wails dev -tags`
#                                                (combined with the ABI tag).

set -euo pipefail

# shellcheck source=bootstrap-frontend.sh
source "$(dirname "${BASH_SOURCE[0]}")/bootstrap-frontend.sh"

require_tool go 'Install Go from https://go.dev/dl/ or via your package manager.'
require_tool npm 'Install Node.js 20.19+ (or 22.12+) from https://nodejs.org/.'

DAYGO_WAILS_TAG="$(webkit_tag)"
printf 'Selected WebKit2GTK build tag: %s\n' "$DAYGO_WAILS_TAG"

printf 'Syncing frontend dependencies...\n'
npm --prefix "$ROOT_DIR/frontend" install --no-audit --no-fund

# Bootstrap runs before wails dev because wails dev assumes the placeholder
# dist + generated wailsjs exist; if the bundle was just rebuilt above it
# might still be the placeholder, so we re-run unconditionally here.
printf 'Running bootstrap (placeholder dist → bindings → bundle)...\n'
daygo_bootstrap

printf 'Starting wails dev (the first run may take a while to download the Wails CLI)...\n'
run_wails dev "$@"