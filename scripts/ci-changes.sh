#!/bin/sh

set -eu

frontend=false
go=false
desktop=false
reader=false
e2e=false
android=false
docs=false
all=false

if [ "${1-}" = --all ]; then
  all=true
else
  while IFS= read -r path || [ -n "$path" ]; do
    case $path in
      '') ;;
      docs/* | mkdocs.yml | requirements.txt) docs=true ;;
      frontend/android/*) android=true ;;
      frontend/*.go) go=true ;;
      frontend/*) frontend=true ;;
      e2e/*) e2e=true ;;
      desktop/*) desktop=true ;;
      reader/*) reader=true ;;
      mobile/*) go=true android=true ;;
      scripts/build-android-lib.sh | scripts/resolve-android-jdk.sh) android=true ;;
      *.go | go.mod | go.sum | .golangci.yml | cmd/* | internal/* | server/* | shelf/*) go=true ;;
      *.md | .claude/* | LICENSE | image.png) ;;
      *) all=true ;;
    esac
  done
fi

if [ "$all" = true ]; then
  frontend=true go=true desktop=true reader=true e2e=true android=true docs=true
fi

printf '%s\n' "frontend=$frontend" "go=$go" "desktop=$desktop" "reader=$reader" \
  "e2e=$e2e" "android=$android" "docs=$docs"
