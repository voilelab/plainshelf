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

expect release-branch v0.11.0 release/0.x
expect release-branch v1.0.0-rc1 release/1.x
expect release-branch v12.3.4 release/12.x
if "$resolver" release-branch dev >/dev/null 2>&1; then
  echo "release-branch accepted invalid version 'dev'" >&2
  exit 1
fi

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

actual=$(cd "$temp_repo" && "$resolver" latest-tag)
if [ "$actual" != v0.8.0 ]; then
  echo "latest-tag: got '$actual', want 'v0.8.0'" >&2
  exit 1
fi

for next in v0.8.1 v0.9.0 v1.0.0-rc1 v1.0.0; do
  if ! (cd "$temp_repo" && "$resolver" validate-next "$next" >/dev/null 2>&1); then
    echo "validate-next rejected '$next' after v0.8.0" >&2
    exit 1
  fi
done
for stale in v0.8.0 v0.7.0 v0.8.0-beta.2 v0.7.9 dev; do
  if (cd "$temp_repo" && "$resolver" validate-next "$stale" >/dev/null 2>&1); then
    echo "validate-next accepted '$stale' after v0.8.0" >&2
    exit 1
  fi
done

# SemVer 11.4: numeric < alphanumeric, shorter < longer, numbers compare numerically.
git -C "$temp_repo" tag v1.0.0-rc.2
for next in v1.0.0-rc.10 v1.0.0-rc.2.1 v1.0.0-rcx v1.0.0; do
  if ! (cd "$temp_repo" && "$resolver" validate-next "$next" >/dev/null 2>&1); then
    echo "validate-next rejected '$next' after v1.0.0-rc.2" >&2
    exit 1
  fi
done
for stale in v1.0.0-rc.1 v1.0.0-rc v1.0.0-beta.9 v1.0.0-1 v0.9.9; do
  if (cd "$temp_repo" && "$resolver" validate-next "$stale" >/dev/null 2>&1); then
    echo "validate-next accepted '$stale' after v1.0.0-rc.2" >&2
    exit 1
  fi
done

# Git's version sort puts v1.0.0-2 above v1.0.0-1a; SemVer does not.
git -C "$temp_repo" tag -d v1.0.0-rc.2 >/dev/null
git -C "$temp_repo" tag v1.0.0-2
git -C "$temp_repo" tag v1.0.0-1a
actual=$(cd "$temp_repo" && "$resolver" latest-tag)
if [ "$actual" != v1.0.0-1a ]; then
  echo "latest-tag: got '$actual', want 'v1.0.0-1a'" >&2
  exit 1
fi
if (cd "$temp_repo" && "$resolver" validate-next v1.0.0-3 >/dev/null 2>&1); then
  echo "validate-next accepted 'v1.0.0-3' after v1.0.0-1a" >&2
  exit 1
fi
