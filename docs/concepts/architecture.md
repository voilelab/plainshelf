# Architecture

PlainShelf is one Go binary with an embedded Vue frontend. The same frontend is
served three ways — over HTTP to a browser, inside a Wails desktop window, and
inside a Capacitor WebView on Android — and every one of them is a client of the
same API surface.

---

## System shape

![Architecture diagram: web, desktop and Android clients reach a local Go server
behind a local-token boundary, where a shelf manager keeps an in-memory book
cache over the shelf filesystem that holds the library. A separate key-value
store holds application settings, and the Android client can also read a
pCloud-hosted shelf directly as a read-only
backend.](../assets/plainshelf-architecture.svg)

Three things in that picture are load-bearing.

**The shelf filesystem is the source of truth.** `ShelfManager` holds an
in-memory book cache, but the cache is only an accelerator — it is rebuilt from
`books/` and never becomes the authority. See
[Data Model](data-model.md) for the on-disk layout and
[Shelf Cache and Disk I/O](shelf-cache-and-io.md) for how the cache stays fresh.

**The desktop client does not speak HTTP.** Wails serves the embedded frontend
and routes any request whose path starts with `/api/` straight into the server's
handler in the same process. There is no listening socket, which is why the
desktop app needs no port and no token.

**The Android client has two independent paths.** It can talk to a PlainShelf
server over HTTP like any other client, or it can read a shelf held on pCloud
directly, bypassing the server entirely. The second path is a read-only storage
backend, not synchronisation: nothing is ever written back to pCloud.

### The security boundary

Mutating requests cross the `local_token` boundary. The token is generated at
startup and carried in the `X-PlainShelf-Token` header; origin checking is
applied alongside it. `protect_read` extends the same requirement to reads. The
`password` and `external` modes are reserved names and are not implemented.

---

## Reading state is not part of the shelf

Reading progress, read history and reading statistics are **device-local**. They
are not stored in the shelf, so they are not carried by a backup of `books/`,
and two devices pointed at the same shelf keep their own.

![Architecture diagram: three feature documents — reading progress, read history
and reading stats — all mutate through one queued device document store, which
narrows to a single format-blind interface that only loads and saves a plain
string. Three platform backends implement it: browser localStorage, a JSON file
beside shelves.json on the Wails desktop, and an in-memory store for mock mode
and unit tests.](../assets/plainshelf-device-storage.svg)

The pivot is deliberately narrow. `DeviceDocumentStorage` only knows how to load
and save **one string**; parsing, merging and trimming live in each feature's own
document module, so every platform shares one implementation and a new backend
only has to persist text.

Two details the diagram cannot show:

- **Mutations are queued.** Every change is a read-modify-write chained behind
  the previous one. Without that, opening the reader while a reading-time tick
  fires would let one write silently overwrite the other. A failed mutation does
  not wedge the chain.
- **The storage key is scoped to the server, not just the shelf.** A client that
  can be repointed at another server (the Android shell) keys its document on
  `{apiBase}|{shelfID}`, because two servers commonly both call a shelf
  `default_shelf` while their book IDs mean entirely different things.
  Same-origin clients (web, desktop) keep the bare shelf ID.

---

## Where the code lives

| Path | Responsibility |
|---|---|
| `shelf/` | Filesystem-backed library core: books, layers, trash, locking, cache |
| `server/` | HTTP API, routing, security, settings store, worker pool |
| `frontend/` | Vue UI, device-local storage, Capacitor Android project |
| `desktop/` | Wails desktop client |
| `internal/epub/` | EPUB parsing and rendering, used only at [import](../epub-import.md) time |
