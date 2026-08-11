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
   └─ tmp/
```

### `books/`

Source of truth. This directory contains all user-owned data: book metadata, text files, cover images, and other long-lived files. Books can be nested under [layers](layers.md) by placing them inside sub-directories.

### `app/`

Runtime state used by the server: file lock and temporary files. All of it is rebuildable and none of it is user data — the server recreates it on the next startup.

Older shelves may still contain `app/stats/reading/{YYYY-MM}.json`. That is reading-time history from before it moved onto each device; nothing reads it any more and it can be deleted.

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
| `sources/{source-id}/source.txt` | The plain-text content for this source |
| `sources/{source-id}/meta.json` | Source-level metadata |
| `sources/{source-id}/assets/` | Optional illustrations this source's text references |

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

The reader displays these images for books stored as Markdown (`"format": "md"`
in `book.json`). A plain-text book has no image syntax, so its illustrations are
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

Today PlainShelf only reads this directory: you put the images there yourself.
Adding them from the app, and carrying them into offline downloads, are not
supported yet.

### Book IDs

The book ID is generated once when the book is created and then persisted in `book.json`; it is **not** recomputed from the folder name or the display title afterwards. This means you can rename a book's title, or move the book to a different layer, without breaking reading progress, bookmarks, or any external references.

---

## Design principles

- **Human-readable** — the shelf directory can be opened and inspected with any file manager or text editor.
- **Backup-friendly** — because everything is plain files, the shelf is trivially backed up with `cp`, `rsync`, or committed to Git.
- **Rebuildable runtime state** — everything under `app/` can be deleted and the server will recreate it on the next startup. See [Back up before upgrading](data-format-versioning.md#back-up-before-upgrading) for what a complete backup covers.
