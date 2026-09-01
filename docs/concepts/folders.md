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

Some directories inside `books/` are not created by you, and PlainShelf does not
treat them as folders. They are skipped when scanning for folders and for books,
they never reach the exported book cache the Android client reads, and you
cannot create a folder with one of these names.

- Any directory whose name starts with a dot — for example `.git`, `.stfolder`
  (Syncthing), `.dropbox.cache`, `.Spotlight-V100`, `.fseventsd`, and
  `.TemporaryItems`. This one is a fixed rule.
- `@eaDir` (Synology index and thumbnails), `#recycle` (Synology network recycle
  bin), `$RECYCLE.BIN` (Windows recycle bin over SMB), and `lost+found`. These
  four are the **defaults**, and a shelf can replace them — see below.

A book package placed inside one of these directories is invisible to
PlainShelf, so deleting a book on a NAS does not bring it back through the
recycle bin.

Trying to create a folder with one of these names is refused with a message that
says which name and why, rather than the generic "invalid folder name" reported
for a name that is malformed.

### Choosing the list for one shelf

The four names above are defaults, not a fixed list. A shelf can name its own
directories: create a [`shelf.json`](data-model.md#shelfjson) at the shelf root —
beside `books/`, not inside it — and list what it should skip.

```json
{
  "schema_version": 1,
  "scan": {
    "ignored_dirs": [
      "@eaDir",
      "#recycle",
      "$RECYCLE.BIN",
      "lost+found",
      { "name": "@Snapshot", "reason": "Synology snapshot directory" }
    ]
  }
}
```

This is for the directories your own storage creates that the defaults do not
know about — a NAS snapshot directory, a photo-tool thumbnail cache, a backup
tool's working directory sitting in the middle of your library. Each entry is one
directory name, matched wherever it appears in the tree and without regard to
case; a `reason` is optional and is what PlainShelf quotes back to you when it
refuses a folder of that name.

**The list replaces the defaults rather than adding to them.** That is why the
example above repeats the four default names: keep the ones you want. A list that
leaves out `@eaDir` means a Synology share's index directories become ordinary
folders and your library grows a second copy of its own folder tree — PlainShelf
warns in the log when your list drops a default, but it does not overrule you.
An empty list, `"ignored_dirs": []`, means exactly what it says: nothing is
skipped but hidden directories.

A name you skip behaves like a default one: it is not a folder, the books inside
it are not listed, and PlainShelf refuses to create a folder with that name. Take
the name back out of the file and the directory returns after the next scan —
nothing on disk was moved or deleted in the meantime.

Because the setting lives in the shelf, every PlainShelf reading that shelf
applies it, the Android client reading it from pCloud included. The file is read
when the shelf is opened, so restart the server after editing it. There is no
settings page for it yet; it is a file you write by hand.

## Example use cases

| Use case | Folder structure |
|---|---|
| Genre classification | `Fiction/`, `Non-Fiction/`, `Poetry/` |
| Reading status | `To Read/`, `Reading/`, `Done/` |
| Language | `English/`, `中文/`, `Français/` |
| Mixed | `Fiction/English/`, `Fiction/中文/` |
