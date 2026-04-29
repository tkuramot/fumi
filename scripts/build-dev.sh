#!/usr/bin/env bash
# Build fumi, fumi-host, and the Chrome extension for local unpacked
# development. Patches dist/manifest.json with the dev "key" so the unpacked
# install resolves to the same ID baked into cmd/fumi/constants.go.
set -euo pipefail

cd "$(dirname "$0")/.."

host_bin="$PWD/bin/fumi-host"
echo "host binary path: $host_bin"

go build -ldflags "-X main.hostBinaryPath=$host_bin" -o ./bin/fumi ./cmd/fumi
go build -o ./bin/fumi-host ./cmd/fumi-host

pnpm -C chrome-extension install --frozen-lockfile >/dev/null
pnpm -C chrome-extension build

node scripts/inject-dev-key.mjs chrome-extension/dist/manifest.json

echo "done. load chrome-extension/dist/ as unpacked extension."
