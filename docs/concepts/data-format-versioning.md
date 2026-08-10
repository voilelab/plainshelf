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
  Bookmarks live there and are *not* derived from `books/`. A v0.8 store also
  contains server-side reading history. v1 does not import or display that
  history, so keep the backup if you may want to recover it with v0.8.
- **Keep `app/stats/reading/` when upgrading from v0.8.** Those monthly files
  contain the old reading-time totals. v1 leaves them in place but does not
  read them. The lock file and `app/tmp/` can still be discarded safely; the
  server recreates them.

After upgrading, new reading history and reading time live only on the device
that records them. They are not covered by a server-side shelf backup; back up
the browser profile, desktop configuration directory, or mobile app data if you
need to preserve them.

Stop the server or desktop app before copying if you want a guaranteed-consistent
snapshot. The shelf lock coordinates PlainShelf's own writes; it does not stop
your backup tool from reading a file mid-write.

## v0.8 reading-data breaking change

!!! warning "Export or back up before upgrading from v0.8"
    v1 starts a new, empty reading history and reading-time record on each
    device. It does not import v0.8's server-side values, so the old history and
    dashboard activity no longer appear in v1. This is an intentional pre-1.0
    breaking change.

While v0.8 is still running, a server installation can archive the old JSON
through its existing read APIs. Replace the base URL, shelf id, and date range
with your own values:

```sh
curl --fail --silent --show-error \
  'http://127.0.0.1:20000/api/shelves/default_shelf/read_history' \
  --output plainshelf-v0.8-read-history.json

curl --fail --silent --show-error \
  'http://127.0.0.1:20000/api/shelves/default_shelf/reading_activity?from=2026-01-01&to=2026-12-31' \
  --output plainshelf-v0.8-reading-activity.json
```

If `protect_read` is enabled, add the configured token header to both commands,
for example `--header 'X-PlainShelf-Token: YOUR_TOKEN'`. These JSON files are
archives only: v1 has no importer for them. The history export is an ordered
list of book IDs, so retain the matching shelf backup if you need to resolve
those IDs back to book titles later.

The desktop app does not expose its internal API on a TCP port. Before updating
it, quit the app and copy its complete PlainShelf configuration directory
(`~/Library/Application Support/PlainShelf` on macOS) together with every shelf
directory. That copy contains the v0.8 application store and monthly reading
stats. Do not try to inspect the copy by simply relaunching the v0.8 desktop
app: it always opens the current user's fixed configuration directory, and the
copied `shelves.json` still points at the original shelf paths.

To inspect a desktop backup safely, run the v0.8 `plainshelf-srv` binary against
the copies instead. Create a recovery-only configuration such as:

```yaml
logger:
  level: info
  format: text
  log_file:
    type: stderr

server_conf:
  addr: "127.0.0.1:20001"
  read_timeout: 60s
  write_timeout: 60s

app_conf:
  logger:
    level: info
    format: text
    log_file:
      type: stderr
  shelves:
    - id: default_shelf
      name: Default Shelf
      lib_root: "/absolute/path/to/recovery/shelf"
  store_path: "/absolute/path/to/recovery/PlainShelf/store"
  read_history_limit: 100
  security:
    mode: none
```

Copy each shelf's exact `id` and `name` from the backed-up `shelves.json`, set
every `lib_root` to the absolute path of its **copied** shelf, and repeat the
shelf entry when necessary. `store_path` must point to the copied desktop
`PlainShelf/store` directory. Then start that same v0.8 server version with
`plainshelf-srv -conf /path/to/recovery.yaml` and open
`http://127.0.0.1:20001`; the JSON export commands above work at that address.

The complete recovery sequence is therefore:

1. Stop v1 and make another backup of its current data.
2. Copy the v0.8 shelves and application store to a separate recovery
   location.
3. Point a recovery-only v0.8 server configuration at those copies and use a
   different loopback port from any current PlainShelf server.
4. Start v0.8 with that configuration, then view or export the old history.

Never point v0.8 at the live shelf after v1 has modified it. There is no
supported downgrade or merge path, and new device-local v1 reading data is not
combined with the recovered v0.8 data. If no pre-upgrade backup exists, v1
normally leaves the old store entry and `app/stats/reading/` files untouched;
copy them before attempting best-effort recovery with v0.8.

## Restoring from a backup

1. **Stop the server or desktop app.** The shelf lock is not a substitute for
   stopping the process.
2. Restore the shelf directory, and the application store if you backed it up.
3. Start PlainShelf again.

You can skip `app/library.lock` and `app/tmp/` when restoring; they are
recreated on the next startup. Restore `app/stats/reading/` too only when you
are reconstructing a v0.8 environment to inspect its old reading time.

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
