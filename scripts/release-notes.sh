#!/bin/sh

# Prints a release's notes: the tag's CHANGELOG.md section, then GitHub's list
# of pull requests merged since the previous release tag. Needs gh, GH_TOKEN
# and GITHUB_REPOSITORY; the tag itself need not exist yet.

set -eu

[ "$#" -eq 2 ] || { echo "Usage: $0 <tag> <target commit or branch>" >&2; exit 2; }
tag=$1
target=$2

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

"$script_dir/release-changelog.sh" notes "$tag"

set -- -f tag_name="$tag" -f target_commitish="$target"
previous=$("$script_dir/resolve-version.sh" previous-tag "$tag")
# Named explicitly: GitHub's own guess does not follow SemVer precedence.
[ -z "$previous" ] || set -- "$@" -f previous_tag_name="$previous"

printf '\n'
gh api "repos/${GITHUB_REPOSITORY}/releases/generate-notes" "$@" --jq .body
