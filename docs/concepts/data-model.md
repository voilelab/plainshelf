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
   ├─ tmp/
   └─ stats/
      └─ reading/
         └─ {YYYY-MM}.json
```

### `books/`

Source of truth. This directory contains all user-owned data: book metadata, text files, cover images, and other long-lived files. Books can be nested under [layers](layers.md) by placing them inside sub-directories.

### `app/`

Runtime state used by the server: file lock and temporary files (rebuildable, not user data), plus per-day reading-time history under `stats/reading/` (one JSON file per calendar month) that powers the dashboard's reading heatmap and streak. Unlike the lock and temporary files, reading-time history is **not** derived from `books/` — deleting `app/` discards it.

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
| `book.json` | Book metadata (title, authors, tags, language, …). Also holds `current_source`, the authoritative pointer to the active source. |
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
- **Rebuildable runtime state** — the file lock and temporary files under `app/` can be deleted and the server will recreate them on the next startup. The one exception is `app/stats/reading/`: it holds reading-time history that isn't derived from `books/`, so deleting it loses that history (dashboard heatmap and streak) even though nothing else breaks.
