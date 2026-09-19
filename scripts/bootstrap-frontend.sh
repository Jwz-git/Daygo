#!/usr/bin/env bash

# Two roles for this script:
#
#   ./bootstrap-frontend.sh            standalone — runs the bootstrap pipeline
#                                      (placeholder dist → bindings → real
#                                      bundle) used by every dev entry.
#   . bootstrap-frontend.sh            sourced — exports helpers (require_tool,
#                                      webkit_tag, run_wails, daygo_bootstrap)
#                                      WITHOUT running the pipeline, so callers
#                                      source to get helpers then call
#                                      daygo_bootstrap explicitly.
#
# Both roles share ROOT_DIR, WAILS_VERSION, and the tool / tag resolution so
# the gate stays consistent: changing the Wails version, the host check, or
# the WebKit2GTK ABI heuristic happens here, not in three near-duplicate
# scripts.

set -euo pipefail

# BASH_SOURCE[0] is the script's own filename whether sourced or executed.
# dirname on it resolves to the scripts/ directory regardless of the caller's
# CWD.
THIS_FILE="${BASH_SOURCE[0]:-$0}"
ROOT_DIR="$(cd "$(dirname "$THIS_FILE")/.." && pwd)"
WAILS_VERSION="${WAILS_VERSION:-v2.15.0}"

# Detect "sourced vs executed": when sourced (`. script` or `source script`)
# BASH_SOURCE has at least 2 frames — the caller's and ours. When executed
# directly, it has exactly 1 (us).  The test runs in a subshell so `return`
# exits only the subshell, never the caller's shell.
_daygo_sourced=0
if (return 0 2>/dev/null); then
  _daygo_sourced=1
fi

# ─── shared helpers (defined unconditionally so source + run both work) ───

# require_tool prints a one-line error and exits non-zero when $1 is not on
# PATH. Each caller lists what it actually needs; we do not assume a fixed
# host because the three dev scripts each have different tool requirements
# (xash is macOS-only, pkg-config is Linux-only, etc.).
require_tool() {
  local name="$1"
  local hint="${2:-}"
  if ! command -v "$name" >/dev/null 2>&1; then
    printf 'error: %s is required but was not found in PATH.\n' "$name" >&2
    if [[ -n "$hint" ]]; then
      printf '       %s\n' "$hint" >&2
    fi
    exit 1
  fi
}

# webkit_tag resolves the WebKit2GTK build tag for `wails dev` / `wails build`
# on Linux. Returns 0 with the tag printed on stdout; errors to stderr.
# Override with $DAYGO_LINUX_WEBKIT_TAG when the host has only one ABI and
# pkg-config happens to disagree (it does not — we test the same library name
# wails itself uses — but the override exists for policy reasons).
webkit_tag() {
  if [[ -n "${DAYGO_LINUX_WEBKIT_TAG:-}" ]]; then
    printf '%s\n' "$DAYGO_LINUX_WEBKIT_TAG"
    return 0
  fi
  if ! command -v pkg-config >/dev/null 2>&1; then
    printf 'error: pkg-config is required to detect WebKit2GTK on Linux.\n' >&2
    return 1
  fi
  if pkg-config --exists webkit2gtk-4.1; then
    printf 'webkit2_41\n'
    return 0
  fi
  if pkg-config --exists webkit2gtk-4.0; then
    printf 'webkit2_40\n'
    return 0
  fi
  printf 'error: no WebKit2GTK development headers were found.\n' >&2
  printf '       Debian/Ubuntu 22.04+: sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config\n' >&2
  printf '       Fedora 40+:          sudo dnf install gtk3-devel webkit2gtk4.1-devel pkgconf-pkg-config\n' >&2
  printf '       Arch:                sudo pacman -S --needed gtk3 webkit2gtk-4.1 pkgconf\n' >&2
  printf '       On older distributions (4.0-only), set DAYGO_LINUX_WEBKIT_TAG=webkit2_40.\n' >&2
  return 1
}

# run_wails executes the Wails CLI from cmd/daygo with the tag the caller
# passed (Linux only — other platforms leave the tag empty so wails applies
# its own defaults).  DAYGO_WAILS_TAG is checked with -n so an empty string
# (unset on non-Linux) does not produce a spurious -tags "" flag.
run_wails() {
  local subcommand="$1"
  shift
  if [[ -n "${DAYGO_WAILS_TAG:-}" ]]; then
    (
      cd "$ROOT_DIR/cmd/daygo"
      exec go run "github.com/wailsapp/wails/v2/cmd/wails@$WAILS_VERSION" \
        "$subcommand" -s -tags "$DAYGO_WAILS_TAG" "$@"
    )
  else
    (
      cd "$ROOT_DIR/cmd/daygo"
      exec go run "github.com/wailsapp/wails/v2/cmd/wails@$WAILS_VERSION" \
        "$subcommand" -s "$@"
    )
  fi
}

# daygo_bootstrap runs the placeholder dist → bindings → real bundle pipeline.
# Defined unconditionally so callers can source this script and call it by name;
# skipped automatically when the script is executed directly (the direct path
# calls it at the bottom of the file).
#
# Two generated directories depend on each other:
#
#   frontend/dist/     go:embed all:dist — the Go module does not compile when
#                      the pattern matches no files, and `npm run build`
#                      (= vue-tsc + vite) cannot run until dist exists.
#   frontend/wailsjs/  imported by src/api/*.ts — vue-tsc and vite both fail
#                      without it, and generating it compiles the Go module.
#
# Order: placeholder dist → bindings → real bundle. Each step is skipped when
# its output already exists, so this is safe to run repeatedly.
daygo_bootstrap() {
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
  # point while leaving its assets/ in place, and the app then serves a blank
  # page.
  if [[ ! -f "$ROOT_DIR/frontend/dist/index.html" ]]; then
    printf '<!doctype html>\n' > "$ROOT_DIR/frontend/dist/index.html"
  fi
  (cd "$ROOT_DIR/cmd/daygo" \
    && go run "github.com/wailsapp/wails/v2/cmd/wails@$WAILS_VERSION" build -s -m -nopackage)

  # Build the real bundle. The test is the entry point's content, not the
  # presence of assets/: a real index.html references its hashed asset files
  # and a placeholder does not, whereas assets/ can survive a placeholder write
  # and would make this skip the rebuild that is needed.
  if ! grep -q 'assets/' "$ROOT_DIR/frontend/dist/index.html" 2>/dev/null; then
    printf 'Building frontend bundle for go:embed...\n'
    npm --prefix "$ROOT_DIR/frontend" run build
  fi
}

# ─── standalone role: run bootstrap and exit ───
#
# When sourced, _daygo_sourced=1 and this block is skipped — the caller
# decides when (or whether) to call daygo_bootstrap. When executed directly,
# _daygo_sourced=0 and this calls the pipeline then exits.
if [[ "$_daygo_sourced" -eq 0 ]]; then
  daygo_bootstrap
fi
