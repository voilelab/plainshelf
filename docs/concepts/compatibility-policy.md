# Compatibility policy

The on-disk format freezes at **`1.0.0-rc1`**, not at `1.0.0`. The first release
candidate is the point from which the shelf's user-data files stop taking
breaking changes; every release after it (a further `1.0.0-rc2` should one be
needed, the stable `1.0.0`, and the rest of 1.x) inherits those commitments
rather than starting them. What a shelf can rely on therefore depends on which
side of that tag it runs on, and the two are described separately below: the 0.x
series is still unstable, and everything from `1.0.0-rc1` on is covered.

## Before the freeze — the 0.x series

During the v0.x series the on-disk format may still change in breaking ways
between releases. Such changes are announced in the release notes, under
*Breaking changes*, by a pull request labelled `breaking`. That marker is
retired for the on-disk format at `1.0.0-rc1`: from the freeze on there is no such change left to
announce, so a new one appearing would be a bug rather than a documented break.
Concretely, for a shelf you are running on a 0.x build today:

- **Reading is not promised to survive a minor upgrade.** Moving from, say, 0.9
  to 0.10 may change how the format is read. Nothing here commits a later 0.x
  build to reading an earlier one's shelf unchanged.
- **No data migration is promised.** PlainShelf does not undertake to carry 0.x
  data forward across a breaking change. Where it drops data it says so in the
  release notes.
- **The refusal to write a newer format already protects you.** This is the one
  guarantee that holds today rather than at the freeze: PlainShelf will not
  write a `book.json`, source `meta.json`, or `trash.json` whose on-disk
  `schema_version` is higher than the running build understands
  (`bookpkg.Book.EnsureWritable`, `bookpkg.Source.EnsureWritable`, and `shelf`'s
  `trashMetaWritable`). Such an object stays readable on a best-effort basis,
  and every attempt to modify it fails with an explicit error instead of
  overwriting the file. So an older build cannot silently rewrite an object
  whose `schema_version` a newer build actually raised, because its guard
  refuses it (see [When PlainShelf refuses to
  write](data-format-versioning.md#when-plainshelf-refuses-to-write)). It does **not** cover a field a
  newer build added *without* raising the version: that object still reads as a
  version the older build accepts, so the guard does not fire and the field is
  dropped on the older build's next write. See [What we do not
  promise](#what-we-do-not-promise).

Beyond that write refusal, treat the 0.x on-disk format as unstable: keep a
backup before each upgrade, and do not rely on the frozen commitments below.

## From PlainShelf 1.0.0-rc1 on

**These commitments take effect with the `1.0.0-rc1` tag** and hold for every
release after it — any further release candidate, the stable `1.0.0`, and the
rest of the 1.x line. From that tag on, for any shelf whose books are at
`book.json` schema v1, PlainShelf makes the commitments below.

### What the freeze covers

Four files, the shelf's user data:

- `books/**/book.json`
- `books/**/sources/{id}/meta.json`
- `trash/**/trash.json`
- `books/**/folder.json`

It covers the **on-disk format of those four files** and nothing else. Three
exclusions are deliberate:

- **The HTTP API and the user interface**, which may still change. The freeze is
  a promise about your files, not about the application around them.
- **The derived caches under `app/`:** `scan-cache.json`,
  `fingerprint-cache.json`, `page-hash-cache.json`, and
  `book-cache-{writer-id}.json`. Their
  `schema_version` is the opposite kind: no migration, discarded and rebuilt on
  any mismatch. Nothing of yours is lost when one is thrown away, so nothing
  about them is frozen. See [What is versioned today](data-format-versioning.md#what-is-versioned-today).
- **Per-device reading state**, which is not shelf data at all: the desktop and
  standalone reader's `reading_progress.json`, `read_history.json` and
  `reading_stats.json` in the application store, the
  browser's `localStorage`, and the Android client's WebView storage and
  app-private files.
  [Backup and restore](../backup-and-restore.md#reading-progress-history-and-time)
  says where each one lives.

Releases before the freeze are not covered either: data a 0.x release dropped
in a documented breaking change is not data the freeze undertakes to migrate.

### What backward compatible means

Every later change to those four files is backward compatible, which here is a
narrow and checkable statement rather than a general intention:

- **A new field is always an optional addition.** It may be absent, and a build
  that does not know it still reads the file correctly. No existing field
  changes meaning or type, none is removed, and none becomes required.
  `book.json`'s [`nsfw`](data-format-versioning.md#bookjson-schema-v1) is the first field added under this
  rule: it is written only when `true`, so a shelf that marks nothing carries the
  same bytes it carried at the freeze, and a build predating it reads every other
  field exactly as before.
- **`schema_version` is not raised again.** Raising it is exactly what a
  breaking change would need, and there is no longer such a change to make, so
  `book.json` stays at schema v1, source `meta.json` at schema v1,
  `trash.json` at schema v2, and `folder.json` at schema v1 for the life of the
  promise. A build that meets a higher version on disk is meeting a file
  PlainShelf did not write.

Both halves point one way: a **newer build reads an older shelf**. That is the
direction the freeze protects, and it is not the same as making mixed-version
use safe — see
[Running two versions against one shelf](data-format-versioning.md#running-two-versions-against-one-shelf).

### Unknown fields are not covered

`book.json`, source `meta.json` and `trash.json` do not pass unknown fields
through; `folder.json` does, and is described [on its
own](data-format-versioning.md#folderjson-schema-v1). PlainShelf reads each of the other three into a
fixed set of fields (`book.json` into [its schema table](data-format-versioning.md#bookjson-schema-v1),
source `meta.json` and `trash.json` into their own) and rewrites the whole file
from those fields, so a top-level key it does not recognize is gone the next
time it writes that file: a key you added by hand and a key a newer build wrote
alike. What counts as "next time" differs per file:

- `book.json` is rewritten by any change to the book: its metadata, its cover,
  its current source. This is where `nsfw` is lost: a build that predates the
  field drops it the first time it writes that book, and the book stops being
  marked. That is accepted rather than worked around, since the alternative
  would be passthrough, which none of these files does. It is one more reason to
  mark a whole folder in [`shelf.json`](data-model.md#shelfjson), which no build
  rewrites at all. `series` is lost the same way, with no such fallback.
- Source `meta.json` is rewritten by any change to that source: editing its
  comment, replacing its content, or a background hash repair.
  [`split_config`](data-format-versioning.md#giving-a-legacy-source-chapters-again) is the worked
  example — a field PlainShelf itself once wrote, now dropped like any other
  key it does not know.
- `trash.json` is written once, when the book is moved to the trash, and never
  by listing or restoring it. A key added to an existing record therefore
  survives until that book is trashed again.

This is the commitment most easily read as its opposite. The freeze says
PlainShelf will not break the fields *it* defines; it does not say a field of
your own survives. None of those three files is a place to keep data PlainShelf
does not know about. Passthrough for such fields is itself an optional addition
(`folder.json` is written that way from its first release), so nothing here
rules it out for the others later; but until a file gains it, assume it holds
only the fields this page documents.

### What we promise

- A shelf whose books are at schema v1 stays readable by every later PlainShelf
  release, from `1.0.0-rc1` through the 1.x line. Read support for schema v1 is
  not removed.
- The schema version is not raised again. Only a change that an older build
  cannot read correctly would raise it (a field changing meaning or type, a
  field being removed, or a new field becoming required), and the freeze rules
  all three out. Additive changes never raised it in the first place.
- Upgrades are lazy and per-book. Opening a library never rewrites a book; a
  book is written in the new format only when you next change something about
  that book. Source `meta.json` is the one exception to the *upgrade* half: it
  is never raised as a side effect of an ordinary write. A legacy source keeps
  its unversioned metadata even when something else about it is written, because
  the write preserves the version it found rather than stamping the current one.
  A legacy source is therefore never upgraded in place; to get a schema v1
  source with chapters, convert it in the source editor. See [Giving a legacy
  source chapters again](data-format-versioning.md#giving-a-legacy-source-chapters-again).
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

### What we do not promise

- **Top-level keys PlainShelf does not recognize are removed the next time it
  writes that file**, in `book.json`, source `meta.json` and `trash.json` alike;
  see [Unknown fields are not covered](#unknown-fields-are-not-covered) above.
  That is why editing such a book from an older build loses the values only a
  newer one knows about: run one version against a shelf, or upgrade both. A
  read-only reader, such as the Android client on a pCloud shelf, is exempt from
  the losing half of this, since it never rewrites a book, but it can still be
  built against an older schema than the shelf and show stale or missing fields
  (see [What a version mismatch costs a remote
  shelf](exported-book-cache.md#what-a-version-mismatch-costs-a-remote-shelf)). [Hand-editing
  `book.json`](#hand-editing-bookjson) below says which edits do survive.
- Reading a book whose `schema_version` is higher than the build supports is
  best-effort. Fields may be missing or misinterpreted, and the displayed
  metadata may be wrong. It is shown so you can see the book exists, not so you
  can rely on it.
- There is no downgrade path. PlainShelf will not rewrite a schema v2 book back
  to schema v1.
  To go back to an older release, restore from a backup taken before the
  upgrade — see [Rolling back to an older
  release](../backup-and-restore.md#rolling-back-to-an-older-release).
- Nothing outside the four files listed in
  [What the freeze covers](#what-the-freeze-covers) is promised — not the caches
  under `app/`, and not per-device reading state. A file that gains user-data
  versioning later gets these promises from then on, not retroactively.
- Hand-edited `book.json` files are read on a best-effort basis. Malformed JSON,
  which includes a duplicated key and invalid UTF-8 as well as a missing brace,
  makes that book unopenable; the error names the file and the key at fault. See
  [How the file is parsed](#how-the-file-is-parsed).

## Hand-editing `book.json`

`book.json` is a plain file and nothing stops you from opening it in an editor.
Two things decide what happens to your edit: whether the file still parses, and
whether PlainShelf knows the field.

### How the file is parsed

PlainShelf reads its metadata files strictly, and says so rather than guessing:

- **Field names are matched exactly, including case.** `"Title"` is not
  `"title"` — it is a key PlainShelf does not know, so the title you typed is
  not read, and the key itself is dropped by the next write to that book (see
  below). The same goes for `"Star"`, `"AUTHORS"`, and every other variant. There
  is no fuzzy matching and no auto-correction: check the spelling against the
  [schema table](data-format-versioning.md#bookjson-schema-v1).
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
names the file and the key at fault (`books/Dune.bookpkg/book.json: duplicate
object member name "title"`) in the log, and in the message PlainShelf answers
with when you act on that book.

The same rules apply to a source's `meta.json`, to `trash.json`, and to
[`shelf.json`](folders.md) — except that a `shelf.json` PlainShelf cannot read
is reported and then ignored, leaving the built-in defaults in place, rather than
stopping anything. The caches under `app/` are the exception in the other
direction: they are rebuildable, so an unreadable one is silently discarded and
recomputed.

### What survives a write

**Any top-level key outside the schema is gone after the next write to that
book.** Add `"shelf_mark": "B-12"` or `"douban_id": "1770782"` next to
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

- **Changing the value of any field in the [schema table](data-format-versioning.md#bookjson-schema-v1),
  within the type and range that table gives**: title, authors, tags,
  identifiers, language, comments, star, series, published date. A value that fits is
  read back exactly as written. A value that does not is worse than a typo,
  because PlainShelf validates on write rather than on read: `"star": 6`, a
  `language` that is not a BCP-47 tag, a blank key in `identifiers`, or a
  `series` with a blank name or a negative index all load
  fine, and then every later edit to that book is refused until you repair the
  file by hand. A `published_at` that is neither `YYYY-MM-DD` nor an RFC 3339
  timestamp is worse still: it fails at read time and makes the book unopenable,
  exactly like malformed JSON. `schema_version` is its own exception: it is
  managed by PlainShelf, and raising it locks you out of the book.
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
- **A series belongs in `series`.** Any other short note or grouping belongs in
  `comments` or `tags`.
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
cannot validate, into the file the promises above rest on. A narrower scheme,
reserving a prefix such as `x_` for keys PlainShelf preserves untouched, is
still under consideration, but nothing is committed and nothing has been built.
Until that changes, assume any key outside the table is temporary.
