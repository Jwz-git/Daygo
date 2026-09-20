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
# The script is idempotent: if the identity already exists it does nothing.

set -euo pipefail

CERT_NAME="${DAYGO_DEV_CERT_NAME:-Daygo Dev}"
KEYCHAIN="${DAYGO_DEV_KEYCHAIN:-$HOME/Library/Keychains/login.keychain-db}"

if [[ "$(uname -s)" != "Darwin" ]]; then
  printf 'error: this script only runs on macOS\n' >&2
  exit 1
fi

if security find-identity -v -p codesigning "$KEYCHAIN" | grep -qF "\"$CERT_NAME\""; then
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

# Bundle key + cert into a passwordless PKCS#12 for import.
openssl pkcs12 -export \
  -inkey "$WORK_DIR/key.pem" \
  -in "$WORK_DIR/cert.pem" \
  -out "$WORK_DIR/identity.p12" \
  -name "$CERT_NAME" \
  -passout pass:

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
