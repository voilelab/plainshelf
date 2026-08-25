#!/usr/bin/env bash
set -euo pipefail

if [ $# -lt 1 ]; then
  echo "Usage: $0 <version>   e.g. $0 v0.7.0" >&2
  exit 1
fi

VERSION_NUM="${1#v}"
VERSION_TAG="v${VERSION_NUM}"

BASE_URL="https://github.com/voilelab/plainshelf/releases/download/${VERSION_TAG}"

# Both casks ship from the same release at the same tag, and the plainshelf
# cask depends on bookpkg-reader at the matching version, so they are pinned
# together to keep the pair in sync. Format: "<cask file>:<release asset>".
CASKS=(
  "Casks/plainshelf.rb:plainshelf-desktop_${VERSION_TAG}_darwin_arm64.zip"
  "Casks/bookpkg-reader.rb:bookpkg-reader_${VERSION_TAG}_darwin_arm64.zip"
)

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

sed_inplace() {
  # BSD sed (macOS) needs an argument to -i; GNU sed does not.
  if sed --version >/dev/null 2>&1; then
    sed -i "$@"
  else
    sed -i '' "$@"
  fi
}

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

for entry in "${CASKS[@]}"; do
  cask_file="${entry%%:*}"
  asset="${entry#*:}"
  url="${BASE_URL}/${asset}"

  echo "Downloading $url"
  curl -fSL -o "$tmp" "$url"

  sha=$(sha256_of "$tmp")
  sed_inplace "s/^  version \".*\"/  version \"${VERSION_NUM}\"/" "$cask_file"
  sed_inplace "s/^  sha256 \".*\"/  sha256 \"${sha}\"/" "$cask_file"

  echo "Updated ${cask_file} -> version ${VERSION_NUM}, sha256 ${sha}"
done
