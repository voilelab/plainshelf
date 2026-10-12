#!/usr/bin/env bash
# Assembles a static site: the frontend plus the wasm server under wasm/.
# Usage: cmd/plainshelf-wasm/build.sh <site-dir> [base]
#   base "/" (default) puts the demo at the site root, from a prebuilt frontend/dist.
#   Any other base, such as "/demo/", builds the frontend for that path into
#   <site-dir><base> and adds a deep-link redirect to <site-dir>/404.html (created
#   if absent), the one page GitHub Pages serves for an unknown path.
set -euo pipefail
repo=$(cd "$(dirname "$0")/../.." && pwd)
site=${1:?usage: build.sh <site-dir> [base]}
base=${2:-/}
[[ $base == /*/ || $base == / ]] || { echo "base must start and end with /: $base" >&2; exit 2; }
mkdir -p "$site" && site=$(cd "$site" && pwd)
out="$site${base%/}"

if [[ $base == / ]]; then
  test -f "$repo/frontend/dist/index.html" || { echo "build the frontend first" >&2; exit 1; }
  rm -rf "$out" && mkdir -p "$out"
  cp -R "$repo/frontend/dist/." "$out/"
else
  rm -rf "$out"
  (cd "$repo/frontend" && npx --no-install vite build --base "$base" --outDir "$out" --emptyOutDir)
fi
mkdir -p "$out/wasm"
(cd "$repo" && GOOS=js GOARCH=wasm go build -trimpath -ldflags='-s -w' -o "$out/wasm/plainshelf.wasm" ./cmd/plainshelf-wasm)
# The relay sits at the base so its scope covers the app's pages.
cp "$repo/cmd/plainshelf-wasm/web/sw.js" "$out/plainshelf-sw.js"
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$repo"/cmd/plainshelf-wasm/web/{memfs.js,opfs.js,opfs-writer.js,boot.js,toolbar.js,index.html} "$out/wasm/"

inject() { # <html file> <tag> <markup>: insert after <head>, or else before <tag>
  python3 - "$@" <<'PY'
import sys
p, tag, markup = sys.argv[1:]
s = open(p, encoding='utf-8').read()
if tag not in s:
    sys.exit(f'{p}: no {tag}')
new = tag + '\n    ' + markup if tag == '<head>' else markup + '\n' + tag
open(p, 'w', encoding='utf-8').write(s.replace(tag, new, 1))
PY
}
# boot.js must patch fetch before the app's module script runs.
tags=""
for js in memfs opfs wasm_exec boot toolbar; do tags+="<script src=\"${base}wasm/$js.js\"></script>"; done
inject "$out/index.html" "<head>" "$tags"

if [[ $base != / ]]; then
  test -f "$site/404.html" || printf '<!doctype html>\n<html><head><title>Not found</title></head><body>Not found</body></html>\n' >"$site/404.html"
  grep -q demo-route= "$site/404.html" || inject "$site/404.html" "</head>" "<script>$(sed "s|__BASE__|$base|" "$repo/cmd/plainshelf-wasm/web/deep-link.js")</script>"
fi
echo "assembled $out"
