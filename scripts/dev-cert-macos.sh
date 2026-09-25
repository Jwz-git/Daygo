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
# Export mode (for CI): set DAYGO_DEV_CERT_P12_OUT to a path OUTSIDE the repo.
# The script writes a password-protected .p12 and a separate .password file,
# without printing either secret. Upload them as the release environment secrets
# DAYGO_DEV_CERT_P12_BASE64 / DAYGO_DEV_CERT_P12_PASSWORD. Run this once and
# reuse the same identity for every release.
#
#   DAYGO_DEV_CERT_P12_OUT=/tmp/daygo-ci.p12 ./scripts/dev-cert-macos.sh
#
# Without export mode, an existing identity is left alone. Export mode refuses
# to create a second certificate when the local identity already exists.

set -euo pipefail
umask 077

CERT_NAME="${DAYGO_DEV_CERT_NAME:-Daygo Dev}"
KEYCHAIN="${DAYGO_DEV_KEYCHAIN:-$HOME/Library/Keychains/login.keychain-db}"
P12_OUT="${DAYGO_DEV_CERT_P12_OUT:-}"
PASSWORD_OUT="${DAYGO_DEV_CERT_PASSWORD_OUT:-${P12_OUT:+$P12_OUT.password}}"

if [[ "$(uname -s)" != "Darwin" ]]; then
  printf 'error: this script only runs on macOS\n' >&2
  exit 1
fi

# The exported .p12 holds the signing private key. Refuse to write it inside the
# repository so it can never be committed (keys never live in Git-tracked files).
if [[ -n "$P12_OUT" ]]; then
  ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
  for out_path in "$P12_OUT" "$PASSWORD_OUT"; do
    OUT_DIR="$(cd "$(dirname "$out_path")" && pwd)"
    case "$OUT_DIR/" in
      "$ROOT_DIR/"*)
        printf 'error: secret output (%s) is inside the repo.\n' "$out_path" >&2
        exit 1
        ;;
    esac
    if [[ -e "$out_path" ]]; then
      printf 'error: secret output already exists: %s\n' "$out_path" >&2
      exit 1
    fi
  done
fi

identity_exists() {
  # A self-signed identity may be listed as NOT_TRUSTED by -v even though
  # codesign can use it. Match all code-signing identities by name instead.
  security find-identity -p codesigning "$KEYCHAIN" | grep -qF "\"$CERT_NAME\""
}

# Without export mode, an already-present identity means there is nothing to do.
if [[ -z "$P12_OUT" ]] && identity_exists; then
  printf 'Identity "%s" already exists in %s — nothing to do.\n' "$CERT_NAME" "$KEYCHAIN"
  printf 'Build with:\n\n  DAYGO_DEV_SIGN_IDENTITY="%s" ./scripts/package-macos.sh\n' "$CERT_NAME"
  exit 0
fi

if [[ -n "$P12_OUT" ]] && identity_exists; then
  printf 'error: identity "%s" already exists; refusing to create a different CI identity.\n' "$CERT_NAME" >&2
  printf '       Export the existing identity or choose a deliberate migration first.\n' >&2
  exit 1
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

# OpenSSL 3's default PBES2/SHA-256 PKCS#12 fails macOS Keychain import with
# "MAC verification failed". Use a password-protected, macOS-compatible 3DES
# container for both local import and the CI backup.
P12_PASSWORD="$(openssl rand -base64 24)"
printf '%s' "$P12_PASSWORD" > "$WORK_DIR/identity.password"
openssl pkcs12 -export \
  -legacy \
  -keypbe PBE-SHA1-3DES \
  -certpbe PBE-SHA1-3DES \
  -macalg sha1 \
  -inkey "$WORK_DIR/key.pem" \
  -in "$WORK_DIR/cert.pem" \
  -out "$WORK_DIR/identity.p12" \
  -name "$CERT_NAME" \
  -passout "file:$WORK_DIR/identity.password"

# Export mode: emit a password-protected copy for GitHub Secrets before the
# local import. Never print the private key or password to terminal output.
if [[ -n "$P12_OUT" ]]; then
  cp "$WORK_DIR/identity.p12" "$P12_OUT"
  cp "$WORK_DIR/identity.password" "$PASSWORD_OUT"
  printf 'Created encrypted PKCS#12: %s\n' "$P12_OUT"
  printf 'Created password file: %s\n' "$PASSWORD_OUT"
  printf 'Store both outside the repository and upload them to the release environment secrets.\n'
fi

if identity_exists; then
  printf 'A "%s" identity is already in %s; skipping local import.\n' "$CERT_NAME" "$KEYCHAIN"
  exit 0
fi

# Import into the login keychain, pre-authorizing codesign to use the private
# key (-T) so signing does not raise a UI prompt on every build.
printf 'Importing into %s (you may be asked for your keychain password)...\n' "$KEYCHAIN"
security import "$WORK_DIR/identity.p12" \
  -k "$KEYCHAIN" \
  -P "$P12_PASSWORD" \
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
