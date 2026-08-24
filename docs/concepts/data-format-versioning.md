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
  preserves the version it found rather than stamping the current one — so the
  only way to upgrade legacy sources is the explicit
  [`cmd/migrate-legacy-sources`](#migrating-legacy-sources-in-place) pass.
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
  fields. [Hand-editing `book.json`](#hand-editing-bookjson) below says which
  edits do survive.
- Reading a book whose `schema_version` is higher than the build supports is
  best-effort. Fields may be missing or misinterpreted, and the displayed
  metadata may be wrong. It is shown so you can see the book exists, not so you
  can rely on it.
- There is no downgrade path. PlainShelf will not rewrite a schema v2 book back
  to schema v1.
  To go back to an older release, restore from a backup taken before the
  upgrade.
- Files other than `book.json`, source `meta.json`, and `trash.json` are not
  versioned yet — see
  [What is versioned today](#what-is-versioned-today). They get the same
  treatment as they are covered, not retroactively.
- Hand-edited `book.json` files are read on a best-effort basis. Malformed JSON
  makes that book unopenable; the error is logged with the file path.

### Hand-editing `book.json`

`book.json` is a plain file and nothing stops you from opening it in an editor.
What decides whether an edit lasts is whether PlainShelf knows the field.

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

The shelf is plain files, so a backup is a copy of a directory:

```sh
cp -a /path/to/shelf /path/to/backup/shelf-2026-07-28
# or
rsync -a /path/to/shelf/ /path/to/backup/shelf-2026-07-28/
```

Those two are equivalent for this purpose. Committing the shelf to Git is not —
see [Git does not back up empty folders](#git-does-not-back-up-empty-folders)
below.

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

### Git does not back up empty folders

Git tracks files, not directories, so a [folder](folders.md) that holds no book —
one you created ahead of time, or one whose books you have since moved out — is
not in the commit and is not there after a checkout. Books themselves are
directories full of files and come back intact; what a Git backup loses is the
shape of the shelf around them.

PlainShelf hits the same property internally, which is why it records the folder
list in its own right instead of deriving it from the books: an empty folder
holds no book, so nothing can rebuild it from the library.

Two ways to live with that:

- **Accept it,** and re-create the empty folders by hand after a restore. They
  are only directories, so `mkdir` under `books/` or the app's own folder
  creation is enough. [Restoring from a backup](#restoring-from-a-backup) below
  says how to spot which ones are gone.
- **Put a `.gitkeep` (or any placeholder file) in each folder you want
  preserved.** Git then has a file to track and keeps the directory.
  PlainShelf's scanners look only at directories under `books/`, so such a file
  never shows up as a folder or a book — but it is still a file you did not put
  in your library, and while it is there PlainShelf refuses to delete that
  folder, reporting `cannot delete non-empty folder`; remove the file first.
  PlainShelf neither creates nor removes these files, and does not plan to:
  `books/` holds your files, not the app's.

Use `cp -a` or `rsync` when you want a backup with none of these caveats.

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

1. **Stop the server or desktop app.** The shelf lock is not a substitute for
   stopping the process.
2. Restore the shelf directory, and the application store if you backed it up.
3. Start PlainShelf again.

You can skip `app/library.lock`, `app/tmp/`, and `app/book-cache-*.json` when
restoring; they are recreated on the next startup. Restore `books/` and `trash/`
in full: both hold books, and a restored `.trash/` from an older backup is
renamed to `trash/` on the next start.

If you restored from a Git checkout rather than a file copy, check the folder
tree before you start filing books again: every folder that was empty at commit
time is missing, and no error says so — the library simply comes back one or
more folders shallower. Compare the folder list in the app (or `find books/ -type d`)
with what you expect, and re-create the ones that are gone. No book can go
missing this way: a folder that still holds a book holds files, so Git kept it.

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

The one layout change so far, the `.trash/` → `trash/` rename, is the model:
`migrateLegacyTrash` (`shelf/trash.go`) decides entirely by whether `.trash/`
exists, consulting no marker and writing none. [What is versioned
today](#what-is-versioned-today) records the same fact in its table.

A manifest was rejected deliberately. It would be the first file under `app/`
that could not be thrown away and rebuilt, breaking the rule that [everything
under `app/` is disposable](data-model.md#app) — so it would have to live outside
`app/`, or carve out a documented exception with its own rebuild story, to hold a
single integer that changes roughly once a year. At that frequency the manifest
costs more than it saves.

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

What makes the trade acceptable today: layout changes are rare (the tally is one,
`.trash/`), each is a one-way startup migration that must be idempotent and
destroy nothing, and each is announced in the changelog. If that frequency ever
rises enough that the bespoke conditions become a burden, revisit this decision —
an `app/`-external manifest is the escape hatch — but that is a future call made
on evidence, and taking it would not retrofit any existing shelf.

### This changes no existing behavior

This is a written policy, not a code change. The `.trash/` → `trash/` presence
detection keeps working exactly as it does now; no existing shelf's migration
path moves, and no shelf gains or needs a layout marker. A shelf with no such
marker is not a shelf waiting to be upgraded — it is the only kind there is.

---

## What is versioned today

Book, source, and trash metadata carry independent schema versions.

| Path | Versioned? |
|---|---|
| `books/**/book.json` | Yes — `schema_version`, described on this page |
| `books/**/sources/{id}/meta.json` | Yes — `schema_version`; schema v1 owns source `format` |
| `trash/**/trash.json` | Yes — `schema_version`; schema v2 owns the book's restore path |
| `app/fingerprint-cache.json` | Yes — `schema_version` and an `algo` block, but discarded and rebuilt on any mismatch, never migrated (it is a cache) |
| `books/` directory layout | No — the folder tree and the `.bookpkg` folder naming carry no version marker. An older layout is handled by detecting the old path at startup and moving it (`shelf/trash.go`'s `migrateLegacyTrash`, `.trash/` → `trash/`), not by a layout schema version. This is a decision, not an oversight — see [Shelf layout changes are not versioned](#shelf-layout-changes-are-not-versioned) |
| Application store | No |

The practical rule remains: **run one PlainShelf version against a shelf at a
time.** Unversioned files and optional fields still cannot make mixed-version
writes safe in general.
