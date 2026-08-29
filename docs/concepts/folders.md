# Folders

A folder is a named directory sitting between the `books/` root and one or more
book folders. Folders nest inside each other to whatever depth you want:

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

They are real directories, so they survive a server restart and can be browsed
in any file manager. The `books/` root is itself the top-level group: a book
sitting directly under it is in no folder.

Create folders, move books between them, and delete empty ones from the web
interface — none of it needs the filesystem touched by hand. A folder change you
make in PlainShelf appears in the very next listing; one made outside PlainShelf
appears after the next shelf scan. See
[Shelf cache and disk I/O](shelf-cache-and-io.md#folder-listing).

## Rules

- A folder cannot be deleted while it still holds books. Move or delete the
  books first.
- A book belongs to exactly one folder, because it lives in exactly one
  directory.
- Moving a book between folders does not change its ID, so reading progress and
  bookmarks follow it.

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

## Example use cases

| Use case | Folder structure |
|---|---|
| Genre classification | `Fiction/`, `Non-Fiction/`, `Poetry/` |
| Reading status | `To Read/`, `Reading/`, `Done/` |
| Language | `English/`, `中文/`, `Français/` |
| Mixed | `Fiction/English/`, `Fiction/中文/` |
