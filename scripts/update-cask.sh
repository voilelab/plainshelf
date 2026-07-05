#!/usr/bin/env bash
set -euo pipefail

if [ $# -lt 1 ]; then
  echo "Usage: $0 <version>   e.g. $0 v0.7.0" >&2
  exit 1
fi

VERSION_NUM="${1#v}"
VERSION_TAG="v${VERSION_NUM}"

CASK_FILE="Casks/plainshelf.rb"
ASSET="plainshelf-desktop_${VERSION_TAG}_darwin_arm64.zip"
URL="https://github.com/voilelab/plainshelf/releases/download/${VERSION_TAG}/${ASSET}"

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

echo "Downloading $URL"
curl -fSL -o "$tmp" "$URL"

if [[ "$OSTYPE" == darwin* ]]; then
  SHA=$(shasum -a 256 "$tmp" | awk '{print $1}')
  sed -i '' "s/^  version \".*\"/  version \"${VERSION_NUM}\"/" "$CASK_FILE"
  sed -i '' "s/^  sha256 \".*\"/  sha256 \"${SHA}\"/" "$CASK_FILE"
else
  SHA=$(sha256sum "$tmp" | awk '{print $1}')
  sed -i "s/^  version \".*\"/  version \"${VERSION_NUM}\"/" "$CASK_FILE"
  sed -i "s/^  sha256 \".*\"/  sha256 \"${SHA}\"/" "$CASK_FILE"
fi

echo "Updated ${CASK_FILE} -> version ${VERSION_NUM}, sha256 ${SHA}"
