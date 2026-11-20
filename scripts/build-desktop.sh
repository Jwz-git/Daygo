#!/usr/bin/env bash
# Compile the v3 desktop host. Produces a development bundle/executable only;
# release signing, installers and updates remain in package-*.sh/ps1.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/bootstrap-frontend.sh"
cd "$ROOT_DIR"
DAYGO_BUILD_OS="$(go env GOOS)"
DAYGO_BUILD_ARCH="${DAYGO_ARCH:-$(go env GOARCH)}"
DAYGO_BUILD_TAGS="daygo_desktop"
DAYGO_LINKER_ARGS=()
if [[ "${DAYGO_DEV:-0}" != 1 ]]; then
  DAYGO_BUILD_TAGS+=",production"
  DAYGO_LINKER_ARGS=(-ldflags '-s -w')
fi
if [[ -n "${DAYGO_EXTRA_TAGS:-}" ]]; then DAYGO_BUILD_TAGS+=",$DAYGO_EXTRA_TAGS"; fi
if [[ -n "${EXTRA_TAGS:-}" ]]; then DAYGO_BUILD_TAGS+=",$EXTRA_TAGS"; fi
if [[ "${DAYGO_SKIP_FRONTEND:-0}" != 1 ]]; then
  if [[ ! -d frontend/node_modules ]]; then npm --prefix frontend ci --no-audit --no-fund; fi
  daygo_bootstrap
  if [[ "${DAYGO_DEV:-0}" != 1 ]]; then npm --prefix frontend run build; fi
fi
mkdir -p build/bin
case "$DAYGO_BUILD_OS" in
  darwin)
    export MACOSX_DEPLOYMENT_TARGET="${MACOSX_DEPLOYMENT_TARGET:-14.0}"
    export CGO_CFLAGS="${CGO_CFLAGS:-} -mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET"
    export CGO_LDFLAGS="${CGO_LDFLAGS:-} -mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET"
    native/darwin/build.sh
    DAYGO_APP_PATH="$ROOT_DIR/build/bin/Daygo.app"
    mkdir -p "$DAYGO_APP_PATH/Contents/MacOS" "$DAYGO_APP_PATH/Contents/Resources"
    if [[ "$DAYGO_BUILD_ARCH" == universal ]]; then
      for DAYGO_TARGET_ARCH in arm64 amd64; do
        CGO_ENABLED=1 GOARCH="$DAYGO_TARGET_ARCH" go build -tags "$DAYGO_BUILD_TAGS" ${DAYGO_LINKER_ARGS[@]+"${DAYGO_LINKER_ARGS[@]}"} -o "build/bin/Daygo-$DAYGO_TARGET_ARCH" ./cmd/daygo
      done
      lipo -create -output "$DAYGO_APP_PATH/Contents/MacOS/Daygo" build/bin/Daygo-arm64 build/bin/Daygo-amd64
      rm build/bin/Daygo-arm64 build/bin/Daygo-amd64
    else
      CGO_ENABLED=1 GOARCH="$DAYGO_BUILD_ARCH" go build -tags "$DAYGO_BUILD_TAGS" ${DAYGO_LINKER_ARGS[@]+"${DAYGO_LINKER_ARGS[@]}"} -o "$DAYGO_APP_PATH/Contents/MacOS/Daygo" ./cmd/daygo
    fi
    cp build/darwin/Info.plist "$DAYGO_APP_PATH/Contents/Info.plist"
    DAYGO_BUILD_VERSION="${DAYGO_VERSION:-0.0.0}"
    /usr/libexec/PlistBuddy -c "Set :CFBundleVersion $DAYGO_BUILD_VERSION" "$DAYGO_APP_PATH/Contents/Info.plist"
    /usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $DAYGO_BUILD_VERSION" "$DAYGO_APP_PATH/Contents/Info.plist"
    run_wails generate icons -input build/appicon.png -macfilename "$DAYGO_APP_PATH/Contents/Resources/iconfile.icns" -windowsfilename "$DAYGO_APP_PATH/Contents/Resources/unused.ico"
    rm "$DAYGO_APP_PATH/Contents/Resources/unused.ico"
    codesign --force --sign - "$DAYGO_APP_PATH"
    ;;
  linux)
    DAYGO_GTK_TAG="$(webkit_tag)"
    if [[ -n "$DAYGO_GTK_TAG" ]]; then DAYGO_BUILD_TAGS+=",$DAYGO_GTK_TAG"; fi
    CGO_ENABLED=1 GOARCH="$DAYGO_BUILD_ARCH" go build -tags "$DAYGO_BUILD_TAGS" ${DAYGO_LINKER_ARGS[@]+"${DAYGO_LINKER_ARGS[@]}"} -o build/bin/Daygo ./cmd/daygo
    ;;
  *) printf 'error: use scripts/build.ps1 on Windows\n' >&2; exit 1 ;;
esac
