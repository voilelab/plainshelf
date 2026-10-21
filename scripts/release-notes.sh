#!/bin/sh


set -eu

[ "$#" -eq 2 ] || { echo "Usage: $0 <tag> <target commit or branch>" >&2; exit 2; }
tag=$1
target=$2

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

set -- -f tag_name="$tag" -f target_commitish="$target"
previous=$("$script_dir/resolve-version.sh" previous-tag "$tag")
[ -z "$previous" ] || set -- "$@" -f previous_tag_name="$previous"

gh api "repos/${GITHUB_REPOSITORY}/releases/generate-notes" "$@" --jq .body
