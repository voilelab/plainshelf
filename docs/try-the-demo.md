# Try the Demo

The demo at <https://plainshelf.org/demo/> is the full PlainShelf web app,
running entirely in your browser. You can try it without installing anything.
The PlainShelf server is compiled to WebAssembly and runs in the page itself,
and the library is kept in your browser's storage. The site only serves the
app's files. Books you import are converted in the browser and are never
uploaded.

The demo is rebuilt with every release, so it matches the latest
[release](installation.md).

## What you can do

The demo starts with an empty shelf named **Demo Shelf**. You can import TXT,
Markdown and EPUB files ([EPUB Import](epub-import.md)), read them, sort them
into folders, and use the trash, just as with the server.

A **Demo** bar in the corner adds two buttons that the regular app does not
have:

- **Export shelf** downloads the whole shelf as a zip.
- **Import shelf…** replaces the whole shelf with a zip.

Do not confuse **Import shelf…** with the app's own **Import** menu, which adds
individual books.

## Where your library is kept

The shelf is stored in the browser's
[Origin Private File System](https://developer.mozilla.org/docs/Web/API/File_System_API/Origin_private_file_system),
so it survives a reload or closing the browser. It is kept separately for each
browser and each device. Nothing syncs it between them.

- **Clearing site data** for plainshelf.org deletes the library. Export it
  first if you want to keep it.
- **Several tabs** share one library. One tab runs the server and the others
  pass their requests to it. When that tab closes, another one takes over.
- **Closing the tab** right after a change can lose the last moment of edits.
  A change is written to storage within a fraction of a second, and a closing
  tab cannot wait for that write to finish.
- **Reading progress, history and stats** are kept in the browser's local
  storage, as in the regular web app
  ([Architecture](concepts/architecture.md#reading-state-is-not-part-of-the-shelf)).
  They are not part of an export.

If the browser offers no such storage, the demo still runs, but only in memory:
a reload starts over with an empty shelf, and **Import shelf…** is refused
because the reload that follows an import would lose it. This can happen in
some private windows. Even where a private window has storage, the browser
discards it when the window closes.

## Move a library between the demo and a real install

An export is an ordinary shelf. The zip holds `books/` and `trash/`, the same
layout as a shelf directory's `lib_root` ([Data Model](concepts/data-model.md)).

**From the demo to PlainShelf:**

1. Click **Export shelf**.
2. Unzip the file into an empty shelf directory.
3. Point a [local shelf](configuring-local-shelf.md) at that directory.

Book IDs, folders, covers and the trash come along.

**From PlainShelf to the demo:**

1. Stop PlainShelf, as for a [backup](backup-and-restore.md#stop-plainshelf-first).
2. Zip the shelf directory. The zip may have the directory itself at the top.
3. Click **Import shelf…** and choose the zip.

Only `books/` and `trash/` travel, in either direction. `app/` holds an
install's caches and is rebuilt. [`shelf.json`](concepts/data-model.md#shelfjson),
the shelf's own settings such as ignored directories and NSFW folders, is left
out too, so the demo runs with those settings at their defaults. Moving to a
real install, copy `shelf.json` across yourself if the original shelf had one.

An import replaces the demo's books and trash, and every open tab reloads onto
the imported shelf. A file that is not a shelf zip is refused and changes nothing.
The zip may be at most 512 MiB.

## Limits

- **The whole shelf is held in memory** while the demo runs, so a large library
  is slow to open and costs memory in proportion to its size. Your browser's
  storage quota also applies.
- **One shelf only.** The demo has no config file, so it cannot add shelves or
  use an [SMB shelf](configuring-smb-shelf.md).
- **The demo needs HTTPS and a service worker.** Covers are loaded through the
  service worker. If the browser blocks service workers, the demo still works
  without covers.
- **Not every browser is tested.** Chromium runs the demo's automated check on
  every change, and Firefox runs it nightly. Safari's storage has not been
  verified, because the WebKit engine used in testing does not provide it.

For daily use, [install PlainShelf](installation.md). Your files then live in a
folder you control and can back up, rather than inside a browser.
