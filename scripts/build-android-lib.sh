#!/usr/bin/env bash
set -euo pipefail
repo=$(cd "$(dirname "$0")/.." && pwd)
out=${1:?usage: build-android-lib.sh <out-dir>}
mobile_version=v0.0.0-20261011034528-197770444867
min_sdk=$(sed -n 's/.*minSdkVersion = \([0-9]*\).*/\1/p' "$repo/frontend/android/variables.gradle")

mkdir -p "$out" && out=$(cd "$out" && pwd)
cd "$repo"
go install "golang.org/x/mobile/cmd/gomobile@$mobile_version"
go install "golang.org/x/mobile/cmd/gobind@$mobile_version"
# gomobile 197770444867 requires golang.org/x/mobile/bind in the current module graph (cmd/gomobile/bind.go).
module_backup_dir=$(mktemp -d)
cp go.mod go.sum "$module_backup_dir/"
trap 'cp "$module_backup_dir/go.mod" "$module_backup_dir/go.sum" "$repo/"; rm -rf "$module_backup_dir"' EXIT
go get "golang.org/x/mobile/bind@$mobile_version"

PATH="$(go env GOPATH)/bin:$PATH" gomobile bind \
  -target=android/arm64 -androidapi "$min_sdk" \
  -javapkg lab.voile.plainshelf.gobind \
  -trimpath -ldflags='-s -w' \
  -o "$out/plainshelf.aar" ./mobile

unpacked="$out/aar"
rm -rf "$unpacked" && mkdir -p "$unpacked"
(cd "$unpacked" && unzip -q "$out/plainshelf.aar")

readelf=$(command -v readelf || ls "${ANDROID_NDK_HOME:-/nonexistent}"/toolchains/llvm/prebuilt/*/bin/llvm-readelf 2>/dev/null | head -1 || true)
[[ -n $readelf ]] || { echo "no readelf: install binutils or set ANDROID_NDK_HOME" >&2; exit 2; }
# BSD stat uses -f for formatting; GNU stat uses -c (stat(1)).
bytes() { wc -c <"$1" | tr -d ' '; }

report="$out/report.md"
status=0
{
  echo "### Android Go library"
  echo
  echo "| File | Bytes | gzip -9 | Min LOAD align | 16 KB |"
  echo "|---|---:|---:|---:|---|"
  echo "| plainshelf.aar | $(bytes "$out/plainshelf.aar") | | | |"
  while IFS= read -r so; do
    align=$("$readelf" -lW "$so" | awk '$1 == "LOAD" { print $NF }' | sort -u | while read -r a; do echo $((a)); done | sort -n | head -1)
    ok=yes
    if (( align < 0x4000 )); then ok=no; status=1; fi
    printf '| %s | %s | %s | 0x%x | %s |\n' "${so#"$unpacked"/}" "$(bytes "$so")" "$(gzip -9 -c "$so" | wc -c | tr -d ' ')" "$align" "$ok"
  done < <(find "$unpacked" -name '*.so' | sort)
} >"$report"
cat "$report"
if [[ -n ${GITHUB_STEP_SUMMARY:-} ]]; then cat "$report" >>"$GITHUB_STEP_SUMMARY"; fi
exit "$status"
