#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
resolver="$script_dir/resolve-version.sh"

expect() {
  mode=$1
  input=$2
  expected=$3
  actual=$($resolver "$mode" "$input")
  if [ "$actual" != "$expected" ]; then
    echo "$mode $input: got '$actual', want '$expected'" >&2
    exit 1
  fi
}

expect display v0.8.0 v0.8.0
expect native v0.8.0 0.8.0
expect native v0.8.0-beta.1 0.8.0
expect native v0.7.0-138-gabcdef 0.7.0
expect native dev 0.0.0
expect native abcdef 0.0.0
expect native v01.2.3 0.0.0
expect android-name v0.8.0-beta.1 0.8.0-beta.1
expect android-name v0.7.0-138-gabcdef 0.7.0-138-gabcdef
expect android-name dev 0.0.0-dev

for valid in v0.8.0 v0.8.0-beta.1 v1.2.3-rc.2; do
  "$resolver" validate-tag "$valid"
done

for invalid in dev v1.2 v01.2.3 v1.2.3+build v1.2.3- v1.2.3-01 v1.2.3-alpha..1; do
  if "$resolver" validate-tag "$invalid" >/dev/null 2>&1; then
    echo "validate-tag accepted invalid tag '$invalid'" >&2
    exit 1
  fi
done

temp_repo=$(mktemp -d "${TMPDIR:-/tmp}/plainshelf-version-test.XXXXXX")
trap 'rm -rf "$temp_repo"' EXIT
git -C "$temp_repo" init -q
git -C "$temp_repo" config user.name version-test
git -C "$temp_repo" config user.email version-test@example.invalid
git -C "$temp_repo" commit --allow-empty -qm initial
git -C "$temp_repo" tag v0.7.0
git -C "$temp_repo" tag v0.8.0-beta.1
git -C "$temp_repo" tag v0.8.0

actual=$(cd "$temp_repo" && "$resolver" display)
if [ "$actual" != v0.8.0 ]; then
  echo "default display: got '$actual', want 'v0.8.0'" >&2
  exit 1
fi

git -C "$temp_repo" commit --allow-empty -qm development
short_commit=$(git -C "$temp_repo" rev-parse --short=7 HEAD)
actual=$(cd "$temp_repo" && "$resolver" display)
expected="v0.8.0-1-g${short_commit}"
if [ "$actual" != "$expected" ]; then
  echo "development display: got '$actual', want '$expected'" >&2
  exit 1
fi
