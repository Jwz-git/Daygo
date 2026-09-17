#!/usr/bin/env bash

set -euo pipefail

# ─────────────────────────────────────────────────────────────
# Daygo macOS Packager
#
# Usage:
#   ./scripts/package-macos.sh
#   ./scripts/package-macos.sh 0.1.0
#
# Universal:
#   DAYGO_ARCH=universal ./scripts/package-macos.sh 0.1.0
#
# Developer ID signing:
#   DAYGO_SIGN_IDENTITY="Developer ID Application: ..." \
#   ./scripts/package-macos.sh 0.1.0
#
# Notarization:
#   DAYGO_SIGN_IDENTITY="Developer ID Application: ..." \
#   DAYGO_NOTARY_PROFILE="daygo-notary" \
#   ./scripts/package-macos.sh 0.1.0
# ─────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Reuse Daygo existing helpers.
# shellcheck source=bootstrap-frontend.sh
source "$SCRIPT_DIR/bootstrap-frontend.sh"

APP_NAME="Daygo"

VERSION="${1:-dev}"
MACOS_MIN_VERSION="${DAYGO_MACOS_MIN_VERSION:-14.0}"

SIGN_IDENTITY="${DAYGO_SIGN_IDENTITY:-}"
NOTARY_PROFILE="${DAYGO_NOTARY_PROFILE:-}"

case "$(uname -m)" in
  arm64)
    DEFAULT_ARCH="arm64"
    ;;
  x86_64)
    DEFAULT_ARCH="amd64"
    ;;
  *)
    printf 'error: unsupported architecture: %s\n' "$(uname -m)" >&2
    exit 1
    ;;
esac

ARCH="${DAYGO_ARCH:-$DEFAULT_ARCH}"

APP_PATH="$ROOT_DIR/build/bin/$APP_NAME.app"
EXECUTABLE="$APP_PATH/Contents/MacOS/$APP_NAME"

DIST_DIR="$ROOT_DIR/dist"
STAGE_DIR="$DIST_DIR/stage"

BACKGROUND="$ROOT_DIR/build/darwin/dmg-background.png"

if [[ "$VERSION" == "dev" ]]; then
  DMG_NAME="Daygo.dmg"
else
  DMG_NAME="Daygo-${VERSION}-${ARCH}.dmg"
fi

DMG_PATH="$DIST_DIR/$DMG_NAME"

# ─────────────────────────────────────────────────────────────
# Pretty output
# ─────────────────────────────────────────────────────────────

bold="$(printf '\033[1m')"
dim="$(printf '\033[2m')"
green="$(printf '\033[32m')"
cyan="$(printf '\033[36m')"
yellow="$(printf '\033[33m')"
red="$(printf '\033[31m')"
reset="$(printf '\033[0m')"

step() {
  printf '\n%s%s━━━ %s%s\n' "$bold" "$cyan" "$1" "$reset"
}

success() {
  printf '%s✓%s %s\n' "$green" "$reset" "$1"
}

warn() {
  printf '%s!%s %s\n' "$yellow" "$reset" "$1"
}

fail() {
  printf '%s✗%s %s\n' "$red" "$reset" "$1" >&2
  exit 1
}

printf '\n'
printf '%s%s' "$bold" "$cyan"
printf '╭──────────────────────────────────────────────╮\n'
printf '│                                              │\n'
printf '│              Daygo Packager                  │\n'
printf '│                                              │\n'
printf '╰──────────────────────────────────────────────╯\n'
printf '%s\n' "$reset"

printf '  Version       %s%s%s\n' "$bold" "$VERSION" "$reset"
printf '  Architecture  %s%s%s\n' "$bold" "$ARCH" "$reset"
printf '  Minimum macOS %s%s%s\n' "$bold" "$MACOS_MIN_VERSION" "$reset"

if [[ -n "$SIGN_IDENTITY" ]]; then
  printf '  Signing       %sDeveloper ID%s\n' "$green" "$reset"
else
  printf '  Signing       %sAd-hoc / testing%s\n' "$yellow" "$reset"
fi

if [[ -n "$NOTARY_PROFILE" ]]; then
  printf '  Notarization  %senabled%s\n' "$green" "$reset"
else
  printf '  Notarization  %sdisabled%s\n' "$dim" "$reset"
fi

# ─────────────────────────────────────────────────────────────
# Environment
# ─────────────────────────────────────────────────────────────

step "Checking environment"

[[ "$(uname -s)" == "Darwin" ]] \
  || fail "macOS packages must be built on macOS."

require_tool go "Install Go."
require_tool npm "Install Node.js/npm."
require_tool xcrun "Run: xcode-select --install"
require_tool codesign "codesign is required."
require_tool create-dmg "Install it with: brew install create-dmg"

success "Build environment ready"

# ─────────────────────────────────────────────────────────────
# Bootstrap
# ─────────────────────────────────────────────────────────────

step "Preparing frontend"

daygo_bootstrap

success "Frontend and Wails bindings ready"

# ─────────────────────────────────────────────────────────────
# Cleanup
# ─────────────────────────────────────────────────────────────

step "Cleaning old builds"

rm -rf "$ROOT_DIR/build/bin"
rm -rf "$DIST_DIR"

mkdir -p "$DIST_DIR"
mkdir -p "$STAGE_DIR"

success "Workspace cleaned"

# ─────────────────────────────────────────────────────────────
# Build
# ─────────────────────────────────────────────────────────────

step "Building Daygo"

export MACOSX_DEPLOYMENT_TARGET="$MACOS_MIN_VERSION"

export CGO_CFLAGS="${CGO_CFLAGS:-} -mmacosx-version-min=$MACOS_MIN_VERSION"
export CGO_LDFLAGS="${CGO_LDFLAGS:-} -mmacosx-version-min=$MACOS_MIN_VERSION"

run_wails build \
  -clean \
  -platform "darwin/$ARCH"

success "Wails build completed"

# ─────────────────────────────────────────────────────────────
# Verify app bundle
# ─────────────────────────────────────────────────────────────

step "Inspecting application bundle"

[[ -d "$APP_PATH" ]] \
  || fail "$APP_PATH was not generated."

if [[ ! -f "$EXECUTABLE" ]]; then
  printf '\nContents/MacOS:\n'
  ls -lah "$APP_PATH/Contents/MacOS" || true
  printf '\n'

  fail "Daygo executable is missing. DMG creation aborted."
fi

chmod +x "$EXECUTABLE"

success "Executable exists"

printf '\n%sBinary:%s\n' "$dim" "$reset"
file "$EXECUTABLE"

# ─────────────────────────────────────────────────────────────
# Info.plist
# ─────────────────────────────────────────────────────────────

step "Updating application metadata"

PLIST="$APP_PATH/Contents/Info.plist"

/usr/libexec/PlistBuddy \
  -c "Set :LSMinimumSystemVersion $MACOS_MIN_VERSION" \
  "$PLIST"

if [[ "$VERSION" != "dev" ]]; then
  /usr/libexec/PlistBuddy \
    -c "Set :CFBundleShortVersionString $VERSION" \
    "$PLIST"

  # CFBundleVersion should technically be a monotonically increasing
  # build number. For early Daygo releases we keep it aligned.
  /usr/libexec/PlistBuddy \
    -c "Set :CFBundleVersion $VERSION" \
    "$PLIST"
fi

success "Minimum system set to macOS $MACOS_MIN_VERSION"

# ─────────────────────────────────────────────────────────────
# Signing
# ─────────────────────────────────────────────────────────────

step "Signing application"

# Remove whatever temporary Wails signature exists.
codesign --remove-signature "$APP_PATH" 2>/dev/null || true

if [[ -n "$SIGN_IDENTITY" ]]; then

  codesign \
    --force \
    --deep \
    --options runtime \
    --timestamp \
    --sign "$SIGN_IDENTITY" \
    "$APP_PATH"

  success "Signed with Developer ID"

else

  codesign \
    --force \
    --deep \
    --sign - \
    "$APP_PATH"

  success "Ad-hoc signed"

  warn "Suitable for local testing, not public distribution."

fi

# ─────────────────────────────────────────────────────────────
# Verification
# ─────────────────────────────────────────────────────────────

step "Validating Daygo.app"

codesign \
  --verify \
  --deep \
  --strict \
  --verbose=2 \
  "$APP_PATH"

BUNDLE_EXECUTABLE="$(
  /usr/libexec/PlistBuddy \
    -c "Print :CFBundleExecutable" \
    "$PLIST"
)"

[[ "$BUNDLE_EXECUTABLE" == "$APP_NAME" ]] \
  || fail "CFBundleExecutable is '$BUNDLE_EXECUTABLE'."

success "Application bundle valid"

# ─────────────────────────────────────────────────────────────
# Prepare DMG
# ─────────────────────────────────────────────────────────────

step "Preparing installer"

ditto \
  "$APP_PATH" \
  "$STAGE_DIR/$APP_NAME.app"

success "Application copied to installer stage"

# ─────────────────────────────────────────────────────────────
# DMG
# ─────────────────────────────────────────────────────────────

step "Creating beautiful DMG"

CREATE_DMG_ARGS=(
  --volname "Daygo"
  --window-pos 200 120
  --window-size 660 420
  --icon-size 112

  --icon "Daygo.app" 175 210
  --hide-extension "Daygo.app"

  --app-drop-link 485 210

  --no-internet-enable
)

if [[ -f "$BACKGROUND" ]]; then
  CREATE_DMG_ARGS+=(
    --background "$BACKGROUND"
  )
else
  warn "No custom DMG background found:"
  printf '  %s\n' "$BACKGROUND"
  printf '  Using standard Finder background.\n'
fi

rm -f "$DMG_PATH"

create-dmg \
  "${CREATE_DMG_ARGS[@]}" \
  "$DMG_PATH" \
  "$STAGE_DIR"

success "DMG created"

# ─────────────────────────────────────────────────────────────
# DMG signing
# ─────────────────────────────────────────────────────────────

if [[ -n "$SIGN_IDENTITY" ]]; then

  step "Signing DMG"

  codesign \
    --force \
    --timestamp \
    --sign "$SIGN_IDENTITY" \
    "$DMG_PATH"

  codesign \
    --verify \
    --verbose=2 \
    "$DMG_PATH"

  success "DMG signed"

fi

# ─────────────────────────────────────────────────────────────
# Notarization
# ─────────────────────────────────────────────────────────────

if [[ -n "$NOTARY_PROFILE" ]]; then

  [[ -n "$SIGN_IDENTITY" ]] \
    || fail "Notarization requires Developer ID signing."

  step "Submitting to Apple notarization"

  xcrun notarytool submit \
    "$DMG_PATH" \
    --keychain-profile "$NOTARY_PROFILE" \
    --wait

  success "Apple notarization accepted"

  step "Stapling notarization ticket"

  xcrun stapler staple "$DMG_PATH"
  xcrun stapler validate "$DMG_PATH"

  success "Notarization ticket stapled"

fi

# ─────────────────────────────────────────────────────────────
# Cleanup
# ─────────────────────────────────────────────────────────────

rm -rf "$STAGE_DIR"

# ─────────────────────────────────────────────────────────────
# Finish
# ─────────────────────────────────────────────────────────────

SIZE="$(du -h "$DMG_PATH" | awk '{print $1}')"

printf '\n%s%s' "$bold" "$green"
printf '╭──────────────────────────────────────────────╮\n'
printf '│                                              │\n'
printf '│              Package complete ✓              │\n'
printf '│                                              │\n'
printf '╰──────────────────────────────────────────────╯\n'
printf '%s\n' "$reset"

printf '  DMG   %s\n' "$DMG_PATH"
printf '  Size  %s\n' "$SIZE"
printf '\n'

printf 'Open installer:\n\n'
printf '  %sopen "%s"%s\n\n' "$cyan" "$DMG_PATH" "$reset"