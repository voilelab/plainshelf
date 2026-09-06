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
      { "name": "@eaDir" },
      { "name": "#recycle" },
      { "name": "$RECYCLE.BIN" },
      { "name": "lost+found" },
      { "name": "@Snapshot", "reason": "Synology snapshot directory" }
    ]
  }
}
```

This is for the directories your own storage creates that the defaults do not
know about — a NAS snapshot directory, a photo-tool thumbnail cache, a backup
tool's working directory sitting in the middle of your library. Every entry is an
object with a `name`: one directory name, matched wherever it appears in the tree
and without regard to case, never a path and never a pattern. The `reason` is
optional and is what PlainShelf quotes back to you when it refuses a folder of
that name.

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

## Marking a folder as adult content

A folder has no settings file of its own — it is a directory, and nothing else —
so a mark that applies to a whole folder lives in
[`shelf.json`](data-model.md#shelfjson), under `content.nsfw_folders`:

```json
{
  "schema_version": 1,
  "content": {
    "nsfw_folders": [
      { "path": "Fiction/成人", "reason": "adult shelf" }
    ]
  }
}
```

A `path` names one folder under `books/`, and it marks that folder **and every
folder below it**. The path is matched without regard to case, and leading,
trailing and doubled `/` make no difference, so `Fiction/成人`, `fiction/成人`
and `/Fiction/成人/` all name the same folder. Only whole names match: marking
`Fiction/成人` does not mark `Fiction/成人漫畫`.

A single book can also be marked on its own, with `"nsfw": true` in its
`book.json`. That one is editable in the app: open the book's metadata editor
and turn on **Adult content**. The two only ever add together:

| In `shelf.json` | In the book's `book.json` | Marked? |
|---|---|---|
| folder not listed | absent or `false` | No |
| folder not listed | `true` | Yes |
| folder listed | anything, including `false` | Yes |

The last row is deliberate: `"nsfw": false` on a book **cannot** take it out of a
marked folder. Missing a book that should have been marked is the worse mistake
of the two, so the folder wins. To exempt one book, move it out of the folder.
The metadata editor shows this: for a book inside a marked folder the switch is
on and disabled, and names the rule that marks it.

The mark travels with the shelf and is written into each entry of the exported
book cache the Android client reads, so every PlainShelf reading the shelf gets
the same answer without a central list to keep in sync. Editing `shelf.json`
takes effect when the shelf is next opened, so restart the server afterwards.

### Whether a marked book is shown

Marking a book says what it is; whether this server shows it is a separate
setting, `show_nsfw`. It is off by default, so a freshly marked book disappears
from the library as soon as the shelf is reopened.

The filtering happens on the server, not in the browser: with `show_nsfw` off, a
marked book is absent from the library listing, from the duplicate and similarity
pages and from the dashboard's totals, and every address that names it — the
book, its cover, its content, its sources — answers **404**, exactly as a book
that is not on the shelf does. Naming it some other way does not get round that:
a batch move or trash refuses the ID the same way, and the background sweeps
(content statistics, source fingerprints) and the fingerprint coverage count
leave it out too, so none of their totals can report that it exists. A folder
disappears with its books: a marked
folder is not listed, and neither is one left with nothing but marked books, so
it cannot show up in a breadcrumb or in the destination list when moving a book.

Deleting a book does not take it out of the setting's reach. The trash page
leaves out a marked book — its title, its authors and the folder it was deleted
from are all withheld — and restoring or permanently deleting it answers **404**,
the same as an ID that was never issued. The folder rule still applies there:
the trash remembers where each book was deleted from, so a book deleted out of a
marked folder stays marked while it sits in the trash.

### Moving a marked folder

A folder rule marks a *path*, so moving or renaming the folder out from under
that path unmarks everything below it at once — rename `Fiction/成人` to
`Fiction/一般`, or move `Fiction` somewhere else, and every book beneath it is
served from the next request on. With `show_nsfw` off that would be a folder's
worth of books going from hidden to public in one action nobody described, so
the folder routes ask first.

While `show_nsfw` is off, a change that would take marked content out of a marked
subtree is refused with **409** and this body, and nothing happens:

```json
{
  "error": "nsfw_reveal_requires_confirmation",
  "message": "this folder holds content the shelf marks as adult ...",
  "hidden_books": 3
}
```

`hidden_books` counts the books that would become visible. It is `0` when the
whole disclosure is a folder name — an empty marked folder is hidden too — so it
does not mean "nothing would change". Repeat the same request with `?confirm=1`
to go ahead:

```sh
curl -X POST 'http://127.0.0.1:20000/api/shelves/default_shelf/folder-moves?confirm=1' \
  -H "X-PlainShelf-Token: $PLAINSHELF_TOKEN" \
  --data '{"folder":["Fiction"],"target_folder":["Archive"]}'
```

Three routes ask: `POST .../folder-moves`, `PATCH .../folders/{path}` (the
rename) and `POST .../folder-transfers`. In the web UI the same question is a
dialog naming the count, and the change goes through when you accept it.

Only what actually becomes visible counts. A book marked in its own `book.json`
keeps that mark wherever it goes, and a folder left holding nothing but such
books stays out of the folder tree on its own account — so moving it asks
nothing. Losing the folder mark is not by itself a disclosure.

Nor does a change that was never going to happen get to ask: a read-only shelf,
or a destination the shelf already holds, is refused as itself rather than
behind a confirmation.

The cross-shelf transfer is judged on the source shelf alone, and asks for a copy
as well as a move: a copy leaves this shelf untouched but publishes the same
titles on the other one. Only a book's own `nsfw` travels with it, because that
is written in its `book.json`; `shelf.json` stays behind, so whether the target
shelf happens to mark the same path is not this shelf's answer to give.

Nothing here rewrites the mark. `shelf.json` is yours and PlainShelf only reads
it, so after a confirmed move the folder really is unmarked — the confirmation is
all this adds. With `show_nsfw` on there is nothing to reveal, and all three
routes behave exactly as they did before this existed.

Three boundaries are deliberate:

- The exported book cache is always a complete mirror, whatever this setting
  says. It records what the shelf says, and the client reading it applies its own
  setting; pruning it here would quietly become that client's whole library.
- The setting is per server, not per shelf. It sits beside `cover_to_jpg` in the
  application store, so it applies to every shelf this server serves.
- **Empty trash** erases every book in the trash, marked ones included. It is one
  command over the whole trash rather than a list of books, and leaving the
  marked ones behind would be worse than the disclosure: they would reappear the
  next time the setting is turned on, long after the trash looked empty. So that
  the confirmation cannot understate what it is about to erase, a filtered
  listing says so in an `X-PlainShelf-Trash-Partial: true` header and the web UI
  then asks about "everything in the trash" rather than naming a count. The
  header reports only that something was withheld, never what or how much.

Switch it in **Settings → Adult content**. Turning it on or off refetches the
library listing and the folder tree straight away, so the sidebar and the book
list agree with the setting without a page reload.

It is also reachable from the API, which is how a script or a headless server
sets it:

```sh
curl -X POST http://127.0.0.1:20000/api/setting/show_nsfw \
  -H "X-PlainShelf-Token: $PLAINSHELF_TOKEN" --data true
```

`GET` the same address to read it, and `DELETE` it to return to the default of
hidden.

A book already open in a browser tab is not a way round the setting: reloading
the page, or pasting the book's address into a new tab, lands on the same
"failed to load" screen any unknown book gives, because the API answers 404.

### A shelf read without a server

The Android client can read a shelf straight out of pCloud, with no PlainShelf
server between it and the files. There is no `show_nsfw` to ask for on that path,
so the app keeps its own answer: **Settings → Adult content** on such a client
offers a **Show adult content on this device** switch, off by default and stored
only on that device.

It hides the same things the server setting hides, computed from the same two
marks the shelf carries — the book's own `nsfw` and the `content.nsfw_folders`
rules, which the app reads out of `shelf.json` itself rather than trusting the
exported book cache's precomputed answer. A marked book is absent from the
library listing, its address answers "not found", a marked folder and one left
holding nothing but marked books drop out of the folder tree, and a book already
downloaded for offline reading is withheld by the offline cache itself — its
entry, its text, its cover, its sources and its illustrations — so it is gone
from **Downloaded books** and unreadable even in flight mode. Being on the device
is not a way past the switch; the stored copy is kept, because the switch hides
books rather than deleting downloads.

The two settings are deliberately **not** synchronised. They are separate
machines' decisions, and syncing them would turn "not on this phone" into "on no
client at all". So a phone pointed at a PlainShelf server rather than at pCloud
ignores its own switch entirely: the server has already filtered what it served,
and that server's `show_nsfw` stays the only authority on that path.

### Seeing which books are marked

While `show_nsfw` is on, a marked book carries an **NSFW** badge in the library
list and card views and on its detail page. That is what the badge is for: it
says which books leave when the setting is turned back off. With the setting off
there is nothing to badge — those books are not served at all.

Editing the folder list is still a `shelf.json` edit. There is no UI for it, and
it needs the server restarted; the mark belongs to the shelf, and it changes
rarely, while marking one book is an everyday action and has a control.

## Example use cases

| Use case | Folder structure |
|---|---|
| Genre classification | `Fiction/`, `Non-Fiction/`, `Poetry/` |
| Reading status | `To Read/`, `Reading/`, `Done/` |
| Language | `English/`, `中文/`, `Français/` |
| Mixed | `Fiction/English/`, `Fiction/中文/` |
