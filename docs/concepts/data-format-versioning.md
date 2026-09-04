# Data Format Versioning

This page explains how PlainShelf marks the on-disk format of your library, what
happens to your files when you upgrade, and what PlainShelf does when it meets
data it cannot safely write.

For what is stored where, see [Data Model](data-model.md); for how to copy your
library and put it back, see [Backup and Restore](../backup-and-restore.md).

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
  "id": "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d",
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
  "current_source": "20260315-083000"
}
```

| Field | Description |
|---|---|
| `schema_version` | On-disk format version of this file. Managed by PlainShelf. |
| `id` | Stable book ID: a version 4 UUID, generated once when the book is created and never recomputed. Shelves from earlier versions carry 8-character hex IDs or 16-character base32 words, which are kept unchanged. See [Book IDs](data-model.md#book-ids) |
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
| `current_source` | Authoritative pointer to the active source. A source ID is the second-granularity timestamp of when the source was created (`YYYYMMDD-HHMMSS`), with a `-1`, `-2`, … suffix only when two are created in the same second. See [Adding and removing sources](data-model.md#adding-and-removing-sources) for how the pointer moves when a source is deleted |

!!! warning "`schema_version` is managed by PlainShelf"
    Editing it by hand is unsupported. Raising it is a reliable way to lock
    yourself out of your own books: PlainShelf refuses to write any book whose
    version is higher than the running build understands.

---

## Reading a shelf created before schema v1

Files written before schema versioning existed have no `schema_version` key.
PlainShelf reads them as schema v1.

**Opening your library never rewrites a book.** No `book.json` is upgraded at
startup, and no book file is touched just because you looked at it. The version
is written to a book only the next time you actually change that book — editing
its metadata, changing its cover, or switching its current source.

One shelf-level migration does run at startup, and it is not a book rewrite: a
shelf whose trash still lives in the old hidden `.trash/` directory has it
renamed to `trash/` the first time a newer build opens it (`data-model.md`'s
[`trash/` was `.trash/` before](data-model.md#trash-was-trash-before) describes
what the rename does). That rename carries no `schema_version` of its own, so it
sits outside the per-file versioning on this page — and it does not reverse.
Open the same shelf again with a build old enough to still expect `.trash/` and
the trashed books, now under `trash/`, drop out of that build's trash view until
they are moved back by hand.

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
one plain-text section unless the book is Markdown. To give it chapters again,
convert it in the source editor (see below).

Source schema v1 is written only when creating a new source, including imports
and explicit TXT/Markdown conversions. A source whose schema version is newer
than this build remains readable, but content, comment, asset, and delete
operations are refused before touching its files.

Deleting a book's *current* source also writes `book.json`, because the pointer
has to be handed over to another source first. That makes it a write to the book
as well as to the source, so it is refused for a book whose own schema version is
newer than this build understands. Deleting any other source only touches that
source and does not write `book.json` at all.

### Giving a legacy source chapters again

A legacy source is never upgraded on its own, so a shelf can carry one
indefinitely, and opening the shelf changes nothing about it. It stays listed
and stays readable, rendering as its `book.json` `format` says — one plain-text
section, unless the book is Markdown, in which case the `## ` headings already in
its text still divide it into chapters. What no build does any more is read a
source's chapter structure out of its own metadata.

To give a plain-text legacy source chapters, open it in the source editor and run
a **TXT → Markdown** conversion, either by a regular expression or by a fixed line
count. The conversion writes the chapter boundaries into the text as `## `
headings and saves them as a **new** schema v1 source, leaving your legacy source
untouched, so nothing about the original is lost. Making that new source the
book's current source also switches `book.json`'s compatibility `format` to `md`;
pointing the book back at the legacy source restores its text but not that format
mirror, so correct the book's `format` back to `txt` as well if you want it read
as plain text again. This per-source conversion is the only supported way to
re-chapter a legacy source — there is no batch or in-place upgrade, and a one-off
migration tool that once did this shelf-wide has been removed.

Two fields survive in older `meta.json` files and no longer do anything:

- **`split_config`** was a per-source chapter model. It is now just an ignored
  unknown key: it decodes into nothing, cannot be set, and is dropped the next
  time anything rewrites that source's `meta.json`, the same as any other
  unrecognized key. The chapters it once produced are not reconstructed on read;
  the source-editor conversion above is how you get them back.
- **`default_split_config`** was a shelf-wide fallback split that earlier
  PlainShelf releases applied to a source carrying no `split_config` of its own.
  The setting no longer exists and nothing consults it.

---

## Trash metadata schema v2

Every book you delete moves into `trash/books/{id}.bookpkg` with a `trash.json`
beside its `book.json`. That file, too, records its format version in the first
key:

```json
{
  "schema_version": 2,
  "deleted_at": "2026-08-12T09:41:00Z",
  "original_path": "books/fiction/genji.bookpkg",
  "original_folder": [
    "fiction"
  ],
  "delete_reason": "user"
}
```

`trash.json` is the only record of where a book came from, so it is versioned
for the same reason `book.json` is: a build that rewrote it with fields it did
not understand would restore the book to the wrong place. The version is written
only when a book is moved to the trash, never by listing it.

Schema v2 renamed the origin key from `original_layer` to `original_folder`, part
of the same folder rename that runs through the rest of PlainShelf. It is a hard
cut with no dual read: a v1 record's `original_layer` is not looked at, so a book
trashed by a pre-v2 build is still listed and still restorable, but it returns to
the top level of `books/` rather than the folder it came from. The version bump
makes that visible instead of silent — a pre-v2 build that meets a v2 record
refuses to modify it rather than rewriting it and dropping the restore path.

A trashed book whose `trash.json` is newer than the running build stays in the
trash list and keeps showing where it came from, but this build will not modify
it. Restoring it and permanently deleting it both fail with `409 Conflict`, and
the refusal happens before anything moves, so the book is never left half
restored. Emptying the trash is background work rather than a single request: it
skips that book, reports the sweep as partially completed, and leaves the book
in the trash.

Deleting a book is also a *write to the trash*: if the book folder already
carries a `trash.json` from a newer build — which happens when a book was
carried back out of the trash by hand — moving it to the trash is refused
before the book is moved.

---

## Compatibility policy

PlainShelf has not released 1.0.0 yet, so what the on-disk format guarantees
today is not what it will guarantee once 1.0 ships. The two are described
separately below: what a shelf on the current 0.x series can rely on now, and
the longer commitments that begin at 1.0. Read the 1.0 commitments as a
statement of intent, not as protection a 0.x shelf already has.

### Now — the 0.x series

During the v0.x series the on-disk format may still change in breaking ways
between releases. Such changes are announced in the changelog with a
`Breaking (pre-1.0)` marker — v0.8's reading data
([below](#v08-reading-data-breaking-change)) is one of them. Concretely, for a
shelf you are running on a 0.x build today:

- **Reading is not promised to survive a minor upgrade.** Moving from, say, 0.9
  to 0.10 may change how the format is read. Nothing here commits a later 0.x
  build to reading an earlier one's shelf unchanged.
- **No data migration is promised.** PlainShelf does not undertake to carry 0.x
  data forward across a breaking change. Where it drops data it says so in the
  changelog, as it did for v0.8's server-side reading history and reading time.
- **The refusal to write a newer format already protects you.** This is the one
  guarantee that holds today rather than at 1.0: PlainShelf will not write a
  `book.json`, source `meta.json`, or `trash.json` whose on-disk
  `schema_version` is higher than the running build understands (`book.go:229`,
  `source.go:87`, `trash.go:387`). Such an object stays readable on a
  best-effort basis, and every attempt to modify it fails with an explicit error
  instead of overwriting the file. So an older build cannot silently rewrite an
  object whose `schema_version` a newer build actually raised — its guard refuses
  it (see [When PlainShelf refuses to write](#when-plainshelf-refuses-to-write)).
  It does **not** cover a field a newer build added *without* raising the version:
  that object still reads as a version the older build accepts, so the guard does
  not fire and the field is dropped on the older build's next write — see
  [What we do not promise](#what-we-do-not-promise).

Beyond that write refusal, treat the 0.x on-disk format as unstable: keep a
backup before each upgrade, and do not rely on the 1.0 commitments below.

### From PlainShelf 1.0 on

**These commitments take effect with PlainShelf 1.0.0, which has not shipped
yet — until it does, they are not in force.** From 1.0.0 on, for any shelf whose
books are at `book.json` schema v1, PlainShelf makes the following commitments.
They cover the **on-disk format only** — the HTTP API and the user interface are
still pre-alpha and may change. Releases before 1.0 are not covered: v0.8's
server-side reading history and reading time, in particular, are a documented
breaking change, not data that 1.0 guarantees to migrate.

#### What we promise

- A shelf whose books are at schema v1 stays readable by every later PlainShelf
  release in the 1.x line. We will not remove schema v1 read support within 1.x.
- The schema version is raised **only** when a change cannot be read correctly
  by an older build: a field changing meaning or type, a field being removed, or
  a new field becoming required. Cosmetic and additive changes do not raise it.
- Upgrades are lazy and per-book. Opening a library never rewrites a book; a book
  is written in the new format only when you next change something about that
  book. Source `meta.json` is the one exception to the *upgrade* half: it is
  never raised as a side effect of an ordinary write. A legacy source keeps its
  unversioned metadata even when something else about it is written — the write
  preserves the version it found rather than stamping the current one. A legacy
  source is therefore never upgraded in place; to get a schema v1 source with
  chapters, convert it in the source editor — see
  [Giving a legacy source chapters again](#giving-a-legacy-source-chapters-again).
- PlainShelf will never write a `book.json`, source `meta.json`, or `trash.json`
  whose on-disk `schema_version` is higher than the running build understands.
  Such data stays visible and readable on a best-effort basis, and every attempt
  to modify it fails with an explicit error rather than overwriting the file.
  For a trashed book that covers restoring, permanently deleting, and moving a
  book into the trash on top of such a record.
- The server is no longer the only thing that reads the format. The Android
  client reads a pCloud-held shelf directly, without a server in between. That
  reader is read-only, so it inherits the read half of these promises and never
  the write half: it cannot raise a version, cannot drop a field, and cannot
  rewrite a book it does not fully understand, because it does not write at all.

#### What we do not promise

- **Top-level keys PlainShelf does not recognize are removed the next time it
  writes that book.** PlainShelf reads `book.json` into the fixed set of fields
  in the table above and rewrites the whole file from them; anything else in the
  file is not carried over. This applies to a key you added by hand and to one a
  newer build wrote — which is why adding an optional field does not raise the
  schema version, and why editing such a book from an older build loses the
  values only the newer one knows about. Run one version against a shelf, or
  upgrade both. A read-only reader — the Android client on a pCloud shelf — is
  exempt from the losing half of this, since it never rewrites a book, but it can
  still be built against an older schema than the shelf and show stale or missing
  fields — which it says on the book's page rather than leaving you to guess (see
  [What a version mismatch costs the phone](#what-a-version-mismatch-costs-the-phone)).
  [Hand-editing `book.json`](#hand-editing-bookjson) below says which edits do
  survive.
- Reading a book whose `schema_version` is higher than the build supports is
  best-effort. Fields may be missing or misinterpreted, and the displayed
  metadata may be wrong. It is shown so you can see the book exists, not so you
  can rely on it.
- There is no downgrade path. PlainShelf will not rewrite a schema v2 book back
  to schema v1.
  To go back to an older release, restore from a backup taken before the
  upgrade — see [Rolling back to an older
  release](../backup-and-restore.md#rolling-back-to-an-older-release).
- These promises cover `book.json`, source `meta.json`, and `trash.json` — the
  user-data files. The caches under `app/` carry a `schema_version` of their own,
  but the opposite kind: no migration, discarded and rebuilt on any mismatch, so
  they are not what this section commits to — see
  [What is versioned today](#what-is-versioned-today). A file that gains
  user-data versioning later gets these promises from then on, not
  retroactively.
- Hand-edited `book.json` files are read on a best-effort basis. Malformed JSON —
  which includes a duplicated key and invalid UTF-8, not only a missing brace —
  makes that book unopenable; the error names the file and the key at fault. See
  [How the file is parsed](#how-the-file-is-parsed).

### Hand-editing `book.json`

`book.json` is a plain file and nothing stops you from opening it in an editor.
Two things decide what happens to your edit: whether the file still parses, and
whether PlainShelf knows the field.

#### How the file is parsed

PlainShelf reads its metadata files strictly, and says so rather than guessing:

- **Field names are matched exactly, including case.** `"Title"` is not
  `"title"` — it is a key PlainShelf does not know, so the title you typed is
  not read, and the key itself is dropped by the next write to that book (see
  below). The same goes for `"Star"`, `"AUTHORS"`, and every other variant. There
  is no fuzzy matching and no auto-correction: check the spelling against the
  [schema table](#bookjson-schema-v1).
- **A key written twice is refused.** `"title"` appearing two times does not
  quietly resolve to the last one; the file is rejected and the book will not
  open until you delete one of them.
- **Invalid UTF-8 is refused.** Save the file as UTF-8. An editor that wrote
  Big5 or Shift-JIS bytes into it makes the book unopenable.
- **Anything after the closing brace is refused.** A half-finished edit that
  leaves a second object, or stray text, behind is not read as the first object
  plus junk.

A file that fails any of these makes that one book unopenable. It does not stop
the rest of the shelf from loading: the other books list as usual, and the error
names the file and the key at fault — `books/Dune.bookpkg/book.json: duplicate
object member name "title"` — in the log, and in the message PlainShelf answers
with when you act on that book.

The same rules apply to a source's `meta.json`, to `trash.json`, and to
[`shelf.json`](folders.md) — except that a `shelf.json` PlainShelf cannot read
is reported and then ignored, leaving the built-in defaults in place, rather than
stopping anything. The caches under `app/` are the exception in the other
direction: they are rebuildable, so an unreadable one is silently discarded and
recomputed.

#### What survives a write

**Any top-level key outside the schema is gone after the next write to that
book.** Add `"series": "The Tale of Genji"` or `"douban_id": "1770782"` next to
`"title"` and it stays there — until something writes the file. Then the file is
rebuilt from the known fields and your key is not one of them. There is no
warning, no log line, and no copy of the previous file.

The actions that write `book.json` go well beyond what looks like metadata
editing:

- setting or clearing a star rating;
- editing the title, authors, tags, identifiers, language, published date, or
  comments;
- uploading, replacing, or deleting the cover;
- importing a file into the book, or converting a source between TXT and
  Markdown;
- switching the current source, or deleting the source that is currently active;
- moving the book to another folder.

Treat that list as illustrative rather than complete: assume every change you
make to a book rewrites its `book.json`. Rating one book does not touch any
other, so a hand-added key disappears one book at a time — the shelf keeps
looking fine while the book you just rated has quietly lost it.

None of this is a reason to leave the file alone. These edits are safe:

- **Changing the value of any field in the
  [schema table](#bookjson-schema-v1), within the type and range that table
  gives** — title, authors, tags, identifiers, language, comments, star,
  published date. A value that fits is read back exactly as written. A value
  that does not is worse than a typo, because PlainShelf validates on write
  rather than on read: `"star": 6`, a `language` that is not a BCP-47 tag, or a
  blank key in `identifiers` all load fine, and then every later edit to that
  book is refused until you repair the file by hand. A `published_at` that is
  neither `YYYY-MM-DD` nor an RFC 3339 timestamp is worse still — it fails at
  read time and makes the book unopenable, exactly like malformed JSON.
  `schema_version` is its own exception: it is managed by PlainShelf, and
  raising it locks you out of the book.
- **Renaming the `.bookpkg` directory, or moving it into another folder with a
  file manager.** Identity lives in the `id` field, not in the path — see
  [Book IDs](data-model.md#book-ids).
- **Deleting `CURRENT_SOURCE.txt`.** It is a hint that is never read back; see
  [Book folder](data-model.md#book-folder-bookpkg).

If you have custom data you want to keep, put it somewhere PlainShelf owns or
somewhere PlainShelf never touches, not in an invented key:

- **An external identifier belongs in `identifiers`.** Its keys are free-form,
  so `douban_id` goes there and survives every write. The book edit screen
  exposes it, so you do not have to hand-edit the file at all.
- **A short note or a grouping belongs in `comments` or `tags`.** A series name
  works as a tag today.
- **Anything larger belongs in a file of your own.** PlainShelf neither reads nor
  deletes files it does not know about, so a `notes.md` you drop in the
  `.bookpkg` directory itself travels with the book, including into `trash/`.
  Keep it out of `sources/`, which is deleted along with its source, and give it
  a distinctive name — the book folder is also where PlainShelf writes. A file
  outside the shelf is the fully independent option.
- **Text that belongs to the book itself belongs in the source.** Content you
  edit in the source editor is stored verbatim.

PlainShelf does not promise to preserve unknown keys, and no build implements
passthrough today. Carrying a key through means writing back data this build
cannot validate, into the file the promises above rest on. A narrower scheme —
reserving a prefix such as `x_` for keys PlainShelf preserves untouched — is
still under consideration, but nothing is committed and nothing has been built.
Until that changes, assume any key outside the table is temporary.

---

## Back up before upgrading

Take a backup before every upgrade: until 1.0, an upgrade can write a format the
release you are on will not touch, and the backup is the only way back.

[Backup and Restore](../backup-and-restore.md) is the full procedure — what to
copy for each installation, what a shelf-only copy silently leaves behind, and
how to put it back. Two points matter specifically for an upgrade:

- **Take it immediately before the upgrade,** not on whatever schedule you
  otherwise keep. What you need is the state the previous release last wrote.
- **A copy of the shelf directory is not the whole picture.** Reading progress,
  history, and time are per-device and have never been in a server-side backup;
  the three stored server settings are in the application store, not the shelf.
  See [What a shelf-only backup
  loses](../backup-and-restore.md#what-a-shelf-only-backup-loses).

## v0.8 reading-data breaking change

!!! warning "v0.8 reading data does not carry into v0.9.0 or later"
    Starting with v0.9.0, PlainShelf keeps a new, empty reading history and
    reading-time record on each device. Web and desktop reading progress also
    starts at zero instead of importing v0.8's server-side bookmarks. Existing
    Android progress was already on-device and remains available. The old
    server-side history, dashboard activity, and bookmarks are not migrated or
    read by v0.9.0 or any later release. This is an intentional pre-1.0 breaking
    change.

PlainShelf provides no export, import, or recovery path for these values.
Upgrade from v0.8 only if you accept that they will no longer be accessible.

## Restoring from a backup

[Backup and Restore](../backup-and-restore.md#restoring) has the steps for the
server, Docker, and the macOS desktop app, along with what can be skipped and
what a restore cannot bring back. For going back to an older release
specifically, see [Rolling back to an older
release](../backup-and-restore.md#rolling-back-to-an-older-release): restoring
is required, because downgrading the binary alone leaves the newer on-disk data
in place and the older build refuses to write it.

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

A book in the trash behaves the same way, with `trash.json` in place of
`book.json`: it is still listed, and restoring or permanently deleting it
returns `409 Conflict` with

```text
trashed book uses a newer on-disk format than this PlainShelf build supports;
upgrade PlainShelf to modify it
```

---

## Fingerprint cache

`app/fingerprint-cache.json`, the cache behind the
[Similar Books](../finding-similar-books.md) page, also carries a
`schema_version` — but it is versioned the opposite way to everything above,
because it is rebuildable cache rather than your data.

The file records both its `schema_version` and an `algo` block naming the exact
rules its fingerprints were produced under (the text normalization, the shingle
size, the hash, and how many hashes each fingerprint keeps). When a build meets a
`schema_version` it does not read, **or** an `algo` block that does not match the
rules it fingerprints by, it throws the whole file away and rebuilds it. There is
no migration and, deliberately, no field-level salvage: changing how text is
normalized invalidates every fingerprint built on top of it, so keeping "the
parts that did not change" would mean trusting entries nobody can vouch for.

That is the right trade only because the file is a cache. Discarding it costs one
recomputation — the next similarity build reads the sources again — and never
loses anything, which is exactly what the migration promises above exist to avoid
for `books/` and `trash/`. See
[the `app/` directory](data-model.md#app) for what the file holds.

!!! note "Building fingerprints can write one source file"
    The Similar Books page reads like pure analysis, but building fingerprints
    has one write side effect on `books/`. When it reads a source whose stored
    `md5_hash` no longer matches the source's actual bytes — content changed
    outside PlainShelf, or an older import that recorded a different hash — it
    corrects that source's `meta.json` in place (`fingerprint_facade.go:42` →
    `source.go:245`, `RepairContentHash`). It deliberately leaves the UI-visible
    line and character counts at their stale values rather than recomputing them,
    but the write still rebuilds the whole file from the fields PlainShelf knows —
    so, like any `meta.json` write, it drops top-level keys outside that set. It
    goes through the same write refusal as any other write, so it never touches a
    read-only shelf or a source whose schema version is newer than this build.

---

## The exported book cache

`app/book-cache-{writer-id}.json` carries a `schema_version` too
(`shelf_cache_export.go:43`, `BookCacheSchemaVersion`), and like the fingerprint
cache it is versioned the throw-away way: there is no migration path, and a
reader that meets a version it does not recognize discards the whole file and
falls back to walking the shelf. What sets it apart from every cache above is
*which implementation* reads it. The scan and fingerprint caches are read only by
the same Go shelf that writes them — which is not the same as being private to one
machine: several installations sharing a shelf all run that one implementation, so
`scan-cache.json` is a single shared file and `fingerprint-cache.json` merges
every machine's entries, yet no reader outside the Go shelf ever parses either.
This file is different. It is read by a **separately written program** — the
TypeScript client on the phone — so its `schema_version` is a contract between two
codebases, and because that second reader runs on another device, a cross-device
one, rather than local scratch state a single implementation keeps to itself.

The server and desktop app export the file so that a client reading the shelf
*directly*, without a server in between, does not have to walk it book by book.
The Android app opening a shelf from pCloud is the client it exists for.
[Shelf cache and disk I/O](shelf-cache-and-io.md#the-exported-book-cache)
describes what the file holds and when it is written; this section adds only
what happens when its version does not match.

### A second reader, kept in step by a shared fixture

The phone shares no code with the server. The Android pCloud client
re-implements the shelf reader in TypeScript under `frontend/src/api/pcloud/` —
the `.bookpkg` layout, `book.json` parsing, and the exported-cache format are
all written a second time — because it reads the on-disk format directly and
cannot import the Go packages. So the schema versions it enforces are
hand-maintained constants standing beside their Go originals:
`BOOK_CACHE_SCHEMA_VERSION` (`bookCacheFile.ts:22`) mirrors
`BookCacheSchemaVersion`, and `BOOK_META_SCHEMA_VERSION` (`bookpkg.ts:55`)
mirrors `book.json`'s own version.

Two hand-written copies of a format drift apart unless something forces them
together. That something is a cross-language conformance suite:
`frontend/src/api/pcloud/conformance.test.ts` and its Go twin
`shelf/conformance_test.go` read the *same* dataset from
`shelf/testdata/conformance/`, so a format change only one side follows fails
the other side's test. The duplicated constants are not kept honest by review
alone — the fixture is what notices when the two readers stop agreeing.

### What a version mismatch costs the phone

`parseBookCacheFile` (`bookCacheFile.ts:117`) rejects any file whose
`schema_version` is not exactly the one it was built against. It does not read a
newer or older cache best-effort: a half-understood payload would surface as a
silently truncated library rather than the cache miss it really is, so the
reader treats any mismatch as no cache at all.

That refusal is safe but not free. When the phone cannot use the exported file
it falls back to walking the shelf, which over pCloud costs **two HTTP requests
per book** — one to get a download link for each `book.json`, one to fetch it —
in place of the single download the cache would have been. Nothing is wrong and
nothing is lost: every book still lists, and its metadata is still read straight
from `book.json`. But on a metered or high-latency connection the gap between
one download and two-requests-per-book is a real performance cliff, not a
rounding error — and it is the everyday shape of "the server upgraded, the phone
did not": a build that raises `BookCacheSchemaVersion` starts exporting a file
the older phone reader declines, until that phone is updated too.

This is the one sense in which "deleting the book cache is safe" needs a
qualifier. From the server's side it is exactly as safe as the caches above: the
next export rebuilds it, and [Data Model](data-model.md#app) says as much. From
the phone's side, deleting or invalidating it is the difference between one
download and a full per-book walk — still safe, still never an error, but no
longer free.

`book.json`'s own version is the other mismatch the phone can meet, and it is
answered differently: the book is read best-effort and listed as usual, because
this reader never writes and so has no write to refuse. What the server says by
refusing one, the phone therefore has to say out loud — a book whose
`schema_version` is newer than `BOOK_META_SCHEMA_VERSION` carries a notice on its
detail page saying the file was saved in a newer format and that some of its
details may be missing here. It is the same fact as the server's `409 Conflict`,
told to a reader who will never trip over the write that produces it.

---

## Shelf layout changes are not versioned

Every marker above versions the *contents of a file*. None of them versions the
*shape of the shelf around those files* — the names and nesting of the top-level
directories, the `.bookpkg` naming convention, or which directory a given file
lives in. That layer carries no version marker, and this section records the
decision not to give it one, along with the test for recognizing when a change
falls on this side of the line.

### Layout change vs file-format change

The two are worth separating because only one of them is versioned.

- A **file-format change** alters the bytes inside a versioned file —
  `book.json`, a source's `meta.json`, or `trash.json`. Adding, removing,
  renaming, or changing the meaning of a key is a file-format change. It shows up
  in a diff of that one file, and it is what `schema_version` exists to mark: an
  older build meets a version it does not understand and refuses to overwrite it.
- A **layout change** alters the shelf around those files without changing any
  file's contents: renaming a top-level directory (`.trash/` → `trash/`),
  introducing or removing one, or changing how book folders are named or nested.
  Read every versioned file in such a shelf and you cannot tell the change
  happened; only the directory tree did.

The test is: **would a diff of one versioned file reveal the change?** If yes, it
is a file-format change and the per-file `schema_version` covers it. If the
change is visible only in directory names, nesting, or whether a directory is
present, it is a layout change — nothing versions it, and it is handled as below.

### The decision: no shelf-level manifest

**PlainShelf does not introduce a shelf-level manifest** — no `app/shelf.json`
with a `layout_version`, and no equivalent elsewhere. A layout change is detected
by looking at what is on disk, and its cross-version consequences are
communicated in the changelog, not enforced by a version guard.

PlainShelf has made two layout changes so far, and neither used a manifest. The
earlier one — book folders changing extension from `.novl` to `.bookpkg` — was a
hard cut: no code detects `.novl` today, so an existing shelf was expected to be
renamed out-of-band rather than migrated at runtime. The later one, the
`.trash/` → `trash/` rename, is the model this policy follows:
`migrateLegacyTrash` (`shelf/trash.go`) decides entirely by whether `.trash/`
exists, consulting no marker and writing none, and it moves nothing
destructively. [What is versioned today](#what-is-versioned-today) records the
same fact in its table.

A manifest was rejected deliberately. It would be the first file under `app/`
that could not be thrown away and rebuilt, breaking the rule that [everything
under `app/` is disposable](data-model.md#app) — so it would have to live outside
`app/`, or carve out a documented exception with its own rebuild story, to hold a
single integer that changes very seldom. At that frequency the manifest costs
more than it saves.

#### `shelf.json` is not that manifest

A shelf may carry a [`shelf.json`](data-model.md#shelfjson) at its root, and it
is worth being explicit that this does not reverse the decision above. It holds
settings the user wrote — currently which directories the scanners skip — and
nothing else reads it as a statement about the shelf's shape:

- It is optional, and a shelf without one is not an older shelf. Absent means
  "the defaults", not "an earlier layout".
- No build writes it, so it can never disagree with what is on disk.
- Its own `schema_version` versions *its own contents*, exactly like
  `book.json`'s — it is a file-format marker for one file, not a layout marker
  for the shelf. Renaming a top-level directory would still be detected by
  looking for that directory, not by reading a number here.

If a layout version is ever wanted, it still has to be argued for on the terms
above; the presence of a settings file at the shelf root is not that argument.

### What this costs

Presence detection instead of a manifest is not free, and the cost is stated
plainly here so the next layout change is made with eyes open:

- **A shelf's layout generation is not readable from its data.** You cannot open
  a shelf and learn which layout it is on; you infer it from which directories
  are present. There is no layout equivalent of the `schema_version` you can read
  off a file.
- **Each layout change grows its own bespoke detection.** Like `.trash/`, every
  future one adds a one-off condition keyed on some directory existing, and none
  is self-describing. That tax rises with the number of such changes.
- **There is no write-refusal guard for the layout.** The per-file guard that
  stops an older build from clobbering a newer `book.json` has no layout
  analogue: an older build meeting a newer directory shape has no version to
  refuse on. Downgrade and cross-version behavior can therefore only be
  *communicated* — through a `Breaking (pre-1.0)` changelog entry — never
  *enforced*.

What makes the trade acceptable today: layout changes are rare — two in the
project's history (`.novl` → `.bookpkg`, then `.trash/` → `trash/`) — and each is
a one-way startup migration that must be idempotent and destroy nothing. Going
forward this policy requires every layout change to carry a `Breaking (pre-1.0)`
changelog entry describing its cross-version effect, as the `.trash/` → `trash/`
rename does. If that frequency ever rises enough that the bespoke conditions
become a burden, revisit this decision — an `app/`-external manifest is the escape
hatch — but that is a future call made on evidence, and taking it would not
retrofit any existing shelf.

### This changes no existing behavior

This is a written policy, not a code change. The `.trash/` → `trash/` presence
detection keeps working exactly as it does now; no existing shelf's migration
path moves, and no shelf gains or needs a layout marker. A shelf with no such
marker is not a shelf waiting to be upgraded — it is the only kind there is.

---

## What is versioned today

Book, source, and trash metadata carry independent schema versions, and so do
the caches under `app/`. What differs is what a version is *for*, which sorts the
versioned files into three kinds:

- **User data** — `book.json`, source `meta.json`, `trash.json`. Losing them
  loses your library, so the version is a guard: a build that meets a newer
  version refuses to overwrite it, and upgrades are lazy and per-book. This is
  the versioning the rest of this page describes.
- **Local disposable cache** — `app/scan-cache.json` and
  `app/fingerprint-cache.json`. Rebuildable from the shelf and read only by the
  same Go implementation that writes them — shared across the installations that
  mount one shelf (`scan-cache.json` is one file; `fingerprint-cache.json` merges
  every machine's entries), but never parsed by any reader outside that
  implementation — so their version carries no migration: any mismatch throws the
  whole file away and it is recomputed.
- **Cross-device contract** — `app/book-cache-{writer-id}.json`. Thrown away on
  a mismatch like the caches above, but read by a *different* program on another
  device — the Android pCloud client — so its version is a handshake between two
  codebases, and a mismatch has a real cost on the phone even though it is never
  an error. See [The exported book cache](#the-exported-book-cache).

| Path | Versioned? | Kind |
|---|---|---|
| `books/**/book.json` | Yes — `schema_version`, described on this page | User data |
| `books/**/sources/{id}/meta.json` | Yes — `schema_version`; schema v1 owns source `format` | User data |
| `trash/**/trash.json` | Yes — `schema_version`; schema v2 owns the book's restore path | User data |
| `app/scan-cache.json` | Yes — `schema_version` (`scancache/scancache.go:53`); no migration, discarded and rebuilt on any mismatch, unlike `books/` and `trash/` which are upgraded in place | Local cache |
| `app/fingerprint-cache.json` | Yes — `schema_version` and an `algo` block, but discarded and rebuilt on any mismatch, never migrated (it is a cache) | Local cache |
| `app/book-cache-{writer-id}.json` | Yes — `schema_version` (`shelf_cache_export.go:43`); no migration, discarded and rebuilt on any mismatch, but read across devices — see [The exported book cache](#the-exported-book-cache) | Cross-device contract |
| `shelf.json` | Yes — `schema_version` versions this file's own contents; optional, never written by PlainShelf, and a higher version is still read for the fields this build knows. It carries no statement about the shelf's layout — see [`shelf.json` is not that manifest](#shelfjson-is-not-that-manifest) | User settings |
| `books/` directory layout | No — the folder tree and the `.bookpkg` folder naming carry no version marker. An older layout is handled by detecting the old path at startup and moving it (`shelf/trash.go`'s `migrateLegacyTrash`, `.trash/` → `trash/`), not by a layout schema version. This is a decision, not an oversight — see [Shelf layout changes are not versioned](#shelf-layout-changes-are-not-versioned) | — |
| Application store | No | — |

The practical rule remains: **run one PlainShelf version against a shelf at a
time.** Unversioned files and optional fields still cannot make mixed-version
writes safe in general.
