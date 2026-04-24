#!/usr/bin/env bash
# Bump the latest vX.Y.Z git tag by patch/minor/major and print the new version (without the "v" prefix).
set -euo pipefail

bump="${1:-patch}"

latest=$(git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n1)
if [[ -z "$latest" ]]; then
  latest="v0.0.0"
fi

IFS='.' read -r major minor patch <<<"${latest#v}"

case "$bump" in
  major) major=$((major + 1)); minor=0; patch=0 ;;
  minor) minor=$((minor + 1)); patch=0 ;;
  patch) patch=$((patch + 1)) ;;
  *) echo "unknown bump type: $bump" >&2; exit 1 ;;
esac

new="${major}.${minor}.${patch}"
git tag "v${new}" >&2
echo "$new"
