#!/usr/bin/env bash
set -euo pipefail

if [ $# -lt 1 ]; then
  echo "Usage: $0 <version>   e.g. $0 v0.7.0" >&2
  exit 1
fi

VERSION_NUM="${1#v}"
VERSION_TAG="v${VERSION_NUM}"

BASE_URL="https://github.com/voilelab/plainshelf/releases/download/${VERSION_TAG}"

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
  # BSD sed requires a suffix argument for -i; GNU sed accepts an omitted suffix (sed(1)).
  if sed --version >/dev/null 2>&1; then
    sed -i "$@"
  else
    sed -i '' "$@"
  fi
}

artifact_download=$(mktemp)
trap 'rm -f "$artifact_download"' EXIT

downloaded_sha256s=()
for entry in "${CASKS[@]}"; do
  asset="${entry#*:}"
  url="${BASE_URL}/${asset}"

  echo "Downloading $url"
  curl -fSL -o "$artifact_download" "$url"
  downloaded_sha256s+=("$(sha256_of "$artifact_download")")
done

for i in "${!CASKS[@]}"; do
  cask_file="${CASKS[$i]%%:*}"
  sha="${downloaded_sha256s[$i]}"

  sed_inplace "s/^  version \".*\"/  version \"${VERSION_NUM}\"/" "$cask_file"
  sed_inplace "s/^  sha256 \".*\"/  sha256 \"${sha}\"/" "$cask_file"

  echo "Updated ${cask_file} -> version ${VERSION_NUM}, sha256 ${sha}"
done
