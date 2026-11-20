#!/usr/bin/env bash
# Shared Wails v3 tool pin and clean-checkout bootstrap. Sourcing only defines
# helpers; executing generates bindings and builds a missing frontend bundle.
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WAILS_VERSION="v3.1.0"
require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'error: %s is required. %s\n' "$1" "${2:-}" >&2
    exit 1
  fi
}
# GTK4/WebKitGTK6 is the v3 default. GTK3/4.1 remains an explicit legacy path.
webkit_tag() {
  if [[ -n "${DAYGO_LINUX_WEBKIT_TAG:-}" ]]; then
    case "$DAYGO_LINUX_WEBKIT_TAG" in gtk3) printf 'gtk3\n'; return ;; *) printf 'error: only gtk3 is a valid legacy v3 tag\n' >&2; return 1 ;; esac
  fi
  require_tool pkg-config
  if pkg-config --exists gtk4 webkitgtk-6.0; then return; fi
  if pkg-config --exists gtk+-3.0 webkit2gtk-4.1; then printf 'gtk3\n'; return; fi
  printf 'error: install GTK4 + WebKitGTK 6.0 development headers (or GTK3 + WebKit2GTK 4.1).\n' >&2
  return 1
}
run_wails() {
  (cd "$ROOT_DIR" && go run "github.com/wailsapp/wails/v3/cmd/wails3@$WAILS_VERSION" "$@")
}
daygo_bootstrap() {
  mkdir -p "$ROOT_DIR/frontend/dist"
  if [[ ! -f "$ROOT_DIR/frontend/dist/index.html" ]]; then printf '<!doctype html>\n' > "$ROOT_DIR/frontend/dist/index.html"; fi
  # server is used only to analyse service types without native SDKs. Daygo
  # does not build or expose a server-mode product.
  # The CLI itself otherwise imports the Linux WebKit probe via cgo, even
  # though the analysed application uses server tags and needs no native SDK.
  CGO_ENABLED=0 run_wails generate bindings -ts -i -names -f '-tags=daygo_desktop,server' -d frontend/bindings ./cmd/daygo
  if ! [[ -f "$ROOT_DIR/frontend/dist/index.html" ]] || ! grep -q 'assets/' "$ROOT_DIR/frontend/dist/index.html"; then
    npm --prefix "$ROOT_DIR/frontend" run build
  fi
}
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  require_tool go; require_tool npm
  daygo_bootstrap
fi
