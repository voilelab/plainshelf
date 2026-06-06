#!/usr/bin/env bash
set -euo pipefail

if ! command -v gomobile >/dev/null 2>&1; then
  echo "gomobile is required to build android/app/libs/shelfmobile.aar." >&2
  echo "Install it with: go install golang.org/x/mobile/cmd/gomobile@latest && gomobile init" >&2
  exit 1
fi

mkdir -p android/app/libs
gomobile bind -target=android -androidapi 21 -o android/app/libs/shelfmobile.aar ./shelfmobile
