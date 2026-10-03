# plainshelf-wasm (experiment)

The unmodified server compiled to `GOOS=js GOARCH=wasm` and run inside the
browser, so the real frontend can be served as a static site with no backend.
This is a spike, not a supported build. The shelf persists in the browser's
Origin Private File System (OPFS), per origin; clearing site data resets it.

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

`smoke.mjs` opens the app on a fresh browser profile, seeds one book with a
cover, and checks that the cover renders on that first visit. A reload must
restore the book from OPFS: the same book ID, no second seed, the cover still
rendered, one book cache. It then moves and trashes books through the API page and
checks that both survive another reload with no stale directories left. Last,
two tabs write at once: only one runs the server, both see both writes, the
other takes over when it closes, a forwarding tab still shows covers, and
nothing is lost across the handover or a reload.

## How it fits together

| Piece | Role |
|---|---|
| `main.go` | Builds `server.App` (security `none`, lock mode `none`) and exposes `App.Handler()` as `plainshelfFetch(method, url, headers, body)` |
| `web/memfs.js` | In-memory stand-in for the Node `fs` API that Go's `syscall/fs_js.go` calls |
| `web/opfs.js` | Restores the memfs tree from OPFS before Go starts, then writes changed paths back within 200 ms of a change, and tries once more on `pagehide` |
| `web/sw.js` | Service worker, served as `/plainshelf-sw.js`. It relays `/api` requests that bypass `fetch` (`<img src>` covers, asset links) to the page's server over a `MessageChannel` |
| `web/serve.mjs` | Local static server with the SPA fallback, used by `run-wasm-demo` and `smoke.mjs` |
| `web/boot.js` | Elects one serving tab per origin with a Web Lock; that tab starts the wasm, the others forward to it over a `BroadcastChannel`. Answers same-origin `/api/*` and `/health` fetches and relayed requests; `window.plainshelfRequest(r)` does the same without `fetch`, and an optional `window.plainshelfSeed(serve)` runs before the serving tab answers |
| `server/store/options_js.go` | Badger in memory: its files are mmapped, which js/wasm cannot do |
| `frontend/web_js.go` | Empty `WebFS`: the static host serves the frontend, so it is not embedded twice |

## Findings

- The wasm is 26.7 MB, 6.0 MB gzipped, and is ready in under a second in
  headless chromium.
- Covers load through `<img src>`, which the `fetch` patch never sees, so
  `sw.js` relays them to the page that runs the server; the worker cannot run
  the wasm itself, since it would hold a second, separate shelf. On a first
  visit the app's requests wait up to 3 s for the worker to take control, so
  the first covers are relayed too. A service worker needs a secure context
  (HTTPS or localhost) and must be served from the site root. An `/api` URL
  opened directly in a new tab has no server page behind it and is not
  answered.
- Persistence keeps `memfs.js` synchronous: the whole shelf is held in memory
  and OPFS is a write-behind copy, so Go never waits on async storage. The cost
  is memory proportional to the shelf, and edits from the last 200 ms can be
  lost when the tab closes, since `pagehide` cannot wait for an async write.
- OPFS names are stored percent-encoded. Chromium 141 resolves a non-ASCII
  directory name to its parent, which put every CJK-titled book's files
  directly under `books/`.
- The badger store is still in memory, so settings saved through the API reset
  on reload. The book cache writer ID, which lives there on other builds, is
  kept in `book-cache-writer-id` instead; otherwise every load would leave
  another `book-cache-*.json` behind.
- Tabs share one shelf: only the tab holding the `plainshelf-demo-server`
  lock loads OPFS and runs the server, and other tabs forward every request to
  it, so there is one writer and every tab sees the same data. When that tab
  closes, the next waiting tab takes the lock and restores from OPFS, so a
  handover loses whatever the closed tab had not yet written (its last 200 ms).
  Forwarded requests are resent until a serving tab acknowledges them; one the
  old tab acknowledged but never answered is sent again to the new one, so a
  write can apply twice if the old tab died between applying and answering.
- Persistence needs `FileSystemFileHandle.createWritable`; where it is missing
  the demo falls back to memory and logs a warning. Only chromium was tested.
- Only the shelf list, book creation, move, trash and the home page were
  exercised. Import, the reader and the source editor are untested here.
