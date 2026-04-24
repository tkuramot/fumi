#!/usr/bin/env bash
# Build fumi, fumi-host, and the Chrome extension wired together for local
# unpacked development. The extension ID is derived from the "key" committed
# in chrome-extension/public/manifest.json, so dev and release share a single
# identity — no local keygen needed.
set -euo pipefail

cd "$(dirname "$0")/.."

HOST_BIN_PATH="$(pwd)/bin/fumi-host"
echo "host binary path: $HOST_BIN_PATH"

go build -ldflags "-X main.hostBinaryPath=$HOST_BIN_PATH" -o ./bin/fumi ./cmd/fumi
go build -o ./bin/fumi-host ./cmd/fumi-host

pnpm -C chrome-extension install --frozen-lockfile >/dev/null
pnpm -C chrome-extension build

echo "done. load chrome-extension/dist/ as unpacked extension."
