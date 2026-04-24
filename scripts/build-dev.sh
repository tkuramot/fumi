#!/usr/bin/env bash
# Build fumi, fumi-host, and the Chrome extension wired together for local
# unpacked development. Requires .dev/key.pem (created by `make keygen`).
set -euo pipefail

cd "$(dirname "$0")/.."

KEY_PEM=".dev/key.pem"
if [[ ! -f "$KEY_PEM" ]]; then
	echo "error: $KEY_PEM missing; run 'make keygen' first" >&2
	exit 1
fi

# SubjectPublicKeyInfo DER, base64-encoded — value of manifest.json "key".
PUBKEY_B64=$(openssl rsa -in "$KEY_PEM" -pubout -outform DER 2>/dev/null | base64 | tr -d '\n')

# Chrome extension ID: SHA-256 of the DER public key, first 16 bytes hex,
# mapped 0-9a-f → a-p.
EXT_ID=$(openssl rsa -in "$KEY_PEM" -pubout -outform DER 2>/dev/null \
	| openssl dgst -sha256 -binary \
	| head -c 16 \
	| xxd -p -c 32 \
	| tr '0-9a-f' 'a-p')

HOST_BIN_PATH="$(pwd)/bin/fumi-host"

echo "unpacked extension ID: $EXT_ID"
echo "host binary path:      $HOST_BIN_PATH"

LDFLAGS="-X main.unpackedExtensionID=$EXT_ID -X main.hostBinaryPath=$HOST_BIN_PATH"
go build -ldflags "$LDFLAGS" -o ./bin/fumi ./cmd/fumi
go build -o ./bin/fumi-host ./cmd/fumi-host

pnpm -C chrome-extension install --frozen-lockfile >/dev/null
pnpm -C chrome-extension build

MANIFEST="chrome-extension/dist/manifest.json"
# Inject the real public key into the built manifest (never the source).
# '|' is safe as a sed delimiter — base64 alphabet is [A-Za-z0-9+/=].
sed -i '' "s|REPLACE_WITH_BASE64_PUBLIC_KEY|$PUBKEY_B64|" "$MANIFEST"

echo "$EXT_ID" > .dev/extension-id
echo "done. load chrome-extension/dist/ as unpacked extension."
