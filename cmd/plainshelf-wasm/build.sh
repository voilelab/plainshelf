#!/usr/bin/env bash
# Assembles a static site: the built frontend plus the wasm server under wasm/.
# Usage: cmd/plainshelf-wasm/build.sh <out-dir>   (run after the frontend build)
set -euo pipefail
repo=$(cd "$(dirname "$0")/../.." && pwd)
out=${1:?usage: build.sh <out-dir>}
test -f "$repo/frontend/dist/index.html" || { echo "build the frontend first" >&2; exit 1; }

rm -rf "$out" && mkdir -p "$out"
cp -R "$repo/frontend/dist/." "$out/"
mkdir -p "$out/wasm"
(cd "$repo" && GOOS=js GOARCH=wasm go build -trimpath -ldflags='-s -w' -o "$out/wasm/plainshelf.wasm" ./cmd/plainshelf-wasm)
# The relay must sit at the root so its scope covers the app's /api requests.
cp "$repo/cmd/plainshelf-wasm/web/sw.js" "$out/plainshelf-sw.js"
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$repo"/cmd/plainshelf-wasm/web/{memfs.js,opfs.js,opfs-writer.js,boot.js,index.html} "$out/wasm/"

# boot.js must patch fetch before the app's module script runs.
tags='<script src="/wasm/memfs.js"></script><script src="/wasm/opfs.js"></script><script src="/wasm/wasm_exec.js"></script><script src="/wasm/boot.js"></script>'
python3 - "$out/index.html" "$tags" <<'PY'
import sys
p, tags = sys.argv[1], sys.argv[2]
s = open(p, encoding='utf-8').read()
open(p, 'w', encoding='utf-8').write(s.replace('<head>', '<head>\n    ' + tags, 1))
PY
echo "assembled $out"
