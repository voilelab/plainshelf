# plainshelf-wasm (experiment)

The unmodified server compiled to `GOOS=js GOARCH=wasm` and run inside the
browser, so the real frontend can be served as a static site with no backend.
This is a spike, not a supported build: nothing persists past a page reload.

## Run it

```sh
just run-wasm-demo        # builds the frontend and wasm, serves on http://127.0.0.1:5180/
just build-wasm-demo      # build only, into workspace/wasm-demo
```

Without `just`, run the recipe's commands from the `justfile` directly.
Any static host can serve `workspace/wasm-demo`; it needs no rewrites beyond
an SPA fallback to `index.html`.

To check it in chromium (needs `npm --prefix e2e ci`):

```sh
node cmd/plainshelf-wasm/web/smoke.mjs workspace/wasm-demo /tmp/app.png
```

`smoke.mjs` checks a create/list round trip on `/wasm/index.html`, then opens
the app at `/` with one seeded book and saves a screenshot.

## How it fits together

| Piece | Role |
|---|---|
| `main.go` | Builds `server.App` (security `none`, lock mode `none`) and exposes `App.Handler()` as `plainshelfFetch(method, url, headers, body)` |
| `web/memfs.js` | In-memory stand-in for the Node `fs` API that Go's `syscall/fs_js.go` calls |
| `web/serve.mjs` | Local static server with the SPA fallback, used by `run-wasm-demo` and `smoke.mjs` |
| `web/boot.js` | Starts the wasm and answers same-origin `/api/*` and `/health` fetches through it; an optional `window.plainshelfSeed(serve)` runs first |
| `server/store/options_js.go` | Badger in memory: its files are mmapped, which js/wasm cannot do |
| `frontend/web_js.go` | Empty `WebFS`: the static host serves the frontend, so it is not embedded twice |

## Findings

- The wasm is 26.7 MB, 6.0 MB gzipped, and is ready in under a second in
  headless chromium.
- Cover images do not load: the web build points `<img src>` at `/api/…/cover`,
  which bypasses `fetch`. Serving them needs a Service Worker or the blob cover
  path the mobile provider already uses.
- Persistence would need `memfs.js` backed by OPFS or IndexedDB; it is
  synchronous today, which keeps the shim simple.
- Only the shelf list, book creation and the home page were exercised. Import,
  the reader and the source editor are untested here.
