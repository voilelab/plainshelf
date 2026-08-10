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
  "id": "book-a82m",
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
| `id` | Stable book ID, generated once and never recomputed |
| `title` | Display title |
| `format` | Source format (`txt` or `md`) |
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
| `current_source` | Authoritative pointer to the active source |

!!! warning "`schema_version` is managed by PlainShelf"
    Editing it by hand is unsupported. Raising it is a reliable way to lock
    yourself out of your own books: PlainShelf refuses to write any book whose
    version is higher than the running build understands.

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
- PlainShelf will never write a `book.json` whose on-disk `schema_version` is
  higher than the running build understands. Such a book stays visible and
  readable on a best-effort basis, and every attempt to modify it fails with an
  explicit error rather than overwriting the file.
- The server is no longer the only thing that reads the format. The Android
  client reads a pCloud-held shelf directly, without a server in between. That
  reader is read-only, so it inherits the read half of these promises and never
  the write half: it cannot raise a version, cannot drop a field, and cannot
  rewrite a book it does not fully understand, because it does not write at all.

### What we do not promise

- **Adding a new optional field does not raise the version — and an older build
  that writes such a book will drop that field.** PlainShelf reads `book.json`
  into a fixed set of known fields and rewrites the whole file; keys it does not
  recognize are not preserved. If you run two PlainShelf versions against one
  shelf and edit a book from the older one, values that only the newer version
  knows about are lost. Run one version against a shelf, or upgrade both. A
  read-only reader — the Android client on a pCloud shelf — is exempt from the
  losing half of this, since it never rewrites a book, but it can still be built
  against an older schema than the shelf and show stale or missing fields.
- Reading a book whose `schema_version` is higher than the build supports is
  best-effort. Fields may be missing or misinterpreted, and the displayed
  metadata may be wrong. It is shown so you can see the book exists, not so you
  can rely on it.
- There is no downgrade path. PlainShelf will not rewrite a v2 book back to v1.
  To go back to an older release, restore from a backup taken before the
  upgrade.
- Files other than `book.json` are not versioned yet — see
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

Two things people miss:

- **Also copy the application store** (`--store-path`, or the platform default).
  Bookmarks live there and are *not* derived from `books/`. (Read history and
  reading time are not in the store: each client keeps its own on the device
  that did the reading, so neither is covered by a server-side backup.)
- **Everything under `app/`** — the lock file and temporary files — can be
  discarded safely; the server recreates it.

Stop the server or desktop app before copying if you want a guaranteed-consistent
snapshot. The shelf lock coordinates PlainShelf's own writes; it does not stop
your backup tool from reading a file mid-write.

## v0.8 reading-data breaking change

!!! warning "v0.8 reading data does not carry into v1"
    v1 starts a new, empty reading history and reading-time record on each
    device. It does not migrate v0.8's server-side values, so the old history and
    dashboard activity no longer appear in v1. This is an intentional pre-1.0
    breaking change.

PlainShelf provides no export, import, or recovery path for these values.
Upgrade from v0.8 only if you accept that they will no longer be accessible.

## Restoring from a backup

1. **Stop the server or desktop app.** The shelf lock is not a substitute for
   stopping the process.
2. Restore the shelf directory, and the application store if you backed it up.
3. Start PlainShelf again.

You can skip `app/library.lock` and `app/tmp/` when restoring; they are
recreated on the next startup.

---

## When PlainShelf refuses to write

If a book was last written by a newer PlainShelf than the one you are running,
that book becomes **read-only**. You will see a log line like:

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

Only `book.json` carries `schema_version`.

| Path | Versioned? |
|---|---|
| `books/**/book.json` | Yes — `schema_version`, described on this page |
| `books/**/sources/{id}/meta.json` | No |
| `.trash/**/trash.json` | No |
| Application store | No |

Because source metadata is not versioned yet, an older build can still add a
source folder to a book it cannot otherwise write. The book will not be switched
to point at that source — that change goes through `book.json` and is refused —
but a stray folder can be left behind under `sources/`.

The practical rule while these gaps remain: **run one PlainShelf version against
a shelf at a time.**
