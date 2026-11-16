# Data format versioning

This page explains how PlainShelf marks the on-disk format of your library, what
happens to your files when you upgrade, and what PlainShelf does when it meets
data it cannot safely write.

For what is stored where, see [Data model](data-model.md); for how to copy your
library and put it back, see [Backup and restore](../backup-and-restore.md).

---

## Why the shelf has a schema version

A shelf created today may be opened years from now by a much later PlainShelf
release, or by a much older one on a machine you forgot to update.

The version marker in each file lets a build recognize data it does not fully
understand and refuse to overwrite it, instead of silently rewriting the file
into a shape that loses information.

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
| `format` | Compatibility mirror of the current source format (`txt`, `md` or `images`) |
| `tags` | Free-form tags |
| `identifiers` | External identifiers such as `isbn` |
| `cover` | Cover filename relative to the book folder |
| `authors` | Author names |
| `language` | BCP-47 language tag |
| `comments` | Free-form notes |
| `star` | Rating, 0–5 |
| `series` | The series this book belongs to, as `{"name": "Saga", "index": 2}`. Optional and omitted when the book is in no series. `name` must not be blank; `index` is the book's place in the series, a non-negative number that may be fractional, so `1.5` sorts a side story between volumes 1 and 2. A book belongs to at most one series. Written by the metadata editor's **Series** fields, or by hand |
| `nsfw` | Marks this one book as adult content. Optional and omitted when `false`, so most `book.json` files do not carry it. Written by the metadata editor's **Adult content** switch, or by hand. It can only add: a shelf may also mark a whole folder in [`shelf.json`](data-model.md#shelfjson), and `false` here does not take a book out of a marked folder. See [Marking a folder as adult content](folders.md#marking-a-folder-as-adult-content) |
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

Which means:

- A shelf can sit half-upgraded indefinitely. Some books carry
  `schema_version`, others do not, and that is a perfectly normal state.
- File modification times change one book at a time, as you edit them, rather
  than all at once after an upgrade. Backup tools see a trickle, not a
  library-wide rewrite.

---

## Source metadata schema v1

New `sources/{id}/meta.json` files also carry `schema_version: 1` and an
authoritative `format` (`txt`, `md` or `images`). A source without those fields is a
legacy source. It is still listed and still readable, but nothing interprets its
`split_config` any more: it renders as `book.json`'s `format` says, which means
one plain-text section unless the book is Markdown. To give it chapters again,
convert it in the source editor (see below).

An `images` source may also carry `page_direction`, written only as `"rtl"` or
`"ttb"`; absent means left to right. `images` and `page_direction` are part of schema v1
as `1.0-rc1` freezes it, not later additions. A build from before them reads an
`images` source's page list as plain text, and refuses any metadata edit to a
book whose `book.json` mirrors `format: "images"`, because it validates `format`
on every write. See [Image sources](data-model.md#image-sources).

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

To give a plain-text legacy source chapters, open it in the source editor and
run a **TXT → Markdown** conversion, either by a regular expression or by a
fixed line count. The conversion writes the chapter boundaries into the text as
`## ` headings and saves them as a **new** schema v1 source, leaving your legacy
source untouched, so nothing about the original is lost. Making that new source
the book's current source also switches `book.json`'s compatibility `format` to
`md`; pointing the book back at the legacy source restores its text but not that
format mirror, so correct the book's `format` back to `txt` as well if you want
it read as plain text again. This per-source conversion is the only supported
way to re-chapter a legacy source. There is no batch or in-place upgrade, and a
one-off migration tool that once did this shelf-wide has been removed.

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
carries a `trash.json` from a newer build, which happens when a book was carried
back out of the trash by hand, moving it to the trash is refused before the book
is moved.

---

## `folder.json` schema v1

A folder may carry its own metadata in a `folder.json` inside the folder
directory, for example `books/Fiction/folder.json`. Because the file lives in
the directory it describes, it moves with the folder: a rename or move in
PlainShelf, or an `mv` outside it, carries it along with no list to update. A
copy or move to another shelf copies the file unchanged.

```json
{
  "icon": "rocket",
  "schema_version": 1
}
```

| Field | Meaning |
|---|---|
| `schema_version` | On-disk format version of this file. Managed by PlainShelf; a file without it reads as v1. |
| `icon` | The folder's icon: one name from the list below. Absent means the default folder icon. |
| `sort_key` | Orders a folder among its siblings: a finite number, lower first, before siblings without a key. Zero and negative values are valid. Missing, null, and wrong-typed values mean unspecified. Equal or absent keys retain each interface’s existing name/path fallback; reading never rewrites the file. |

The icon names are a subset of [Tabler](https://tabler.io/icons) icon names:
`alien`, `atom`, `bolt`, `book`, `bookmark`, `books`, `briefcase`, `camera`,
`code`, `coffee`, `compass`, `crown`, `device-gamepad-2`, `diamond`, `flame`,
`flask`, `flower`, `folder`, `ghost`, `heart`, `leaf`, `map`, `mood-smile`,
`moon`, `movie`, `music`, `notebook`, `palette`, `paw`, `planet`, `puzzle`,
`robot`, `rocket`, `school`, `shield`, `skull`, `star`, `sun`, `sword`, `world`.
A later release may add names. A name this build does not offer is shown as the
default icon and kept as written.

The shelf root, `books/` itself, has no metadata: a `books/folder.json` is not
read.

**Reading never hides a folder.** A missing file, malformed JSON, a top-level
value that is not an object, a member of the wrong type, an icon this build
does not offer, or a `schema_version` other than 1 all read as "no metadata"
for that folder, and the problem is logged once rather than on every listing.
Reading never rewrites the file.

**Writing changes only what it is asked to.** Unlike the three files above,
`folder.json` is written member by member: setting or clearing the icon replaces
that one member and keeps every other one (`sort_key`, a member a newer build
added, a member you added by hand) with its value and nested structure intact.
Formatting is not kept: the file is rewritten with two-space indentation and
members in alphabetical order. Clearing the icon of a file that then holds
nothing but `schema_version` deletes the file, so the folder is left as it was
before it had any metadata.

A write is refused, and the file left exactly as it is, when this build cannot
read the file safely: malformed JSON, not an object, or a `schema_version`
other than 1. It is also refused when the file changed after the caller read
it, whether by hand or by another PlainShelf sharing the shelf, so an edit made
in between is never overwritten. That check covers writers that take the shelf
lock; a program that does not cannot be fenced off, and PlainShelf does not
promise a transaction across machines.

Because the file is named `folder.json`, no folder may be given that name, in
any letter case. A directory of that name made outside PlainShelf is still
listed as a folder and can be renamed or deleted, but its parent's metadata
reads as none and refuses writes until it is renamed.

---

## Compatibility policy

What the format promises before and after the `1.0.0-rc1` freeze, and what
hand-editing `book.json` can and cannot rely on, is on its own page:
[Compatibility policy](compatibility-policy.md).

---

## Back up before upgrading

Take a backup before every upgrade: until 1.0, an upgrade can write a format the
release you are on will not touch, and the backup is the only way back.

[Backup and restore](../backup-and-restore.md) is the full procedure — what to
copy for each installation, what a shelf-only copy silently leaves behind, and
how to put it back. Two points matter specifically for an upgrade:

- **Take it immediately before the upgrade,** not on whatever schedule you
  otherwise keep. What you need is the state the previous release last wrote.
- **A copy of the shelf directory is not the whole picture.** Reading progress,
  history, and time are per-device and have never been in a server-side backup;
  the three stored server settings are in the application store, not the shelf.
  See [What a shelf-only backup
  loses](../backup-and-restore.md#what-a-shelf-only-backup-loses).

## Restoring from a backup

[Backup and restore](../backup-and-restore.md#restoring) has the steps for the
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
  file's contents: renaming a top-level directory (`.trash/` to `trash/`),
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
communicated in the release notes, not enforced by a version guard.

PlainShelf has made two layout changes so far, and neither used a manifest. The
earlier one, book folders changing extension from `.novl` to `.bookpkg`, was a
hard cut: no code detects `.novl` today, so an existing shelf was expected to be
renamed out-of-band rather than migrated at runtime. The later one, renaming
`.trash/` to `trash/`, is the model this policy follows: `migrateLegacyTrash`
(`shelf/trash.go`) decides entirely by whether `.trash/` exists, consulting no
marker and writing none, and it moves nothing destructively. [What is versioned
today](#what-is-versioned-today) records the same fact in its table.

A manifest was rejected deliberately. It would be the first file under `app/`
that could not be thrown away and rebuilt, breaking the rule that [everything
under `app/` is disposable](data-model.md#app) — so it would have to live outside
`app/`, or carve out a documented exception with its own rebuild story, to hold a
single integer that changes very seldom. At that frequency the manifest costs
more than it saves.

#### `shelf.json` is not that manifest

A shelf may carry a [`shelf.json`](data-model.md#shelfjson) at its root, and it
does not reverse the decision above. It holds settings the user wrote (currently
which directories the scanners skip), and nothing else reads it as a statement
about the shelf's shape:

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

Presence detection instead of a manifest is not free:

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
  *communicated*, through a pull request labelled `breaking`, never *enforced*.

What makes the trade acceptable today: layout changes are rare. There have been
two in the project's history (`.novl` to `.bookpkg`, then `.trash/` to
`trash/`), and each is a one-way startup migration that must be idempotent and
destroy nothing. Going forward this policy requires every layout change to be a
pull request labelled `breaking` whose title states its cross-version effect, as
the `Breaking (pre-1.0)` entry for renaming `.trash/` to `trash/` did. If that
frequency ever rises enough that the bespoke conditions become a burden, revisit
this decision (an `app/`-external manifest is the escape hatch), but that is a
future call made on evidence, and taking it would not retrofit any existing
shelf. A shelf carrying no layout marker is not one waiting to be upgraded; it
is the only kind there is.

---

## What is versioned today

Book, source, trash, and folder metadata carry independent schema versions, and so do
the caches under `app/`. What differs is what a version is *for*, which sorts the
versioned files into three kinds:

- **User data** — `book.json`, source `meta.json`, `trash.json`,
  `folder.json`. Losing them loses your library, so the version is a guard: a
  build that meets a newer version refuses to overwrite it, and upgrades are
  lazy and per-book. This is the versioning the rest of this page describes.
- **Local disposable cache:** `app/scan-cache.json`,
  `app/fingerprint-cache.json` and `app/page-hash-cache.json`. They are rebuildable from the shelf and read
  only by the same Go implementation that writes them. They are shared across
  the installations that mount one shelf (`scan-cache.json` is one file;
  `fingerprint-cache.json` merges every machine's entries), but no reader
  outside that implementation parses them, so their version carries no
  migration: any mismatch throws the whole file away and it is recomputed.
- **Cross-device contract:** `app/book-cache-{writer-id}.json`. It is thrown
  away on a mismatch like the caches above, but read on another device (the
  Android app's pCloud shelf, possibly built from an older release), so a
  mismatch has a real cost there even though it is never an error. See [The
  exported book cache](exported-book-cache.md#when-its-version-does-not-match).

| Path | Versioned? | Kind |
|---|---|---|
| `books/**/book.json` | Yes — `schema_version`, described on this page | User data |
| `books/**/sources/{id}/meta.json` | Yes — `schema_version`; schema v1 owns source `format` | User data |
| `trash/**/trash.json` | Yes — `schema_version`; schema v2 owns the book's restore path | User data |
| `books/**/folder.json` | Yes — `schema_version`; written member by member, so unknown members survive a write — see [`folder.json` schema v1](#folderjson-schema-v1) | User data |
| `app/scan-cache.json` | Yes — `schema_version` (`schemaVersion` in `shelf/scancache/scancache.go`); v2 added the kept files such as `folder.json`, so a v1 snapshot is not reused; no migration, discarded and rebuilt on any mismatch, unlike `books/` and `trash/` which are upgraded in place | Local cache |
| `app/fingerprint-cache.json` | Yes — `schema_version` and an `algo` block, but discarded and rebuilt on any mismatch, never migrated (it is a cache) | Local cache |
| `app/page-hash-cache.json` | Yes — `schema_version` and a `hash` name, discarded and rebuilt on any mismatch, never migrated — see [Page hash cache](similarity-fingerprints.md#page-hash-cache) | Local cache |
| `app/book-cache-{writer-id}.json` | Yes — `schema_version` (`BookCacheSchemaVersion`); v2 renamed `layers` to `folders`; no migration, discarded and rebuilt on any mismatch, but read across devices — see [The exported book cache](exported-book-cache.md#when-its-version-does-not-match) | Cross-device contract |
| `shelf.json` | Yes — `schema_version` versions this file's own contents; optional, never written by PlainShelf, and a higher version is still read for the fields this build knows. It carries no statement about the shelf's layout — see [`shelf.json` is not that manifest](#shelfjson-is-not-that-manifest) | User settings |
| `books/` directory layout | No — the folder tree and the `.bookpkg` folder naming carry no version marker. An older layout is handled by detecting the old path at startup and moving it (`shelf/trash.go`'s `migrateLegacyTrash`, `.trash/` → `trash/`), not by a layout schema version. This is a decision, not an oversight — see [Shelf layout changes are not versioned](#shelf-layout-changes-are-not-versioned) | — |
| Application store | No | — |

### Running two versions against one shelf

The practical rule remains: **run one PlainShelf version against a shelf at a
time.** The freeze does not change it, because the two answer different
directions.

- **A newer build reading an older shelf** is what the freeze protects. Every
  field the older build wrote is still read the same way, and every field the
  newer build adds is optional — see
  [What backward compatible means](compatibility-policy.md#what-backward-compatible-means).
- **An older build reading a shelf a newer one writes** is what mixed-version
  use actually runs into, and the freeze says nothing about it. Two things can
  happen. A file whose `schema_version` the older build does not support is
  read best-effort and refused for writing (`shelf/bookpkg/book.go`'s
  `EnsureWritable`, and its counterparts for sources and trash), so the data is
  safe but the older build cannot edit it. A file at a version the older build
  *does* accept, carrying a field only the newer one knows, hits no guard at
  all: the field is simply dropped the next time the older build writes that
  book, because unknown keys are not carried over — see
  [Unknown fields are not covered](compatibility-policy.md#unknown-fields-are-not-covered).

So "backward compatible from `1.0.0-rc1` on" does not read as "mixed versions are
now safe". Unversioned files and optional fields still cannot make mixed-version
*writes* safe in general. Upgrade every build that writes to a shelf together,
or accept that the older one loses what the newer one added.
