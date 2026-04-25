#!/usr/bin/env bash
# Bump chrome-extension/public/manifest.json by patch/minor/major, write it back,
# and print the new version (without the "v" prefix). Does not touch git.
set -euo pipefail

bump="${1:-patch}"
manifest="chrome-extension/public/manifest.json"

current=$(perl -ne 'print $1 if /"version"\s*:\s*"([^"]+)"/' "$manifest")
if [[ -z "$current" ]]; then
  echo "could not read version from $manifest" >&2
  exit 1
fi

IFS='.' read -r major minor patch <<<"$current"

case "$bump" in
  major) major=$((major + 1)); minor=0; patch=0 ;;
  minor) minor=$((minor + 1)); patch=0 ;;
  patch) patch=$((patch + 1)) ;;
  *) echo "unknown bump type: $bump" >&2; exit 1 ;;
esac

new="${major}.${minor}.${patch}"
NEW="$new" perl -i -pe 's|("version"\s*:\s*")[^"]*(")|$1$ENV{NEW}$2|' "$manifest"
echo "$new"
