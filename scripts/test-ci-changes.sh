#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
detector="$script_dir/ci-changes.sh"

expect() {
  files=$1
  expected=$2
  actual=$(printf '%s' "$files" | "$detector" | grep '=true$' | cut -d= -f1 | tr '\n' ' ' | sed 's/ $//')
  if [ "$actual" != "$expected" ]; then
    echo "changed [$files]: got '$actual', want '$expected'" >&2
    exit 1
  fi
}

everything='frontend go desktop reader e2e android docs'

expect '' ''
expect 'docs/index.md' 'docs'
expect 'mkdocs.yml
requirements.txt' 'docs'
expect 'README.md
AGENTS.md
.claude/rules/20-judgment.md' ''
expect 'frontend/src/App.vue' 'frontend'
expect 'frontend/web.go' 'go'
expect 'frontend/android/app/build.gradle' 'android'
expect 'e2e/tests/library.spec.ts' 'e2e'
expect 'server/api.go
shelf/testdata/conformance/a/book.json' 'go'
expect 'cmd/plainshelf-wasm/web/boot.js' 'go'
expect 'go.mod' 'go'
expect 'desktop/main.go' 'desktop'
expect 'reader/go.sum' 'reader'
expect 'mobile/mobile.go' 'go android'
expect 'scripts/build-android-lib.sh' 'android'
expect 'docs/index.md
frontend/src/App.vue' 'frontend docs'
expect '.github/workflows/ci.yml' "$everything"
expect 'Dockerfile
docs/index.md' "$everything"

actual=$("$detector" --all </dev/null | grep -c '=true$')
if [ "$actual" != 7 ]; then
  echo "--all: got $actual areas, want 7" >&2
  exit 1
fi
