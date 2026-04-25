#!/usr/bin/env bash
# Bump the version by patch/minor/major based on the latest git tag (v*),
# write the new value into chrome-extension/public/manifest.json, and print
# the new version (without the "v" prefix). Does not commit or tag.
set -euo pipefail

bump="${1:-patch}"
manifest="chrome-extension/public/manifest.json"

latest_tag=$(git tag --list 'v[0-9]*' --sort=-v:refname | head -n1)
if [[ -z "$latest_tag" ]]; then
  current="0.0.0"
else
  current="${latest_tag#v}"
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
