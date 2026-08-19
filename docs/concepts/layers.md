# Layers

Layers are the way PlainShelf organizes books into groups. They map directly to sub-directories inside the `books/` folder of your shelf.

---

## What is a layer?

A layer is a named directory that sits between the `books/` root and one or more book folders. You can nest layers inside other layers to build a tree-shaped hierarchy.

```text
books/
├─ {book}.bookpkg/          # book at the root (no layer)
├─ Fiction/
│  ├─ {bookA}.bookpkg/      # book inside the "Fiction" layer
│  └─ Classics/
│     └─ {bookB}.bookpkg/   # book nested two levels deep
└─ Non-Fiction/
   └─ {bookC}.bookpkg/
```

---

## Key properties

- **Filesystem-backed** — layers are real directories; they survive a server restart and can be browsed in any file manager.
- **Nestable** — there is no hard limit on nesting depth.
- **Independent from book IDs** — moving a book between layers does not change its ID or break reading progress.
- **Managed via the UI** — the web interface lets you create layers, delete empty layers, and move books between layers without touching the filesystem manually.
- **Cached like the book listing** — layer changes you make in PlainShelf show up immediately; a layer directory created or removed outside PlainShelf appears after the next shelf scan. See [Shelf cache and disk I/O](shelf-cache-and-io.md#layer-listing).

---

## Layer rules

- A layer cannot be deleted while it still contains books (you must move or delete the books first).
- A book can only belong to one layer at a time (it lives in exactly one directory).
- The `books/` root itself acts as a "no layer" / top-level group.

---

## Ignored directories

Some directories inside `books/` are not created by you, and PlainShelf never
treats them as layers. They are skipped when scanning for layers and for books,
they never reach the exported book cache the Android client reads, and you
cannot create a layer with one of these names.

- Any directory whose name starts with a dot — for example `.git`, `.stfolder`
  (Syncthing), `.dropbox.cache`, `.Spotlight-V100`, `.fseventsd`, and
  `.TemporaryItems`.
- `@eaDir` (Synology index and thumbnails), `#recycle` (Synology network recycle
  bin), `$RECYCLE.BIN` (Windows recycle bin over SMB), and `lost+found`.

A book package placed inside one of these directories is invisible to
PlainShelf, so deleting a book on a NAS does not bring it back through the
recycle bin. The list is fixed; custom ignore patterns are not configurable yet.

---

## Example use cases

| Use case | Layer structure |
|---|---|
| Genre classification | `Fiction/`, `Non-Fiction/`, `Poetry/` |
| Reading status | `To Read/`, `Reading/`, `Done/` |
| Language | `English/`, `中文/`, `Français/` |
| Mixed | `Fiction/English/`, `Fiction/中文/` |

Because layers are just directories, you can reorganize them freely without losing any book data.
