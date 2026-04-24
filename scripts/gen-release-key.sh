#!/usr/bin/env bash
# One-time setup: generate the release key pair and patch the committed
# manifest.json / constants.go with the derived public values.
#
# Both PUBKEY_B64 and EXT_ID are public — they ship in every released
# extension, so they belong in source, not in CI secrets.
#
# The private release-key.pem is git-ignored. You only need it to keep the
# same extension ID if you ever rotate keys; Chrome Web Store itself does
# not require the private key.
set -euo pipefail

cd "$(dirname "$0")/.."

KEY_PEM="release-key.pem"
MANIFEST="chrome-extension/public/manifest.json"
CONSTANTS="cmd/fumi/constants.go"

if [[ -e "$KEY_PEM" ]]; then
  echo "error: $KEY_PEM already exists; refusing to overwrite" >&2
  exit 1
fi

if ! grep -q "REPLACE_WITH_BASE64_PUBLIC_KEY" "$MANIFEST"; then
  echo "error: $MANIFEST has no placeholder — already patched?" >&2
  exit 1
fi
if ! grep -q "REPLACE_WITH_EXTENSION_ID" "$CONSTANTS"; then
  echo "error: $CONSTANTS has no placeholder — already patched?" >&2
  exit 1
fi

openssl genrsa -out "$KEY_PEM" 2048 2>/dev/null
chmod 600 "$KEY_PEM"

PUBKEY_B64=$(openssl rsa -in "$KEY_PEM" -pubout -outform DER 2>/dev/null | base64 | tr -d '\n')
EXT_ID=$(openssl rsa -in "$KEY_PEM" -pubout -outform DER 2>/dev/null \
  | openssl dgst -sha256 -binary \
  | head -c 16 \
  | xxd -p -c 32 \
  | tr '0-9a-f' 'a-p')

PUBKEY_B64="$PUBKEY_B64" perl -i -pe 's|REPLACE_WITH_BASE64_PUBLIC_KEY|$ENV{PUBKEY_B64}|' "$MANIFEST"
EXT_ID="$EXT_ID" perl -i -pe 's|REPLACE_WITH_EXTENSION_ID|$ENV{EXT_ID}|' "$CONSTANTS"

cat >&2 <<EOF
Generated $KEY_PEM (git-ignored — back it up to 1Password / offline storage).

Patched:
  - $MANIFEST     (manifest "key")
  - $CONSTANTS    (Go extensionID)

Next steps:
  1. Review and commit the two patched files.
  2. When first uploading to Chrome Web Store, upload the ZIP with the
     "key" field intact so CWS preserves this extension ID.

Extension ID: $EXT_ID
EOF
