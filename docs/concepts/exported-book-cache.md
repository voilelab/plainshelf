# The exported book cache

The caches in [Shelf cache and disk I/O](shelf-cache-and-io.md) are in-memory
state, rebuilt from the shelf whenever PlainShelf starts. They do nothing for a
client that reads the shelf directly instead of through the server.

The Android app opening a shelf from pCloud is that client. It has no server to
ask, so it walks the shelf itself, and each book costs two HTTP requests — one
to get a download link for `book.json`, one to fetch it. On a shelf of any size
that is slow, and it can hit pCloud's request limit.

So the server and the desktop app write the listing down. Each one exports
`app/book-cache-{writer-id}.json`, containing every book's `book.json`, each
book's package path, and the shelf's folders. A client downloads that one file
instead of walking the shelf, and the cost of opening a library stops growing
with the number of books in it.

One value in it is computed rather than copied: each entry carries `nsfw`, the
finished answer to whether that book is [adult
content](folders.md#marking-a-folder-as-adult-content), assembled from the
folder rules in `shelf.json` and the book's own flag. It is there so that a
client which downloads only this file, which is the point of the file, reaches
the same answer as the server without reading `shelf.json` itself. Like the flag
in `book.json`, it is written only when `true`.

Folders with a [`folder.json`](folders.md#folder-metadata) are listed under
`folder_meta`, keyed like the folder list, with the icon as written. A client
reading the export gets the icons without opening one `folder.json` of its own;
in exchange, an icon changed on the server reaches it when the server next
exports, like any other change. The member is omitted when no folder has
metadata, so such a shelf exports what it did before the field existed, and a
client built before it ignores it.

Each entry also carries `current_source_md5`, the content hash from the current
source's `meta.json`. Editing a source's text in place rewrites that file but
not `book.json`, so without it a client holding an
[offline copy](shelf-sources.md#offline-copies) could not tell the text had
changed without reading every source. It is omitted when unknown.

An entry carries `current_source_format` only when the current source's
`format` differs from the `format` mirrored in `book.json`, which a metadata
edit can change on its own. A remote reader uses it to tell an
[images source](data-model.md#image-sources) from a text one, and falls back to
the mirror when it is absent, so a shelf whose mirrors agree exports the bytes
it did before the field existed.

## When it is written

- after the initial scan, so a client that arrives before anything else has read
  the shelf still finds a file;
- once `book_cache_interval` (default one hour) has elapsed since the last
  export, the next time the shelf is read or written;
- when the shelf is closed;
- on demand, from **Settings → Shelves → Mobile book cache**, or `POST
  /api/shelves/{shelf_id}/book-cache-exports`.

That manual export also forces a rescan, but it is not the way to ask for one:
it exists to publish the file a phone reads, and it writes to the shelf. To make
a book appear in the library, use
[Rescanning on demand](shelf-cache-and-io.md#rescanning-on-demand).

Each of those is a moment the export is *considered*, not a write. Whether
anything reaches the disk is decided by comparing the content against what was
written last: an unchanged shelf is left alone, because an identical rewrite is
a pointless upload on exactly the cloud and network shelves this feature exists
for. That comparison is also why there is no "something changed" flag anywhere —
book metadata is edited in place, and a flag would have to be set by every edit
path, with the one that got missed silently ending the exports.

There is no background timer either. An export only has something new to say
after the shelf has been read or written, and every such path already passes
through the check.

## What `timestamp` means

The manual trigger always rescans the shelf first, because `timestamp` records
when the walk behind the file **began** — not when the file was written, and not
when the walk finished.

A client uses it to decide which books it can take from the cache: a `book.json`
whose modification time is at or after that second is read directly instead. All
three choices are the conservative one:

- a write time would claim freshness the walk never verified;
- the walk's *end* time would cover a book edited while the walk was still
  running, after it had already been read;
- and because both sides carry whole seconds, an edit inside the same second as
  the walk cannot be ordered against it, so equality means "read it".

Each of those costs a couple of requests for one book. The alternative is
showing a title or current source that is quietly wrong.

## Folder metadata freshness on remote shelves

`folder_meta` shares the export's `timestamp`: it is the metadata observed by
that scan, not a promise that every folder is still unchanged now. The writer
captures the folder paths, metadata, book entries and scan timestamp together,
so a concurrent rescan cannot mix two listings in the export or offline snapshot.

A virtual shelf (`NewShelfFromFS`, including pCloud and LAN shares) takes the
whole listing from the newest usable export and checks for another export at
`scan_interval`. It does **not** stat individual `book.json` or `folder.json`
files to validate that listing. The per-book mtime rule in
[Keeping the cache fresh](shelf-cache-and-io.md#keeping-the-cache-fresh) is not applied to
this export-backed listing; opening an individual book is a separate read. An
icon edited on the source reaches the client after the writer next scans and
exports, then the client refreshes. Manual rescan walks the source directly and
revalidates its `folder.json` files.

A legacy export without `folder_meta` shows default icons and makes no extra
folder requests. It does not prevent a later export from supplying metadata.
Exports older than the current listing are ignored; at the same timestamp second,
a changed content digest still refreshes icons, including clearing an icon.

With an offline store configured, the listing and metadata survive restart in
the local snapshot. If the source is unavailable, the current listing or stored
snapshot remains visible with its original freshness. Reconnecting applies the
source's usable export again, including metadata changed within the same second.
Without an offline store there is no persisted listing to restore on an offline
cold start. A source with no usable export is walked at first open (or restored
from an existing offline snapshot); later interval checks look for an export,
and manual rescan can walk it again.

## Folder metadata I/O measurement

Measured on Linux with local temporary files and a counting `ReadFS`, 2026-10-19:
40 folder directories, five with `folder.json`, and no books in the remote case.
The local rescan case also holds one book per folder. Counts describe metadata
costs; they are not measured SMB or pCloud latency.

| Operation | Observed I/O | Folder metadata cost |
|---|---|---|
| Local cold scan | 41 folder directory listings | Five `folder.json` opens/reads, each also checking the opened file's stat |
| Local warm rescan | 41 directory snapshots reused, zero directory listings | The same five opens/reads; directory mtime never substitutes for reading existing metadata |
| Warm folder and metadata listings | Zero `ReadFS` calls within the intervals | Zero |
| Remote cold open from one export | Open `shelf.json`, list `app/`, open one export | Zero accesses under `books/` |
| Remote export refresh | List `app/`, open one export | Zero accesses under `books/` |

The tests are `TestFolderMetaScanReadsOnlyItsFolders` and
`TestFolderMetaRemoteListingIO` in `shelf/shelf_folder_meta_cache_test.go`.
On SMB, each metadata open/read/stat may require network round trips depending
on the OS and mount cache; five files do not imply exactly five wire requests.
On a request-metered backend an export read may require a link lookup and a
download, but the number does not grow per folder. Actual SMB/NAS, pCloud, LAN,
Windows, macOS and Android transport timings were not measured here.

## Several machines, one shelf

The writer ID is generated on first start and stored outside the shelf, with the
server's other settings. It identifies the installation, not the shelf, so a
machine that opens three shelves writes the same ID into all three, and two
machines sharing one shelf keep separate files. A client picks the most recently
written one.

An installation removes another's file only after 30 days without an update —
long enough that a machine switched off for a holiday keeps its entry. Files it
cannot parse are never removed, so anything else under `app/` is left alone.

## When its version does not match

The file carries a `schema_version` (`BookCacheSchemaVersion` in
`shelf/shelf_cache_export.go`), versioned the throw-away way, like the
[fingerprint cache](similarity-fingerprints.md#fingerprint-cache): there is no
migration path, and a reader that meets a version it does not recognize
discards the whole file and falls back to walking the shelf. A new field such as
`nsfw` or `current_source_md5` is an ordinary addition. Nothing here is frozen,
and a reader that does not know a field simply does not read it.

### One reader

Every shelf is read by the Go shelf, including the Android app's pCloud shelf,
which the app opens in its embedded Go server (`shelf.NewShelfFromFS` over
`internal/pcloud`). The frontend parses none of the format: it once
carried a second, TypeScript reader, kept in step with Go only by the shared
fixture in `shelf/testdata/conformance/`. A format change is therefore made in
Go alone, and `BookCacheSchemaVersion` and `book.json`'s version have no
hand-maintained copy elsewhere.

### What a version mismatch costs a remote shelf

A cache whose `schema_version` the reader does not know is no cache at all
(`newestExportedBookCache` skips it): a half-understood payload would surface as
a silently truncated library rather than the cache miss it really is.

That refusal is safe but not free. Without a usable exported file, a pCloud
shelf is walked, which reads each book's `book.json` over the network in place
of the single download the cache would have been. Nothing is wrong and nothing
is lost, but on a metered or high-latency connection that is a real performance
cliff, and it is the everyday shape of "the server upgraded, the phone did not":
a build that raises `BookCacheSchemaVersion` starts exporting a file the older
app declines, until that app is updated too.

This is the one sense in which "deleting the book cache is safe" needs a
qualifier. From the server's side it is exactly as safe as the other caches under `app/`: the
next export rebuilds it, and [Data model](data-model.md#app) says as much. From
a remote reader's side, deleting or invalidating it is the difference between
one download and a full per-book walk — still safe, still never an error, but no
longer free.

## Notes

- The file is disposable. Delete it and it comes back; a client that finds none,
  or one it cannot read, falls back to scanning the shelf.
- Its `schema_version` is a cross-device contract: the phone reader rejects a
  file whose version it does not match exactly and walks the shelf book by book
  instead — safe, but a per-book request cliff over pCloud, not free. See
  [What a version mismatch costs a remote shelf](#what-a-version-mismatch-costs-a-remote-shelf).
- It contains what `book.json` already contains, plus the computed `nsfw` and
  `current_source_md5` above.
  If your shelf is private, it is no more revealing than the shelf itself — but
  it does put every book's metadata in one file, which matters if you share
  `app/` more widely than `books/`.
- `book_cache_writer_id` can be pinned per shelf in the config file if you want
  a predictable name; leave it unset and PlainShelf supplies one.
