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
string. Four platform backends implement it: browser localStorage, Capacitor
Filesystem in app-private storage on Android, a JSON file beside shelves.json on
the Wails desktop, and an in-memory store for mock mode and unit
tests.](../assets/plainshelf-device-storage.svg)

The pivot is deliberately narrow. `DeviceDocumentStorage` only knows how to load
and save **one string**; parsing, merging and trimming live in each feature's own
document module, so every platform shares one implementation and a new backend
only has to persist text.

### Which backend a document actually gets

The backend is resolved per document, not per platform, so the table is not
uniform:

| Document | Browser | Android shell | Wails desktop | Standalone reader |
|---|---|---|---|---|
| `readingProgress` | localStorage | localStorage | JSON file | JSON file (shared with desktop) |
| `readHistory` | localStorage | Capacitor Filesystem | JSON file | localStorage |
| `readingStats` | localStorage | Capacitor Filesystem | JSON file | localStorage |

Mock API mode and unit tests use the in-memory backend for all three.

The standalone reader binds only the reading-progress methods, so its read
history and reading stats fall back to WebView localStorage; its reading
progress goes to the same JSON file the desktop app uses, which is what lets it
reach the desktop library (below).

`readingProgress` has no Android-specific path today: it stays on the WebView's
localStorage there, while read history and reading stats do not. That matters,
because the Capacitor backend was chosen precisely to escape WebView storage —
`Directory.Data` is app-private internal storage, needs no runtime permission,
and, unlike localStorage or IndexedDB in an Android WebView, is not subject to
silent storage eviction.

The Capacitor backend lives in `frontend/src/shells/mobile/`, not in
`frontend/src/storage/`, and the seam is deliberate: it is what stops
`@capacitor/filesystem` from being pulled into the web and desktop bundles by
the read-history and reading-stats stores.

### Reader progress reaches the desktop library

The standalone reader is a separate process with its own window. It has no real
shelf, so it keys every book's progress under one synthetic shelf id, `book`.
To make that progress show up in the desktop library, the reader writes into the
**same** `reading_progress.json` the desktop app keeps, rather than its own
WebView storage.

On the desktop app (macOS), reading a book launches this standalone reader in
its own window rather than the in-app reader: the read action shells out to the
installed `PlainShelfReader` with the book's `.bookpkg` path. Only the desktop
does this — web and mobile keep the in-app reader, which is the same shared code
and stays the desktop's fallback if the reader will not launch. A specific
chapter jump also stays in the in-app reader, which can target a section the
standalone reader cannot yet.

One file now has two writers, so:

- The desktop app **projects** the reader's `book` entries onto the real shelves
  that hold those books. Each entry is matched by its stable book ID — the ID
  survives moves and renames, and identifies exactly one book in the library —
  and copied to that book's real shelf, keeping whichever offset is larger. This
  is one-way (reader → desktop) and monotonic: a book's saved position never
  moves backwards, which is the safe rule to pick when the document carries no
  timestamps to arbitrate by. The projection runs whenever the desktop reads
  progress, so a book read in the reader shows its new position the next time the
  library or that book is opened.
- An entry the desktop cannot place — a book that has been removed, or a loose
  book opened in the reader from outside every shelf — is left under `book`
  untouched, never guessed onto the wrong shelf, and folded in later if that book
  is imported.
- Each process only rewrites its own part of the file (the reader its `book`
  entries, the desktop the real-shelf entries), and the two coordinate through a
  lock on the file, so neither loses the other's writes.

This is not two-way sync: progress recorded in the desktop app does not appear
in the standalone reader, which still reads only its own `book` entries. Nothing
here is written into the shelf; reading progress stays device-local convenience
state.

Two more details the diagram cannot show:

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
