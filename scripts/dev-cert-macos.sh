#!/usr/bin/env bash

# Create a local self-signed code-signing identity for Daygo.
#
# Why this exists: macOS keys a Screen Recording (TCC) grant to the app's
# designated requirement. An ad-hoc signature (`codesign -s -`, the default of
# a bare `wails build`) has no certificate, so its requirement degrades to the
# binary's cdhash — which changes on every build. Every rebuild then looks like
# a new app and the previous grant is orphaned, so recording silently stops.
#
# A certificate-backed signature makes the requirement
# `identifier "io.github.jwz-git.Daygo" and certificate leaf ...`, which is
# stable across rebuilds. This identity is self-signed: it needs no Apple
# account, but it does NOT notarize and is NOT for distribution.
#
# Usage:
#   ./scripts/dev-cert-macos.sh
#   DAYGO_DEV_CERT_NAME="Daygo Dev" ./scripts/dev-cert-macos.sh
#
# Then build with it:
#   DAYGO_DEV_SIGN_IDENTITY="Daygo Dev" ./scripts/package-macos.sh
#
# Export mode (for CI): set DAYGO_DEV_CERT_P12_OUT to a path OUTSIDE the repo to
# also emit a password-protected .p12 and print its base64 + password, for the
# GitHub Secrets DAYGO_DEV_CERT_P12_BASE64 / DAYGO_DEV_CERT_P12_PASSWORD that
# publish-release.yml imports. Run this once and reuse the same secret for every
# release: CI signs every build with that one certificate, so the designated
# requirement — and the Screen Recording grant keyed to it — stays stable across
# updates.
#
#   DAYGO_DEV_CERT_P12_OUT=/tmp/daygo-ci.p12 ./scripts/dev-cert-macos.sh
#
# The script is idempotent: if the identity already exists it does nothing
# (export mode still emits a fresh CI certificate — see the note it prints).

set -euo pipefail

CERT_NAME="${DAYGO_DEV_CERT_NAME:-Daygo Dev}"
KEYCHAIN="${DAYGO_DEV_KEYCHAIN:-$HOME/Library/Keychains/login.keychain-db}"
P12_OUT="${DAYGO_DEV_CERT_P12_OUT:-}"

if [[ "$(uname -s)" != "Darwin" ]]; then
  printf 'error: this script only runs on macOS\n' >&2
  exit 1
fi

# The exported .p12 holds the signing private key. Refuse to write it inside the
# repository so it can never be committed (keys never live in Git-tracked files).
if [[ -n "$P12_OUT" ]]; then
  ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
  OUT_DIR="$(cd "$(dirname "$P12_OUT")" && pwd)"
  case "$OUT_DIR/" in
    "$ROOT_DIR/"*)
      printf 'error: DAYGO_DEV_CERT_P12_OUT (%s) is inside the repo.\n' "$P12_OUT" >&2
      printf '       Choose a path outside the working tree, e.g. /tmp/daygo-ci.p12.\n' >&2
      exit 1
      ;;
  esac
fi

identity_exists() {
  security find-identity -v -p codesigning "$KEYCHAIN" | grep -qF "\"$CERT_NAME\""
}

# Without export mode, an already-present identity means there is nothing to do.
if [[ -z "$P12_OUT" ]] && identity_exists; then
  printf 'Identity "%s" already exists in %s — nothing to do.\n' "$CERT_NAME" "$KEYCHAIN"
  printf 'Build with:\n\n  DAYGO_DEV_SIGN_IDENTITY="%s" ./scripts/package-macos.sh\n' "$CERT_NAME"
  exit 0
fi

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

# A code-signing cert macOS accepts needs the codeSigning extended key usage
# and digitalSignature key usage, both marked critical.
cat > "$WORK_DIR/cert.conf" <<'EOF'
[ req ]
distinguished_name = req_dn
x509_extensions    = v3_codesign
prompt             = no
[ req_dn ]
CN = DAYGO_CERT_CN
[ v3_codesign ]
basicConstraints       = critical, CA:false
keyUsage               = critical, digitalSignature
extendedKeyUsage       = critical, codeSigning
EOF
# Substitute the CN without risking shell-quoting in the heredoc.
/usr/bin/sed -i '' "s/DAYGO_CERT_CN/${CERT_NAME}/" "$WORK_DIR/cert.conf"

printf 'Generating self-signed code-signing certificate "%s"...\n' "$CERT_NAME"
openssl req -x509 -newkey rsa:2048 -sha256 -days 3650 -nodes \
  -keyout "$WORK_DIR/key.pem" \
  -out "$WORK_DIR/cert.pem" \
  -config "$WORK_DIR/cert.conf"

# Bundle key + cert into a passwordless PKCS#12 for local import.
openssl pkcs12 -export \
  -inkey "$WORK_DIR/key.pem" \
  -in "$WORK_DIR/cert.pem" \
  -out "$WORK_DIR/identity.p12" \
  -name "$CERT_NAME" \
  -passout pass:

# Export mode: emit a password-protected copy for GitHub Secrets. Done before the
# local import so it works even when the login keychain already holds the name.
if [[ -n "$P12_OUT" ]]; then
  P12_PASSWORD="$(openssl rand -base64 24)"
  openssl pkcs12 -export \
    -inkey "$WORK_DIR/key.pem" \
    -in "$WORK_DIR/cert.pem" \
    -out "$P12_OUT" \
    -name "$CERT_NAME" \
    -passout "pass:$P12_PASSWORD"
  printf '\n=== GitHub Secrets (copy these, then delete %s) ===\n\n' "$P12_OUT"
  printf 'DAYGO_DEV_CERT_P12_BASE64:\n%s\n\n' "$(base64 < "$P12_OUT" | tr -d '\n')"
  printf 'DAYGO_DEV_CERT_P12_PASSWORD:\n%s\n\n' "$P12_PASSWORD"
  printf 'Set both in the repo Settings > Secrets and variables > Actions.\n'
  printf 'publish-release.yml signs releases with "%s" when they are present.\n\n' "$CERT_NAME"
fi

if identity_exists; then
  printf 'A "%s" identity is already in %s; skipping local import.\n' "$CERT_NAME" "$KEYCHAIN"
  if [[ -n "$P12_OUT" ]]; then
    printf 'Note: the exported .p12 is a NEW, independent certificate. To share ONE\n'
    printf 'identity between local builds and CI (a single Screen Recording grant),\n'
    printf 'remove the existing identity (security delete-identity -c "%s") and re-run.\n' "$CERT_NAME"
  fi
  exit 0
fi

# Import into the login keychain, pre-authorizing codesign to use the private
# key (-T) so signing does not raise a UI prompt on every build.
printf 'Importing into %s (you may be asked for your keychain password)...\n' "$KEYCHAIN"
security import "$WORK_DIR/identity.p12" \
  -k "$KEYCHAIN" \
  -P "" \
  -T /usr/bin/codesign

# Grant codesign non-interactive access to the key. This needs the keychain
# password; security will prompt if it is not supplied here.
printf 'Authorizing codesign to use the key (enter your login keychain password if prompted)...\n'
if ! security set-key-partition-list \
  -S "apple-tool:,apple:,codesign:" \
  -k "" \
  "$KEYCHAIN" >/dev/null 2>&1; then
  read -r -s -p "  login keychain password: " KEYCHAIN_PASSWORD
  printf '\n'
  security set-key-partition-list \
    -S "apple-tool:,apple:,codesign:" \
    -k "$KEYCHAIN_PASSWORD" \
    "$KEYCHAIN" >/dev/null
fi

printf '\nDone. Identity "%s" is ready.\n\n' "$CERT_NAME"
printf 'Build a stably-signed app with:\n\n'
printf '  DAYGO_DEV_SIGN_IDENTITY="%s" ./scripts/package-macos.sh\n\n' "$CERT_NAME"
printf 'After installing, grant Screen Recording once and restart Daygo.\n'
printf 'Because the identity is stable, later rebuilds keep that grant.\n'
