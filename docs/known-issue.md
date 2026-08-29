# Known Issues

## Shelf cache limitations

**If a change you made outside PlainShelf has not appeared, press Update book
list on the library toolbar.** It walks the shelf immediately instead of waiting
for the next interval, and it is the same action in every setup below — local
disk, SMB or NAS, a sync folder, a cloud mount, and the Android client's pCloud
shelf. Every delay below that comes from an interval is bounded by it. See
[Rescanning on demand](concepts/shelf-cache-and-io.md#rescanning-on-demand).

For the operational model, the initial metadata scan, and tuning guidance, see
[Shelf Cache and Disk I/O](concepts/shelf-cache-and-io.md).

Two limits run through every setup, so they are stated once here rather than
repeated below.

**A change made outside PlainShelf is discovered on a timer.** Within
`scan_interval` a refresh may only reopen the books already in the cache rather
than walking the library, so a book added, moved, or deleted by anything other
than PlainShelf can take that long to appear. **Update book list** is what
shortens the wait to now.

**Staleness is judged from `book.json`'s size and modification time.** An edit
that happens to leave both unchanged is not noticed, and **Update book list**
does not change that: the walk finds the file, sees the same size and time, and
keeps what it already had. This is the one limit on this page the button cannot
lift. Nor does a change to a cover or a source file move `book.json` at all, so
cached state derived from a book can stay stale even while the changed file
itself is served correctly. The cover and source contents are not themselves
cached.

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

A file caught mid-sync — being renamed, copied, or still written — can fail to
reopen and is skipped for that pass, then picked up once the sync settles.

Sync clients also tend to preserve or normalize timestamps, which turns the
stat-based blind spot above from a corner case into something you can
realistically hit. Press **Update book list** once the sync has settled.

### A pCloud shelf on the Android client

- **The book list never updates on its own.** Walking the shelf costs one
  recursive listing plus a request per book, so the client scans once and then
  reads the stored copy on the device. A book added, removed, or renamed from
  another device appears only after **Update book list** on the library toolbar.
- **A stale list can make a book fail to open.** The stored copy holds the
  pCloud file references used to open a book. If the book was replaced or moved
  on pCloud since the last update, opening it fails until the list is updated.
  Downloaded books are unaffected — they are read from the device.

The stat rule above governs the device copy too: an update reuses a book's
stored metadata while its size and modification time are unchanged.

---

## Concurrent change handling

PlainShelf is designed for single-user operation. Shelf-level structural
operations (creating, moving, trashing, and restoring books) are serialized by a
file lock, and every file write goes through an atomic temp-file-then-rename
pattern, so a crash cannot leave a half-written file in the shelf. However,
per-book mutations (editing metadata, changing covers, updating source content)
are not individually serialized, which produces the following known behavior.

### Last-writer-wins on the same book

When two requests modify the same book concurrently — for example, editing
metadata in two browser tabs — both read the current `book.json`, apply their
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
