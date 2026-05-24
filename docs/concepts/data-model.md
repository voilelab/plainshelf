# Data Model

PlainShelf is **filesystem-first**: the shelf directory on disk is the source of truth, and the server reads and writes it directly. There is no separate database.

---

## Shelf layout

A typical shelf looks like this:

```text
{shelf}/
├─ books/
│  ├─ {book1-folder}.novl/
│  ├─ {layer1}/
│  │  └─ {book2-folder}.novl/
│  └─ {layer2}/
│     └─ {layer3}/
│        └─ {book3-folder}.novl/
└─ app/
   ├─ library.lock
   └─ tmp/
```

### `books/`

Source of truth. This directory contains all user-owned data: book metadata, text files, cover images, and other long-lived files. Books can be nested under [layers](layers.md) by placing them inside sub-directories.

### `app/`

Runtime state used by the server (file lock, temporary files). This data is considered rebuildable and is **not** user data.

---

## Book folder (`.novl/`)

Each book is stored as a directory whose name ends with `.novl`:

```text
{book-folder}.novl/
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
| `book.json` | Book metadata (title, authors, tags, language, …) |
| `CURRENT_VERSION_LOCATION.txt` | Points to the active source used for reading |
| `cover.(jpg\|png\|webp)` | Optional cover image |
| `sources/{source-id}/source.txt` | The plain-text content for this source |
| `sources/{source-id}/meta.json` | Source-level metadata |

### Book IDs

The book ID is derived from the folder name, **not** from the display title. This means you can rename a book's title in `book.json` without breaking reading progress, bookmarks, or any external references.

---

## Design principles

- **Human-readable** — the shelf directory can be opened and inspected with any file manager or text editor.
- **Backup-friendly** — because everything is plain files, the shelf is trivially backed up with `cp`, `rsync`, or committed to Git.
- **Rebuildable runtime state** — the `app/` directory can be deleted and the server will recreate it on the next startup.
