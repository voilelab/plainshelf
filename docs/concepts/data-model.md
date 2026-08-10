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

Runtime state used by the server: file lock and temporary files. Current builds
keep no user-authored data here and recreate the runtime files on startup. The
one exception is an upgraded v0.8 shelf: its legacy reading-time files remain
under `app/stats/reading/` until the operator archives or removes them.

Older shelves may still contain `app/stats/reading/{YYYY-MM}.json`. That is
reading-time history from before it moved onto each device. v1 does not read or
import it. Keep the files if you want an archive or may need to recover the old
dashboard with v0.8; otherwise they can be deleted.

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
      └─ meta.json
```

| Path | Description |
|---|---|
| `book.json` | Book metadata (title, authors, tags, language, …). Also holds `current_source`, the authoritative pointer to the active source, and `schema_version`, the on-disk format version — see [Data Format Versioning](data-format-versioning.md). |
| `CURRENT_VERSION_LOCATION.txt` | Human-readable hint that points to the active source. It is **write-only** from the server's perspective (regenerated whenever the current source changes) and is never parsed back — `current_source` in `book.json` is the source of truth. |
| `cover.(jpg\|png\|webp)` | Optional cover image |
| `sources/{source-id}/source.txt` | The plain-text content for this source |
| `sources/{source-id}/meta.json` | Source-level metadata |

### Book IDs

The book ID is generated once when the book is created and then persisted in `book.json`; it is **not** recomputed from the folder name or the display title afterwards. This means you can rename a book's title, or move the book to a different layer, without breaking reading progress, bookmarks, or any external references.

---

## Design principles

- **Human-readable** — the shelf directory can be opened and inspected with any file manager or text editor.
- **Backup-friendly** — because everything is plain files, the shelf is trivially backed up with `cp`, `rsync`, or committed to Git.
- **Rebuildable current runtime state** — `app/library.lock` and `app/tmp/` can
  be deleted and the server will recreate them on the next startup. Keep a
  v0.8 shelf's `app/stats/reading/` until it has been archived or is no longer
  needed for recovery.
  See [Back up before upgrading](data-format-versioning.md#back-up-before-upgrading)
  for what a complete backup covers.
