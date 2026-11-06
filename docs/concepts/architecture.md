# Architecture

PlainShelf uses a Go server and a shared Vue frontend. The server binary embeds
the frontend for browsers; the Wails desktop app and Capacitor Android app each
package it with an embedded Go server. Every client uses the same API surface.

---

## System shape

![Architecture diagram: browsers use HTTP and desktop and Android clients can
reach an embedded Go server in-process. The HTTP API has a local-token boundary.
A shelf manager keeps an in-memory book cache over the shelf filesystem that
holds the library. A separate key-value
store holds application settings. The Go engine also opens read-only virtual
shelves over WebDAV or, on Android, pCloud.](../assets/plainshelf-architecture.svg)

Three things in that picture are load-bearing.

**The shelf filesystem is the source of truth.** `ShelfManager` holds an
in-memory book cache, but the cache is only an accelerator — it is rebuilt from
`books/` and never becomes the authority. See
[Data model](data-model.md) for the on-disk layout and
[Shelf cache and disk I/O](shelf-cache-and-io.md) for how the cache stays fresh.

**The desktop client does not speak HTTP.** Wails serves the embedded frontend
and routes any request whose path starts with `/api/` straight into the server's
handler in the same process. By default there is no listening socket, so the
desktop UI needs no port and no token. **Share on local network** opens a
separate listener for the selected shelves' read-only WebDAV export, protected
by its own token; it exposes neither the API nor the web UI. See
[the read-only WebDAV export](../deployment-and-threat-model.md#the-read-only-webdav-export).

**Wails stays at the edge of the desktop and reader apps.** `DesktopApp` and
`ReaderApp` reach dialogs, the browser, script execution, reload and quit
through a small `nativeUI` interface, and describe their menus as plain data.
Only each module's `main.go` and `wails_native.go` import Wails; together they
hold the Wails context, translate the menu, and decide lifecycle timing such
as deferring dialogs until the page is up (`WindowRuntimeReady`). Tests drive
the apps through a fake `nativeUI`. Both apps run on Wails v3.

**The Android app embeds the Go server.** Local shelves, pCloud shelves and
WebDAV sources (including another PlainShelf's LAN share) are all read through
that server, reached in-process through the native bridge and WebView request
interception, without a UI listener. The Go engine opens each source; virtual
shelves are read-only storage backends, and nothing is written back to them.
The app also retains a mode that connects to an external PlainShelf server's
full API over HTTP. Like the desktop app, Android can explicitly open a
separate read-only LAN share for selected local shelves; see
[Android development](../development/android.md).

### The security boundary

Mutating requests cross the `local_token` boundary. The token is generated at
startup and carried in the `X-PlainShelf-Token` header; origin checking is
applied alongside it. `protect_read` extends the same requirement to reads. The
`password` and `external` modes are reserved names and are not implemented.

A shelf rescan (`POST /api/shelves/{id}/scans`) is the one `POST` the token
treats as a read: it walks the shelf and rebuilds the cache without writing, so
`protect_read` governs it, and read-only mode accepts it for the same reason.
The origin check still applies, so it is exempt from the token but not from
CSRF.

Downloading a book from a virtual shelf
(`PUT`/`DELETE /api/shelves/{id}/books/{book_id}/download`) is the other way
round: read-only mode accepts it, because it writes only this installation's
[offline copies](shelf-sources.md#offline-copies) and never the shelf, but it
still needs the token, because it writes to this machine's disk.

The log API (`/api/logs`) is the one read that always needs the token, whatever
`protect_read` says. Logs are server internals rather than shelf content: they
record every request path, so reading them yields the shelf's structure along
with access times and remote addresses. Turning `protect_read` off to let the
household read books over the LAN is not a decision to publish that. The origin
check rides along with the token as it does for a write, so a browser reading
the logs must also come from an allowed origin. Security mode `none` still opens
everything, because it is an explicit choice not to authenticate.

---

## Reading state is not part of the shelf

Reading progress, read history and reading statistics are **device-local**. They
are not stored in the shelf, so they are not carried by a backup of `books/`,
and two devices pointed at the same shelf keep their own.

![Architecture diagram: three feature documents (reading progress, read history
and reading stats) all mutate through one queued device document store, which
narrows to a single format-blind interface that only loads and saves a plain
string. Four platform backends implement it: browser localStorage, Capacitor
Filesystem in app-private storage on Android, a JSON file beside shelves.json on
the Wails desktop, and an in-memory store for mock mode and unit
tests.](../assets/plainshelf-device-storage.svg)

The pivot is deliberately narrow. `DeviceDocumentStorage` only knows how to load
and save **one string**; parsing, merging and trimming live in each feature's own
document module, so every platform shares one implementation and a new backend
only has to persist text.

The Wails desktop and the standalone reader are the exception for read history
and reading stats. Another
process can write the same files, and neither document can be merged after the
fact: read history is an ordered list without timestamps, and reading stats are
summed seconds, so taking the newer copy drops an update and taking the union
counts one twice. Both therefore send each change as one operation
(`RecordReadHistory`, `ClearReadHistory`, `SetReadHistoryLimit`,
`AddReadingSeconds`), and Go applies it inside the cross-process file lock
(`internal/readhistory`, `internal/readingstats`, both on
`internal/lockedfile`), including the history limit and the 400-day stats
retention. Reading progress keeps whole-document writes, which Go merges
newest-wins under the same lock.

### Which backend a document actually gets

The backend is resolved per document, not per platform, so the table is not
uniform:

| Document | Browser | Android shell | Wails desktop | Standalone reader |
|---|---|---|---|---|
| `readingProgress` | localStorage | localStorage | JSON file | JSON file (shared with desktop) |
| `readHistory` | localStorage | Capacitor Filesystem | JSON file | JSON file (shared with desktop) |
| `readingStats` | localStorage | Capacitor Filesystem | JSON file | JSON file (shared with desktop) |

Mock API mode and unit tests use the in-memory backend for all three.

Inside a Wails window, a document whose binding is missing is an error, not a
reason to fall back to WebView localStorage: every read and write rejects. Only
the `?desktop-shell-preview=1` browser preview keeps browser storage.

The standalone reader binds the same methods for all three documents and writes
the same JSON files as the desktop app, through the same operations and lock.
That is what lets a book read in the reader reach the desktop library's progress,
"recent reading" and reading time (below).

The frontend autosaves reading progress every 10 seconds. To narrow what closing
a Wails window between two saves can lose, the frontend also stages its latest
position in Go memory, and the app's single close function waits for it to be
written (`internal/readingclose`); a flush request over an async event proved
unreliable during window teardown. The main window's `WindowClosing` hook and
`OnShutdown` both call that function, which runs once whichever fires first: it
writes every staged position, then stops LAN sharing (desktop), then closes the
server or the open book package. On the desktop, closing the main window then
quits the app, so no reader window outlives it. The wait is bounded (`readingclose.DefaultTimeout`, 1.5
seconds) and best-effort: if the cross-process file lock is held longer, or the
write fails, the window closes anyway and that position can still be lost. The
staged write merges newest-wins, so it never overwrites a newer entry from the
other process.

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

### Desktop reader windows

The desktop's default way to open a book (reading setting "Open in a new
window", `reader-launch-mode` `new-window`) is another window of the same
process. `OpenReaderWindow` names the window after the shelf and book, so
reading a book that already has a window brings that window forward instead of
opening a second one. It first writes that book's staged position, then points
the window back at the reader route, because the window may have navigated to
the book's detail page or the request may name a chapter. The window loads the reader route with `?shelf=` and the
optional `?section=`; the frontend takes its active shelf from that parameter
for that window only and leaves the saved shelf alone.

Every window shares the one server, preference store and document files, so the
merging above covers them without anything window-specific. Closing a reader
window writes only that book's staged position and leaves the app running.
After each reading-data write, Go emits the `plainshelf:reading-data-changed`
event to every window, and the dashboard refetches on it, so "recent reading"
in the main window updates while a reader window is still open beside it.

The mode is offered only inside the desktop app. The standalone reader remains
the "Open a new reader" choice and the way to open a `.bookpkg` without the
desktop app installed.

### Reader progress reaches the desktop library

The standalone reader is a separate process with its own window. How it keys a
book's progress depends on how it was launched:

- **Launched from the desktop app** (macOS, under the "Open a new reader"
  setting), reading a book shells out to the
  installed `PlainShelfReader` with the book's `.bookpkg` path (`-book`) and its
  real shelf id (`-shelf <realShelfID>`). The reader reports that real shelf as
  its active shelf, so it opens at, and writes back to, the same
  `shelves.<realShelfID>.<bookID>` position the desktop library already holds,
  whichever source last recorded it (the reader, the in-app reader, or an
  import). Both sides read and write the same key, so no projection is involved.
  A chapter "read" action adds `-section <index>` (the reader's own section
  index, matching the `?section=` deep link), so the reader opens on that
  chapter instead of the restored progress; a default read omits it.
- **Run standalone** (opened directly, no `-shelf`), the reader has no real shelf,
  so it keys every book's progress under one synthetic shelf id, `book`, which the
  desktop app projects onto the real shelves later (below).

Either way the reader writes into the **same** `reading_progress.json` the desktop
app keeps, rather than its own WebView storage — that is what lets its progress
reach the desktop library.

Only the desktop shells out to the standalone reader — web and mobile keep the
in-app reader, which is the same shared code and stays the desktop's fallback if
the reader will not launch. A chapter jump shells out the same way, passing its
section so the standalone window opens on that chapter; the in-app reader at that
section remains the fallback when the reader will not launch.

One file has more than one writer, so every entry carries the wall-clock time it
was written and all reconciliation is **newest-wins per book**:

- When the desktop and a desktop-launched reader both write the same real shelf,
  the more recent write wins for each book. Neither process owns the namespace, so
  neither can clobber the other by holding a stale in-memory copy — the loser is
  simply the older timestamp. A **reset** is a timestamped tombstone (offset 0)
  rather than a deletion, so it too competes by recency instead of being
  resurrected by an older advance.
- A **standalone** reader still has no real shelf, so it keys progress under
  `book`, and the desktop app **projects** those entries onto the real shelves
  that hold them — matched by stable book ID (which survives moves and renames and
  identifies exactly one book), taking the newer entry. The projection runs
  whenever the desktop reads progress, so a book read in the reader shows its new
  position the next time the library or that book is opened.
- An entry the desktop cannot place (a book that has been removed, or a loose
  book opened in the reader from outside every shelf) is left under `book`
  untouched, never guessed onto the wrong shelf, and folded in later if that
  book is imported.
- Writers coordinate through a lock on the file, and each read-modify-write
  re-reads under it, so a write only ever replaces entries older than itself.

A book opened **through the desktop app** is therefore fully two-way: the reader
starts at the desktop library's stored position and writes back to the same
real-shelf key, and a later position from either side, forward or a reset, wins
by recency. Only a reader opened **directly** stays one-way, its `book`
namespace folded in by projection; progress recorded in the desktop app does not
appear there because it never carried that book's real shelf id. Nothing here is
written into the shelf; reading progress stays device-local convenience state.

Read history and reading time follow the same keying. A desktop-launched reader
records both under the real shelf id, so the desktop sees them directly; the
reader's own record moves the book to the front of "recent reading" once, and
the desktop no longer records a launch itself. A standalone reader records:

- **Read history** under `book`. Whenever the desktop reads history, it moves
  each `book` entry that resolves to exactly one real shelf to the front of that
  shelf's list, in reader order, and removes it from `book`. History carries no
  timestamps, so a copy left under `book` would be moved to the front again on
  every read; moving it is what keeps the order stable.
- **Reading time** under `book/<bookID>`, because a stats entry is only shelf,
  date and seconds and would otherwise lose which book it was for. Whenever the
  desktop reads stats, it adds each such key's seconds to the book's real shelf,
  capped per day as usual, and removes the key.

Both use the same rule as progress: a book found on no shelf, or on more than one,
stays under the reader key until it can be placed. When a real shelf is itself
called `book`, the desktop projects neither document, as for progress.

The diagram cannot show either of these:

- **Mutations are queued.** Every change is a read-modify-write chained behind
  the previous one. Without that, opening the reader while a reading-time tick
  fires would let one write silently overwrite the other. A failed mutation does
  not wedge the chain.
- **The storage key is scoped to the server, not just the shelf.** A client that
  can be repointed at another server (the Android shell) keys its document on
  `{apiBase}|{shelfID}`, because two servers commonly both call a shelf
  `default_shelf` while their book IDs mean entirely different things.
  Same-origin clients (web, desktop) keep the bare shelf ID.

## UI preferences

Language, zoom, reader font and the other UI preferences go through one
synchronous key/value store, `frontend/src/storage/preferences.ts`, instead of
each module calling `window.localStorage`. Components that show the same
preference subscribe to it, so a change in one updates the others in the same
app.

| Client | Where |
|---|---|
| Browser | `localStorage`, per origin |
| Android shell | WebView `localStorage` |
| Wails desktop and standalone reader | `preferences.json` in the app data directory, beside `shelves.json` |

The desktop and the reader share one `preferences.json`, so a font size chosen in
the reader is what the desktop's in-app reader uses next. Only the selected shelf
(`plainshelf.shelf`) is kept per app, under `apps.desktop` or `apps.reader`;
every other key is under `shared`. Go decides which keys are per app
(`internal/preferences`), and writes one key at a time inside the same
cross-process file lock the reading documents use, so the two apps writing
different keys at once keep both.

`frontend/src/main.ts` loads the whole file through `GetPreferences()` before it
imports the rest of the app, because the locale, the zoom and the selected shelf
are read when their modules load; the first paint already uses the stored
values. After that, reads come from the in-memory copy and each change is sent
as `SetPreference(key, value)`, in call order. An empty value removes the key.
When the window regains focus the copy is reloaded, so a font size changed in
the reader reaches a desktop that stayed open. Components subscribed to a
changed key update then; one that read its value only at load, such as the
locale or the zoom, picks it up on the next start.
Like the reading documents, a Wails window without the binding does not fall
back to WebView storage: preferences then last only until the window closes. The
`?desktop-shell-preview=1` browser preview keeps `localStorage`.

The mobile reader's one-time gesture hint (`reader-mobile-gesture-hint-seen`)
is not a preference and stays in `localStorage`.

---

## Where the code lives

| Path | Responsibility |
|---|---|
| `shelf/` | Filesystem-backed library core: books, folders, trash, locking, cache |
| `server/` | HTTP API, routing, security, settings store, worker pool |
| `frontend/` | Vue UI, device-local storage, Capacitor Android project |
| `desktop/` | Wails desktop client |
| `internal/epub/` | EPUB parsing and rendering, used only at [import](../epub-import.md) time |
