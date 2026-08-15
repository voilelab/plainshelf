# Data Model

PlainShelf is **filesystem-first**: the shelf directory on disk is the source of truth, and the server reads and writes it directly. There is no separate database.

---

## Shelf layout

A typical shelf looks like this:

```text
{shelf}/
├─ books/
│  ├─ {book1-folder}.bookpkg/
│  ├─ {layer1}/
│  │  └─ {book2-folder}.bookpkg/
│  └─ {layer2}/
│     └─ {layer3}/
│        └─ {book3-folder}.bookpkg/
└─ app/
   ├─ library.lock
   ├─ book-cache-{writer-id}.json
   └─ tmp/
```

### `books/`

Source of truth. This directory contains all user-owned data: book metadata, text files, cover images, and other long-lived files. Books can be nested under [layers](layers.md) by placing them inside sub-directories.

### `app/`

Runtime state used by the server: file lock, temporary files, and an exported book listing. All of it is rebuildable and none of it is user data — the server recreates it on the next startup.

`book-cache-{writer-id}.json` is a copy of the shelf's book listing, kept so that a client reading the shelf over a slow or request-metered connection — the Android app opening a shelf from pCloud — does not have to walk it book by book. Each installation writes its own file, named after an ID generated on first start, so several machines sharing a shelf do not overwrite each other; a file no installation has refreshed for 30 days is removed. Deleting them is safe: they are rebuilt, and a client that finds none simply scans instead. See [Shelf cache and disk I/O](shelf-cache-and-io.md#the-exported-book-cache).

Older shelves may still contain `app/stats/reading/{YYYY-MM}.json`. That is reading-time history from before it moved onto each device; nothing reads it any more and it can be deleted.

### Per-device reading data

Saved reading progress, reading history, and reading time are client state, not
shelf or server state. They are kept independently on the device that recorded
them and are not synchronized between devices:

| Client | Saved reading progress |
|---|---|
| Web | Browser `localStorage`, key `plainshelf.readingProgress` |
| Desktop | `reading_progress.json` next to `shelves.json` in the app data directory |
| Android | App-private `progress.json` files scoped by connection, shelf, and book |

Each position is a JavaScript UTF-16 character offset. Moving or renaming a
book does not lose it because records use the persistent book ID rather than a
filesystem path.

---

## Book folder (`.bookpkg/`)

Each book is stored as a directory whose name ends with `.bookpkg`:

```text
{book-folder}.bookpkg/
├─ book.json
├─ CURRENT_VERSION_LOCATION.txt
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
| `CURRENT_VERSION_LOCATION.txt` | Human-readable hint that points to the active source. It is **write-only** from the server's perspective (regenerated whenever the current source changes) and is never parsed back — `current_source` in `book.json` is the source of truth. |
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

Sources made before source-level format metadata remain legacy sources. They
continue to use their stored regex, line-count, or boundary split configuration
(and the legacy global default) until the user explicitly upgrades them. Merely
opening or saving such a source never changes its chapter semantics.
When a new-format source is activated, `book.json.legacy_source_formats` keeps
the previous effective format for any legacy source that was current. This is
compatibility bookkeeping only: it lets the mirror be restored when switching
back and does not add `format` or `schema_version` to the legacy source.

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
images that text renders. A book downloaded before this existed keeps working
and shows alt text where its pictures would be; downloading it again stores
them.

Uploading and deleting an illustration are ordinary writes: they need the same
access a metadata edit does, and a read-only server refuses them. Deleting one
leaves the text alone — a link to a removed file renders as its alt text rather
than the prose being rewritten underneath you.

The app has no screen for this yet; the routes are there for a client to use.

### Book IDs

The book ID is generated once when the book is created and then persisted in `book.json`; it is **not** recomputed from the folder name or the display title afterwards. This means you can rename a book's title, or move the book to a different layer, without breaking reading progress, bookmarks, or any external references.

---

## Design principles

- **Human-readable** — the shelf directory can be opened and inspected with any file manager or text editor.
- **Backup-friendly** — because everything is plain files, the shelf is trivially backed up with `cp`, `rsync`, or committed to Git.
- **Rebuildable runtime state** — everything under `app/` can be deleted and the server will recreate it on the next startup. See [Back up before upgrading](data-format-versioning.md#back-up-before-upgrading) for what a complete backup covers.
