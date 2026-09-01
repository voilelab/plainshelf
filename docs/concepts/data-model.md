# Data Model

PlainShelf is **filesystem-first**: the shelf directory on disk is the source of truth, and the server reads and writes it directly. There is no separate database.

---

## Shelf layout

A typical shelf looks like this:

```text
{shelf}/
├─ shelf.json          (optional; this shelf's own settings)
├─ books/
│  ├─ {book1-folder}.bookpkg/
│  ├─ {folder1}/
│  │  └─ {book2-folder}.bookpkg/
│  └─ {folder2}/
│     └─ {folder3}/
│        └─ {book3-folder}.bookpkg/
├─ trash/
│  └─ books/
│     └─ {book-id}.bookpkg/
│        └─ trash.json
└─ app/
   ├─ library.lock
   ├─ book-cache-{writer-id}.json
   ├─ scan-cache.json
   ├─ fingerprint-cache.json
   └─ tmp/
```

The three top-level directories are not the same kind of thing:

| Directory | Contents | Safe to delete? |
|---|---|---|
| `books/` | Your library. The source of truth. | No |
| `trash/` | Books you deleted, kept until you empty the trash. Your data too. | No — deleting it discards those books for good |
| `app/` | Runtime state the server rebuilds. | Yes |

`shelf.json` beside them is optional and holds settings that belong to this
shelf rather than to the program reading it — see [`shelf.json`](#shelfjson)
below. Deleting it is safe in the sense that nothing breaks: the shelf goes back
to the built-in defaults.

Before changing this layout — renaming, adding, or removing a top-level
directory, or how book folders are named or nested — read [Shelf layout changes
are not versioned](data-format-versioning.md#shelf-layout-changes-are-not-versioned):
the shelf carries no top-level version marker, and that page gives the test for
what counts as a layout change and how one is expected to be detected.

### `books/`

Source of truth. This directory contains all user-owned data: book metadata, text files, cover images, and other long-lived files. Books can be nested under [folders](folders.md) by placing them inside sub-directories.

### `trash/`

Books you deleted. Trashing a book moves its whole `.bookpkg` directory here
unchanged and writes a `trash.json` beside its `book.json`; restoring moves it
back and removes that file. Nothing is deleted until you empty the trash or
delete a single book from it, so this is user data and **not** rebuildable
state — a backup that skips it loses whatever you had not yet emptied.

```text
trash/
└─ books/
   └─ {book-id}.bookpkg/
      ├─ trash.json
      └─ …the book's own files, untouched
```

The folder here is named after the book's ID rather than its title, so two
books with the same title cannot collide. `trash.json` records when the book
was deleted and where it came from:

| Field | Description |
|---|---|
| `schema_version` | On-disk format version of this file. Managed by PlainShelf; see [Data Format Versioning](data-format-versioning.md#trash-metadata-schema-v2) |
| `deleted_at` | When the book was moved to the trash |
| `original_path` | The path it was moved from, used to restore its folder name |
| `original_folder` | The [folder](folders.md) it lived in, recreated on restore |
| `delete_reason` | Why it was deleted; `user` for a deletion you asked for |

If the file is missing or unreadable the book still appears in the trash and can
still be restored — it lands at the top level of `books/` under the folder name
it has in the trash.

A `trash.json` written by a *newer* PlainShelf is the opposite case: the book is
still listed with everything the file says, but this build will not rewrite that
record, so restoring or permanently deleting it is refused. See
[Data Format Versioning](data-format-versioning.md#trash-metadata-schema-v2).

Restoring never overwrites: if something already occupies the original path, the
restored folder gets a `-1`, `-2`, … suffix. The book ID inside `book.json` is
unchanged either way, so reading progress and bookmarks survive the round trip.

### `trash/` was `.trash/` before

Older shelves keep the trash in a hidden `.trash/` directory. PlainShelf renames
it to `trash/` the next time it opens such a shelf; if both names exist — a
shelf opened by an older build again after the rename — every book under
`.trash/books/` is moved into `trash/books/`, a book ID already taken there
getting a `-1` suffix, and the emptied `.trash/` is removed. Files you put under
`.trash/` yourself are moved along or, if they block the removal, left exactly
where they are; the migration deletes nothing.

That suffix is the one case where the trash holds two folders for the same book
ID. A book ID is never rewritten — everything else, including your reading
progress, is keyed on it — so the trash screen cannot tell the two apart: it
lists both and restores or deletes the one without the suffix. Move the extra
folder back into `books/` with a file manager if you want it, or empty the
trash, which removes both.

The trash is now visible because it holds your data. A shelf you open in a file
manager should show every directory PlainShelf keeps, and the one directory that
is genuinely disposable, `app/`, is the one you would least mind seeing.

### `app/`

Runtime state used by the server: file lock, temporary files, and an exported book listing. All of it is rebuildable and none of it is user data — the server recreates it on the next startup.

`book-cache-{writer-id}.json` is a copy of the shelf's book listing, kept so that a client reading the shelf over a slow or request-metered connection — the Android app opening a shelf from pCloud — does not have to walk it book by book. Each installation writes its own file, named after an ID generated on first start, so several machines sharing a shelf do not overwrite each other; a file no installation has refreshed for 30 days is removed. Deleting them is safe: they are rebuilt, and a client that finds none simply scans instead. See [Shelf cache and disk I/O](shelf-cache-and-io.md#the-exported-book-cache).

`scan-cache.json` records the modification time of each directory under `books/` alongside the entries found there, so a scan can skip listing the folders that have not changed since the last one. It is validated against the real modification times every time it is used, so a stale or foreign copy costs a slower scan and nothing else. Deleting it is safe; the next scan writes a new one.

`fingerprint-cache.json` stores the content fingerprints behind the [Similar Books](../finding-similar-books.md) page, so that page does not have to re-read every book each time you open it. Unlike the exported book cache there is one shared file, not one per installation — a fingerprint is a pure function of a source's content, so every machine computes the same value and they merge into the one file. It is the largest thing under `app/`, growing with your library at roughly 1.5 KB per source, so on a big shelf it reaches a few megabytes and you will notice it in a backup. Deleting it is safe: the next time you build fingerprints, the ones that are gone are simply read and computed again. It is also discarded and rebuilt whenever the similarity algorithm changes — see [Data Format Versioning](data-format-versioning.md#fingerprint-cache).

Older shelves may still contain `app/stats/reading/{YYYY-MM}.json`. That is reading-time history from before it moved onto each device; nothing reads it any more and it can be deleted.

### `shelf.json`

This shelf's own settings. Optional: a shelf without one is read exactly as
PlainShelf has always read it, and no build creates the file for you.

It sits at the shelf root rather than under `app/` because `app/` is disposable
and a setting that vanishes with the cache would be worse than no setting. It
travels with the shelf rather than with the server's configuration file so that
every reader of the same shelf — this server, the desktop app, the Android
client reading it from pCloud — applies the same rules.

```json
{
  "schema_version": 1,
  "scan": {
    "ignored_dirs": [
      "Thumbs",
      { "name": "@Snapshot", "reason": "Synology snapshot directory" }
    ]
  }
}
```

| Field | Meaning |
|---|---|
| `schema_version` | Format version of this file. `1` is the only one this build writes about; a file declaring a higher one is still read, and the fields this build understands still apply |
| `scan.ignored_dirs` | The directory names under `books/` this shelf skips, **replacing** the [defaults](folders.md#ignored-directories). Each entry is one name — not a path and not a pattern — matched without regard to case, written either as a string or as `{"name": …, "reason": …}`. The reason is shown when a folder of that name is refused |

Leaving `ignored_dirs` out is not the same as giving an empty list: without the
field the shelf skips the defaults, and with `"ignored_dirs": []` it skips
nothing but hidden directories, which are a rule rather than a name and are
never listed here.

PlainShelf only ever reads this file. Nothing rewrites it, so your formatting and
key order survive, and the settings are read when the shelf is opened: edit the
file and restart the server (or reopen the shelf in the desktop app) for the
change to take effect.

An entry that cannot name a directory is skipped rather than fatal — one
containing a `/`, or one that is not a name at all — and the rest of the file
still applies. A file that cannot be read as a single JSON object at all, or one
larger than 1 MiB, leaves the defaults in place; so does a key this build does
not know. The server log says what was dropped and why, and warns when your list
leaves out a default such as `@eaDir`, because that is how a library grows a
duplicate folder tree.

### Per-device reading data

Saved reading progress, reading history, and reading time are client state, not
shelf or server state. They are kept independently on the device that recorded
them and are not synchronized between devices:

| Client | Saved reading progress |
|---|---|
| Web | Browser `localStorage`, key `plainshelf.readingProgress` |
| Desktop | `reading_progress.json` next to `shelves.json` in the app data directory |
| Standalone reader | The desktop's `reading_progress.json` (shared), under a synthetic `book` shelf key |
| Android | App-private `progress.json` files scoped by connection, shelf, and book |

Each position is a JavaScript UTF-16 character offset. Moving or renaming a
book does not lose it because records use the persistent book ID rather than a
filesystem path.

The standalone reader writes into the desktop app's `reading_progress.json` so a
book read there shows the same position in the desktop library. Because the
reader has no real shelf, it stores progress under a synthetic `book` shelf key,
which the desktop app projects onto the book's real shelf by stable book ID; see
[Reading state is not part of the shelf](architecture.md#reading-state-is-not-part-of-the-shelf).

---

## Book folder (`.bookpkg/`)

Each book is stored as a directory whose name ends with `.bookpkg`:

```text
{book-folder}.bookpkg/
├─ book.json
├─ CURRENT_SOURCE.txt
├─ cover.(jpg|png|webp)
└─ sources/
   └─ {source-id}/
      ├─ source.txt
      ├─ meta.json
      └─ assets/
         └─ img-0001.png
```

| Path | Description |
|---|---|
| `book.json` | Book metadata (title, authors, tags, language, …). Also holds `current_source`, the authoritative pointer to the active source, and `schema_version`, the on-disk format version — see [Data Format Versioning](data-format-versioning.md). |
| `CURRENT_SOURCE.txt` | Human-readable hint, in English, that points to the active source. The server writes it whenever the current source changes and never parses it back — `current_source` in `book.json` is the source of truth — so deleting it is safe. Shelves written by older builds carry the same hint as `CURRENT_VERSION_LOCATION.txt`; the next write replaces it with `CURRENT_SOURCE.txt` rather than leaving both. |
| `cover.(jpg\|png\|webp)` | Optional cover image |
| `sources/{source-id}/source.txt` | The content and, for Markdown, its chapter structure |
| `sources/{source-id}/meta.json` | Source-level metadata, including `schema_version` and authoritative `format` for new sources |
| `sources/{source-id}/assets/` | Optional illustrations this source's text references |

### Source format and chapters

For schema-versioned sources, `sources/{source-id}/meta.json` records `format`
as `txt` or `md`. `book.json.format` is only a compatibility mirror of the
current source for older clients. Readers resolve format in this order:

1. current source `meta.json.format`;
2. legacy `book.json.format`;
3. `txt`.

Markdown chapter structure is part of `source.txt`: every ATX H2 line
(`## Title`) outside a fenced code block begins a chapter. The H2 stays in the
body and its text is the chapter title. Meaningful content before the first H2
is an opening section, named by its first H1 when present. H3 and lower
headings, `---`, and `***` never split chapters.

TXT sources are deliberately unstructured and always read as one section. To
add chapters, the source editor creates a new Markdown source and leaves the
TXT original intact. Converting Markdown to plain text likewise creates a new
source because heading hierarchy and chapter navigation are lost.

Sources made before source-level format metadata remain legacy sources. Their
stored `split_config` is no longer interpreted: such a source reads as
`book.json.format` says, which is one plain-text section unless the book is
Markdown. To give one chapters again, convert it in the source editor, which
writes a new schema v1 source and leaves the legacy original intact — see
[Data format versioning](data-format-versioning.md#giving-a-legacy-source-chapters-again).

### Adding and removing sources

A book always has at least one source. Creating a book creates an empty one and
points `current_source` at it, and deleting a book's only source leaves an empty
plain-text source behind rather than a book with nothing to read. That
replacement is a new source with its own ID, so `book.json.format` follows it to
`txt` and it carries no assets.

Deleting the current source hands `current_source` over to the newest surviving
source before the folder is removed, so the pointer is never left naming
something that is gone. Deleting any other source does not touch `book.json` at
all.

The shelf is also edited by hand and by sync tools, so `current_source` can still
end up naming a source that no longer exists. Reads tolerate this: the server
serves the newest source the book does have and logs a warning. It does **not**
rewrite `book.json` — the filesystem stays the source of truth, and only an
explicit write may change it. A book with no source at all is reported as a
missing source, not a server error.

### Source assets

Illustrations live in an `assets/` directory beside the text that references
them, so a Markdown image link in `source.txt` is an ordinary relative path:

```markdown
![A map of the province](assets/img-0001.png)
```

That path resolves the same way in the reader as it does in any editor you open
`source.txt` with. It also means a source owns its images: deleting a source
deletes its illustrations too, so nothing is left orphaned.

The directory is flat, and file names must be a plain `.jpg`, `.jpeg`, `.png`,
`.webp`, or `.gif`. Nothing records its contents — the filesystem is the list —
so adding or removing an image is just adding or removing a file, and
`book.json` never changes because of it.

Files get here three ways: [EPUB import](../epub-import.md#illustrations) writes
the illustrations it kept, the API stores and removes them, and you can drop
your own images in by hand.

The reader displays these images when the current source's effective format is
Markdown. A plain-text source has no image syntax, so its illustrations are
never shown.

Only a line that is nothing but an image becomes an illustration; an `![]()`
inside a sentence stays as text. The link target must be a single file directly
inside `assets/` — external URLs, `data:` targets, and paths that climb out of
the directory are left as text rather than fetched, so a book's own text cannot
make the reader reach out to the network. An image that cannot be loaded is
replaced by its alt text instead of leaving a gap.

A file name containing spaces can be written plainly, in angle brackets, or
percent-encoded; all three name the same file:

```markdown
![](assets/A Map.png)
![](<assets/A Map.png>)
![](assets/A%20Map.png)
```

Downloading a book for offline reading on Android brings its illustrations
along: the download reads the text it just fetched and stores exactly the
images that text renders. It asks for them as one zip — a single request for
the whole source's figures rather than one per image, which is what a
high-latency or metered mobile connection actually pays for — unpacking and
storing each figure as it goes so an image-heavy book never holds them all in
memory at once. When the server is too old to offer that bundle, or the bundled
request fails, the download falls back to fetching each image on its own. A book
downloaded before this existed keeps working and shows alt text where its
pictures would be; downloading it again stores them.

The bundle is a read-only endpoint beside the per-image one — a source's
`assets.zip`, taking an optional list of file names so the client packs only
what its text references. Like every asset read it is a plain `GET`, gated only
when the server sets `protect_read`; online reading still fetches each
illustration lazily and is unchanged by it.

Uploading and deleting an illustration are ordinary writes: they need the same
access a metadata edit does, and a read-only server refuses them. Deleting one
leaves the text alone — a link to a removed file renders as its alt text rather
than the prose being rewritten underneath you.

The app has no screen for this yet; the routes are there for a client to use.

### Book IDs

The book ID is generated once when the book is created and then persisted in
`book.json`; it is **not** recomputed from the folder name or the display title
afterwards. This means you can rename a book's title, or move the book to a
different folder, without breaking reading progress, bookmarks, or any external
references.

The ID is a version 4 UUID, such as
`9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d`. It carries no information about the book:
122 bits of randomness is what keeps two books apart, so a book you add on one
machine cannot collide with one another machine added to the same shared shelf,
or one you copied in with a file manager, even though neither side can see the
other's book yet. PlainShelf still checks the shelf and its trash for the drawn
ID before using it, but that check is insurance, not the guarantee. Shelves
written by earlier versions carry other forms — an 8-character hex ID, or a
16-character word of `a`–`z` and `2`–`7` — and those keep working unchanged,
side by side with the UUIDs; nothing is migrated.

Copying or moving a book to a **different shelf** applies the same principle from
two directions. A **move** keeps the ID: it is the same book, now living on
another shelf, so reading progress and any external reference to it carry over
unbroken. The move publishes the book on the destination in full before removing
it from the source, so an interruption never loses it from both — at worst it
ends up copied to the destination and still recoverable in the source's trash.
Because the ID is preserved, a move onto a shelf that already holds a book with
that ID is refused rather than silently overwriting it. A **copy** does the
opposite: the copy is a new, independent book, so it is minted a fresh ID drawn
from the destination shelf and the original and the copy coexist — the same
reason a copy made within one shelf gets a new ID.

Shelves created by earlier versions hold 8-character hexadecimal IDs, some with
a `-1`-style suffix, which were derived from the folder path and title at
creation time. Those are kept exactly as they are — nothing is renumbered, and
old and new IDs work side by side in the same shelf. An ID that looks derived
never was reproducible in practice, because it was only ever computed once; the
random form makes that plain.

---

## Backing up a shelf

Because it is all plain files, the shelf can be inspected with any file manager or text editor and copied with `cp -a` or `rsync`. Committing it to Git is *not* an equivalent option: Git does not track empty directories, so a folder holding no book is not in the commit and is not there after a checkout. See [Git does not back up empty folders](data-format-versioning.md#git-does-not-back-up-empty-folders), and [Back up before upgrading](data-format-versioning.md#back-up-before-upgrading) for what a complete backup covers.
