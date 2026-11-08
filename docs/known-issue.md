# Known issues

## Shelf cache limitations

**If a change you made outside PlainShelf has not appeared, press Update book
list on the library toolbar.** It walks the shelf immediately instead of waiting
for the next interval, and it is the same action in every setup below — local
disk, SMB or NAS, a sync folder, a cloud mount, and the Android client's pCloud
shelf. Every delay below that comes from an interval is bounded by it. See
[Rescanning on demand](concepts/shelf-cache-and-io.md#rescanning-on-demand).

For the operational model, the initial metadata scan, and tuning guidance, see
[Shelf cache and disk I/O](concepts/shelf-cache-and-io.md).

Two limits run through every setup, so they are stated once here rather than
repeated below.

**A change made outside PlainShelf is discovered on a timer.** Within
`scan_interval` a refresh may only reopen the books already in the cache rather
than walking the library, so a book added, moved, or deleted by anything other
than PlainShelf can take that long to appear. **Update book list** is what
shortens the wait to now.

**Between scans, staleness is judged from `book.json`'s size and modification
time.** The per-book refresh that runs on `book_check_interval` stats
`book.json` and reopens a book only when its size or modification time changed,
so an edit that leaves both unchanged is not noticed by that check, and neither
is a change to a cover or a source file, which does not move `book.json` at all.
A full scan is different: it reopens every `book.json` outright rather than
trusting its stat, so **Update book list**, which forces one, does pick these
up. A virtual shelf, such as the Android client's pCloud shelf, has no per-book
check at all; see [below](#a-pcloud-shelf-on-the-android-client).

It takes a full scan to find such an edit, whether the timer starts one or you
force one with the button. Until that scan the stale reading stands — though the
file itself is served correctly when something asks for it, since covers and
source text are read from disk, never cached.

On a single desktop machine those two limits are the whole story. The setups
below each add something to them.

### One server, several personal devices (Tailscale and similar)

This means one shelf server process on one machine, with your own devices
reaching that same server over the network.

There is only one server process, so there is one authoritative in-memory cache
and no divergence between clients to reason about. External changes to the
library folder are still discovered on the timer above, and **Update book list**
pressed on any device refreshes what all of them see.

### A shelf inside a sync folder (Dropbox, Google Drive, Syncthing, iCloud)

A file caught mid-sync (being renamed, copied, or still written) can fail to
reopen and is skipped for that pass, then picked up once the sync settles.

Sync clients also tend to preserve or normalize timestamps, which turns the
stat-based blind spot above from a corner case into something you can
realistically hit. Press **Update book list** once the sync has settled.

### A pCloud shelf on the Android client

The app opens the shelf with the same Go engine as the server, read-only. How
its book list follows changes depends on whether a PlainShelf server or desktop
app exports a book cache into the shelf's `app/` directory:

- **With an export,** the list comes from the newest
  `app/book-cache-*.json`. Once `scan_interval` has passed, the next listing
  checks for a newer export and takes it. A change therefore reaches the phone
  after the writer next exports it, which its `book_cache_interval` paces
  (one hour by default), and then within a minute, the default `scan_interval`,
  which the Android app does not change.
- **Without an export,** the app lists the shelf from its saved offline snapshot
  if it has one, and walks pCloud only when it has none. The timer does not walk
  the shelf again: a book added, removed, or renamed from another device appears
  only after **Update book list** on the library toolbar.
- **Update book list** walks pCloud directly in both cases and reopens every
  `book.json`, so it also picks up edits that an export has not caught up with.
  Walking costs one recursive listing plus requests per book.
- **When pCloud cannot be reached,** the app lists the shelf from the saved
  snapshot. Downloaded books open from the device. A book that was not
  downloaded answers that it is not downloaded instead of opening.
- **A stale list can make a book fail to open.** If a book was replaced or
  moved on pCloud since the list was read, opening it fails until the list is
  updated. Downloaded books are unaffected, because they are read from the
  device.

A WebDAV shelf, or another PlainShelf's export, opened as a virtual shelf on the
server, the desktop, or Android follows the same rules, with that source in
place of pCloud.

See [Offline copies](concepts/shelf-sources.md#offline-copies) and [Folder
metadata freshness on remote
shelves](concepts/shelf-cache-and-io.md#folder-metadata-freshness-on-remote-shelves).

---

## Concurrent change handling

PlainShelf is designed for single-user operation. Shelf-level structural
operations (creating, moving, trashing, and restoring books) are serialized by a
file lock, and every file write goes through an atomic temp-file-then-rename
pattern, so a crash cannot leave a half-written file in the shelf. However,
per-book mutations (editing metadata, changing covers, updating source content)
are not individually serialized, which produces the following known behavior.

### Last-writer-wins on the same book

When two requests modify the same book concurrently, for example by editing
metadata in two browser tabs, both read the current `book.json`, apply their
changes independently, and write the result back. The second write silently
replaces the first, and the first edit's changes are lost without warning.

Each write stages through its own uniquely named temp file, so concurrent
writers do not collide with each other and no request fails for that reason;
the file left on disk is always one complete write. What is not guaranteed is
that it contains both edits.

Affected operations: metadata updates, cover uploads/deletes, source content
updates, and current-source selection.

For normal single-user, single-tab usage this does not surface. It matters when
several browser tabs or clients edit the same book at the same time, and what it
costs there is a lost edit, never a damaged shelf.

## Text encoding detection

**A text file's encoding is judged from its first 1 MiB.** Importing a `.txt`
or `.md` file, or saving a source's text, detects the encoding (UTF-8, GB18030,
Big5, or UTF-16 with a byte-order mark) from the start of the file and converts
the whole file from that. A file that is plain ASCII for its whole first 1 MiB
and only then switches to Big5 or GB18030 is taken as UTF-8, and the later
Chinese text shows as garbled characters. Convert such a file to UTF-8 before
importing it.
