# Folders

Folders are the way PlainShelf organizes books into groups.

---

## What is a folder?

A folder is a named directory that sits between the `books/` root and one or more book folders. You can nest folders inside other folders to build a tree-shaped hierarchy.

```text
books/
├─ {book}.bookpkg/          # book at the root (no folder)
├─ Fiction/
│  ├─ {bookA}.bookpkg/      # book inside the "Fiction" folder
│  └─ Classics/
│     └─ {bookB}.bookpkg/   # book nested two levels deep
└─ Non-Fiction/
   └─ {bookC}.bookpkg/
```

---

## Key properties

- **Filesystem-backed** — folders are real directories; they survive a server restart and can be browsed in any file manager.
- **Nestable** — there is no hard limit on nesting depth.
- **Independent from book IDs** — moving a book between folders does not change its ID or break reading progress.
- **Managed via the UI** — the web interface lets you create folders, delete empty folders, and move books between folders without touching the filesystem manually.
- **Cached like the book listing** — folder changes you make in PlainShelf show up immediately; a folder created or removed outside PlainShelf appears after the next shelf scan. See [Shelf cache and disk I/O](shelf-cache-and-io.md#folder-listing).

---

## Folder rules

- A folder cannot be deleted while it still contains books (you must move or delete the books first).
- A book can only belong to one folder at a time (it lives in exactly one directory).
- The `books/` root itself acts as a "no folder" / top-level group.

---

## Ignored directories

Some directories inside `books/` are not created by you, and PlainShelf never
treats them as folders. They are skipped when scanning for folders and for books,
they never reach the exported book cache the Android client reads, and you
cannot create a folder with one of these names.

- Any directory whose name starts with a dot — for example `.git`, `.stfolder`
  (Syncthing), `.dropbox.cache`, `.Spotlight-V100`, `.fseventsd`, and
  `.TemporaryItems`.
- `@eaDir` (Synology index and thumbnails), `#recycle` (Synology network recycle
  bin), `$RECYCLE.BIN` (Windows recycle bin over SMB), and `lost+found`.

A book package placed inside one of these directories is invisible to
PlainShelf, so deleting a book on a NAS does not bring it back through the
recycle bin. The list is fixed; custom ignore patterns are not configurable yet.

Trying to create a folder with one of these names is refused with a message that
says so, rather than the generic "invalid folder name" reported for a name that
is malformed.

---

## Example use cases

| Use case | Folder structure |
|---|---|
| Genre classification | `Fiction/`, `Non-Fiction/`, `Poetry/` |
| Reading status | `To Read/`, `Reading/`, `Done/` |
| Language | `English/`, `中文/`, `Français/` |
| Mixed | `Fiction/English/`, `Fiction/中文/` |

Because folders are just directories, you can reorganize them freely without losing any book data.
