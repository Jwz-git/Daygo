#!/usr/bin/env bash

# Linux counterpart to the macOS `wails build` and the Windows
# `native/windows/build.ps1` script.
#
# Same shared helpers as dev-linux.sh — only difference is which wails
# subcommand gets executed at the end. Keeping both scripts thin means the
# production build tracks the development run exactly: change the WebKit2GTK
# tag detection or the Wails version in one place.
#
# Captures stay in-process on Linux too: the native/ tree has no Linux adapter
# yet (docs/09 §9.8 keeps the platform-adapter shape undecided), so this
# build does NOT link libdaygo_capture.a. Capabilities that depend on the
# native Capture / System adapters report Unsupported at runtime. Secrets uses
# the host's freedesktop Secret Service through secret-tool when installed;
# its absence is reported as SecretUnsupported without blocking startup. The
# Wails shell, Vue frontend, SQLite database and Go core remain functional.

set -euo pipefail

# shellcheck source=bootstrap-frontend.sh
source "$(dirname "${BASH_SOURCE[0]}")/bootstrap-frontend.sh"

require_tool go 'Install Go from https://go.dev/dl/ or via your package manager.'
require_tool npm 'Install Node.js 20.19+ (or 22.12+) from https://nodejs.org/.'

DAYGO_WAILS_TAG="$(webkit_tag)"
printf 'Selected WebKit2GTK build tag: %s\n' "$DAYGO_WAILS_TAG"

printf 'Syncing frontend dependencies...\n'
# `npm ci` here (instead of `npm install`) because production builds are run
# from a clean clone / CI runner and the lockfile is the source of truth.
npm --prefix "$ROOT_DIR/frontend" ci --no-audit --no-fund

printf 'Running bootstrap (placeholder dist → bindings → bundle)...\n'
daygo_bootstrap

printf 'Running wails build with tag %s...\n' "$DAYGO_WAILS_TAG"
run_wails build "$@"
