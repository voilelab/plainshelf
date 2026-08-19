# Data Format Versioning

This page explains how PlainShelf marks the on-disk format of your library, what
happens to your files when you upgrade, how to back up and restore a shelf, and
what PlainShelf does when it meets data it cannot safely write.

For what is stored where, see [Data Model](data-model.md).

---

## Why the shelf has a schema version

Your files outlive the program that wrote them. A shelf created today may be
opened years from now by a much later PlainShelf release — or, on a machine you
forgot to update, by a much older one.

A version marker in the file is what makes that safe. It lets a build recognize
data it does not fully understand and refuse to overwrite it, instead of
silently rewriting the file into a shape that loses information.

---

## `book.json` schema v1

Every book's metadata lives in `book.json` inside its `.bookpkg` folder. As of
schema v1, the first key in that file records the format version:

```json
{
  "schema_version": 1,
  "id": "q7f2mzk4x6rt3vbd",
  "title": "The Tale of Genji",
  "format": "txt",
  "tags": [
    "classic"
  ],
  "identifiers": {
    "isbn": "978-0-14-243714-8"
  },
  "cover": "cover.png",
  "authors": [
    "Murasaki Shikibu"
  ],
  "language": "en",
  "comments": "",
  "star": 5,
  "created_at": "2026-03-15T08:30:00Z",
  "updated_at": "2026-05-01T12:00:00Z",
  "published_at": "2026-03-15",
  "current_source": "20260315-a1"
}
```

| Field | Description |
|---|---|
| `schema_version` | On-disk format version of this file. Managed by PlainShelf. |
| `id` | Stable book ID: a random 16-character word, generated once when the book is created and never recomputed. Shelves from earlier versions carry 8-character hex IDs, which are kept unchanged. See [Book IDs](data-model.md#book-ids) |
| `title` | Display title |
| `format` | Compatibility mirror of the current source format (`txt` or `md`) |
| `tags` | Free-form tags |
| `identifiers` | External identifiers such as `isbn` |
| `cover` | Cover filename relative to the book folder |
| `authors` | Author names |
| `language` | BCP-47 language tag |
| `comments` | Free-form notes |
| `star` | Rating, 0–5 |
| `created_at` | Creation timestamp (RFC 3339) |
| `updated_at` | Last modification timestamp (RFC 3339) |
| `published_at` | Publication date (`YYYY-MM-DD`) |
| `current_source` | Authoritative pointer to the active source; see [Adding and removing sources](data-model.md#adding-and-removing-sources) for how it moves when a source is deleted |

!!! warning "`schema_version` is managed by PlainShelf"
    Editing it by hand is unsupported. Raising it is a reliable way to lock
    yourself out of your own books: PlainShelf refuses to write any book whose
    version is higher than the running build understands.

### Fields PlainShelf does not know

`book.json` is yours to edit, so anything you put in it that is not in the table
above is kept. Add `series`, `douban_id`, or a note of your own, and it is still
there after PlainShelf next writes that book — after a rating, a cover upload, a
move to another layer, or a source switch:

```json
{
  "schema_version": 1,
  "id": "q7f2mzk4x6rt3vbd",
  "title": "The Tale of Genji",
  "…": "…",
  "current_source": "20260315-a1",
  "series": "Genji Cycle",
  "douban_id": 1770782
}
```

Values are written back exactly as you typed them, including nested objects,
array order, and the digits of a number. What is not preserved is where they
sit: PlainShelf writes the fields it knows first, in its own order, and puts
everything else after them in alphabetical order.

Two consequences are worth knowing:

- **A key PlainShelf knows always wins.** If a later release adds a field with
  the name you were already using for something else, that field takes the key
  over. The file never ends up with two values for one key.
- **These keys stay in the file.** They are not returned by the HTTP API and not
  copied into the exported book cache, because nothing in PlainShelf can
  interpret them. They are a fact about your file, not part of an interface.

---

## Reading a shelf created before v1

Files written before schema versioning existed have no `schema_version` key.
PlainShelf reads them as v1.

**Opening your library never rewrites it.** Nothing is migrated at startup, and
no file is touched just because you looked at it. The version is written to a
book only the next time you actually change that book — editing its metadata,
changing its cover, or switching its current source.

This has two consequences worth knowing:

- A shelf can sit half-upgraded indefinitely. Some books carry
  `schema_version`, others do not, and that is a perfectly normal state.
- File modification times change one book at a time, as you edit them, rather
  than all at once after an upgrade. Backup tools see a trickle, not a
  library-wide rewrite.

---

## Source metadata schema v1

New `sources/{id}/meta.json` files also carry `schema_version: 1` and an
authoritative `format` (`txt` or `md`). A source without those fields is a
legacy source. It is still listed and still readable, but nothing interprets its
`split_config` any more: it renders as `book.json`'s `format` says, which means
one plain-text section unless the book is Markdown. Run the migration tool below
to give it chapters again.

Source schema v1 is written only when creating a new source, including imports
and explicit TXT/Markdown conversions. A source whose schema version is newer
than this build remains readable, but content, comment, asset, and delete
operations are refused before touching its files.

Deleting a book's *current* source also writes `book.json`, because the pointer
has to be handed over to another source first. That makes it a write to the book
as well as to the source, so it is refused for a book whose own schema version is
newer than this build understands. Deleting any other source only touches that
source and does not write `book.json` at all.

### Migrating legacy sources in place

Because legacy sources are never upgraded on their own, a shelf can carry them
indefinitely. `cmd/migrate-legacy-sources` upgrades them all in one pass. It is
opt-in, one-off, and not part of the server or any release build:

```sh
go run ./cmd/migrate-legacy-sources -shelf ./shelf              # dry run
go run ./cmd/migrate-legacy-sources -shelf ./shelf -dry-run=false
```

It takes the shelf directory itself, not a server config, so it works on a
detached copy of a shelf as readily as on the live one. Older PlainShelf
releases had a shelf-wide `default_split_config` setting that legacy sources fell
back on; if a shelf relied on one, repeat it here with
`-default-split-config '{"type":"line_count","line_count":500}'`, or those
sources migrate as the single-chapter text they would be without it.

For each legacy source it stamps `schema_version` and the format the source
renders as today, and resets the split config the new schema ignores. Where that
split actually produced chapters, it first bakes them into the text as `## `
headings, rewriting the source in place. A source whose split produces nothing
keeps its bytes untouched.

Before running it with `-dry-run=false`:

- **Stop the server and the desktop app.** The tool takes the shelf lock, which
  stops two migrations racing each other, but a running PlainShelf holds that
  lock only for the length of one operation — so it cannot tell you one is
  running. A concurrent run is actively harmful; closing PlainShelf first is
  your job.
- **Back up the shelf directory.** The rewrite is in place and there is no undo.

`-dry-run` is the default and performs the full computation, so its report is a
real rehearsal. Read it before applying. Two things in it deserve attention:

- Sources reported as `needs-attention` are left legacy and untouched. That
  happens when a split regex uses JavaScript-only syntax Go's engine cannot run,
  or when the split type is not one this build knows. The tool cannot reproduce
  those chapters, and guessing at them is not something an unundoable in-place
  rewrite should do. Such a source reads as one section; add `## ` headings to
  its text in the source editor to give it chapters.
- A split that names no boundary at all — a line count of zero, a blank pattern,
  or a regex that matches nothing — is not an error. It is what "no chapters"
  looks like, so the source is stamped with the format it already rendered as
  and its bytes are left alone.
- The per-source chapter count is there to be compared against what the reader
  has been showing. The tool translates the two known dialect differences that
  would otherwise lose chapters silently (JavaScript treats a carriage return as
  a line terminator for `^`/`$`; its `\s` covers the ideographic space and other
  Unicode spaces), but it cannot guarantee every pattern means the same thing in
  both engines.

---

## Compatibility policy

Starting with `book.json` schema v1, PlainShelf makes the following
commitments. They cover the **on-disk format only** — the HTTP API and the user
interface are still pre-alpha and may change.

Releases before v1 are not covered by this compatibility promise. In
particular, v0.8's server-side reading history and reading time are treated as a
documented breaking change, not as data that v1 guarantees to migrate.

### What we promise

- A shelf whose books are at schema v1 stays readable by every later PlainShelf
  release in the 1.x line. We will not remove v1 read support within 1.x.
- The schema version is raised **only** when a change cannot be read correctly
  by an older build: a field changing meaning or type, a field being removed, or
  a new field becoming required. Cosmetic and additive changes do not raise it.
- Upgrades are lazy and per-book. Opening a library never rewrites it. A book is
  written in the new format only when you next change something about that book.
- PlainShelf will never write a `book.json` or source `meta.json` whose on-disk
  `schema_version` is higher than the running build understands. Such data stays visible and
  readable on a best-effort basis, and every attempt to modify it fails with an
  explicit error rather than overwriting the file.
- Keys in a `book.json` that PlainShelf does not recognize are written back
  unchanged, so a field you added by hand — or one a newer release added and
  this build knows nothing about — survives every write. See
  [Fields PlainShelf does not know](#fields-plainshelf-does-not-know).
- The server is no longer the only thing that reads the format. The Android
  client reads a pCloud-held shelf directly, without a server in between. That
  reader is read-only, so it inherits the read half of these promises and never
  the write half: it cannot raise a version, cannot drop a field, and cannot
  rewrite a book it does not fully understand, because it does not write at all.

### What we do not promise

- **Only `book.json` carries unknown keys through.** A source's `meta.json` and
  a `trash.json` are still read into a fixed set of fields and rewritten whole,
  so a key an older build does not recognize is dropped from those files.
- **Adding a new optional field does not raise the version.** An older build
  keeps such a field in `book.json` instead of dropping it, but it cannot act on
  it: it will not show the value, and a field whose meaning depends on another
  one it does not understand can still end up inconsistent. It shows and edits
  the book as its own schema sees it. Run one version against a shelf, or
  upgrade both. A read-only reader — the Android client on a pCloud shelf —
  never rewrites a book at all, but it can still be built against an older
  schema than the shelf and show stale or missing fields.
- Reading a book whose `schema_version` is higher than the build supports is
  best-effort. Fields may be missing or misinterpreted, and the displayed
  metadata may be wrong. It is shown so you can see the book exists, not so you
  can rely on it.
- There is no downgrade path. PlainShelf will not rewrite a v2 book back to v1.
  To go back to an older release, restore from a backup taken before the
  upgrade.
- Files other than `book.json` and source `meta.json` are not versioned yet — see
  [What is versioned today](#what-is-versioned-today). They get the same
  treatment as they are covered, not retroactively.
- Hand-edited `book.json` files are read on a best-effort basis. Malformed JSON
  makes that book unopenable; the error is logged with the file path.

---

## Back up before upgrading

The shelf is plain files, so a backup is a copy of a directory:

```sh
cp -a /path/to/shelf /path/to/backup/shelf-2026-07-28
# or
rsync -a /path/to/shelf/ /path/to/backup/shelf-2026-07-28/
```

Three things people miss:

- **Also copy the application store** (`--store-path`, or the platform default)
  if you want to preserve server settings. Reading progress, history, and time
  are not in that store: each client keeps its own on the device that did the
  reading, so none is covered by a server-side backup. Back up the browser
  profile or desktop app data directory separately if those records matter.
- **Everything under `app/`** — the lock file, temporary files, and the exported
  book caches — can be discarded safely; the server recreates it.
- **`trash/` is not in that category.** It holds books you deleted but have not
  emptied yet, and nothing rebuilds them. Copying the shelf directory as shown
  above already includes it; only leave it out if you are certain you want the
  backup to drop those books. Older shelves keep the same directory hidden as
  `.trash/`, so a backup command that skips dotfiles silently loses it — see
  [`trash/` was `.trash/` before](data-model.md#trash-was-trash-before).

Stop the server or desktop app before copying if you want a guaranteed-consistent
snapshot. The shelf lock coordinates PlainShelf's own writes; it does not stop
your backup tool from reading a file mid-write.

## v0.8 reading-data breaking change

!!! warning "v0.8 reading data does not carry into v1"
    v1 starts a new, empty reading history and reading-time record on each
    device. Web and desktop reading progress also starts at zero instead of
    importing v0.8's server-side bookmarks. Existing Android progress was
    already on-device and remains available. The old server-side history,
    dashboard activity, and bookmarks are not migrated or read by v1. This is
    an intentional pre-1.0 breaking change.

PlainShelf provides no export, import, or recovery path for these values.
Upgrade from v0.8 only if you accept that they will no longer be accessible.

## Restoring from a backup

1. **Stop the server or desktop app.** The shelf lock is not a substitute for
   stopping the process.
2. Restore the shelf directory, and the application store if you backed it up.
3. Start PlainShelf again.

You can skip `app/library.lock`, `app/tmp/`, and `app/book-cache-*.json` when
restoring; they are recreated on the next startup. Restore `books/` and `trash/`
in full: both hold books, and a restored `.trash/` from an older backup is
renamed to `trash/` on the next start.

---

## When PlainShelf refuses to write

If a book or source was last written by a newer PlainShelf than the one you are
running, that object becomes **read-only**. You may see a log line like:

```text
WARN book.json schema version is newer than this build supports; book is read-only
     path=books/example.bookpkg/book.json schema_version=2 supported=1
```

The book still appears in your library and still opens. Any attempt to modify it
fails, and the API returns `409 Conflict`:

```text
book uses a newer on-disk format than this PlainShelf build supports;
upgrade PlainShelf to modify it
```

**Your file was not modified.** The refusal is the protection — it is what stops
an older build from stripping fields it does not understand. There are two ways
forward:

- **Upgrade PlainShelf** to a build that understands the newer format. This is
  almost always what you want.
- **Restore that book's folder from a backup** taken before the newer build
  touched it, if you intend to stay on the older release.

---

## What is versioned today

Book and source metadata carry independent schema versions.

| Path | Versioned? |
|---|---|
| `books/**/book.json` | Yes — `schema_version`, described on this page |
| `books/**/sources/{id}/meta.json` | Yes — `schema_version`; v1 owns source `format` |
| `trash/**/trash.json` | No |
| Application store | No |

The practical rule remains: **run one PlainShelf version against a shelf at a
time.** Unknown keys in `book.json` now survive such a write, but unversioned
files do not carry them, and each build still shows and edits a book as its own
schema sees it.
