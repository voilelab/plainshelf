# Standalone Reader

`plainshelf-read` is a single executable that opens a shelf folder for reading.
There is no configuration file, no data directory, and no way for it to change
anything: the shelf is opened read-only and left exactly as it was found.

Use it when you want to read a shelf you do not want to manage — a backup, a
snapshot, a folder on a read-only mount or a network share someone else
maintains — or when you want to hand the library to a second machine without
setting a server up on it.

```bash
./plainshelf-read ~/Books
```

It prints the address it picked and opens your browser:

```text
PlainShelf v1.0.0 reading /home/you/Books
Open http://127.0.0.1:41765/
Press Ctrl-C to stop.
```

The port is chosen by the operating system, so several copies can read several
shelves at once. The server listens on the loopback address only and stops when
you press Ctrl-C.

## Flags

| Flag | Meaning |
| --- | --- |
| `-no-browser` | Print the address instead of opening a browser, for a machine that has none. |

The shelf folder is the one positional argument and is required. It must be the
folder that contains `books/` — the same path an ordinary server's `lib_root`
points at.

## What it does not touch

An ordinary PlainShelf server writes to a shelf on its own account: it creates
the shelf's folders, clears `app/tmp/`, takes a lock file and exports a book
cache on a timer. None of that has a request behind it, so refusing writes at
the API would not be enough.

The standalone reader turns all of it off. It never creates the shelf folder — a
path that does not open is reported as an error rather than made — and it keeps
its own settings store in memory, so nothing appears next to the binary either.

Reading progress, bookmarks and reading history still work. They are kept by the
browser, per device, exactly as they are on the web client, and are never
written to the shelf.

## What it does not serve

The reader mounts the routes needed to browse a shelf and read a book, and
nothing else. Editing, importing, the trash, the duplicate scan, the maintenance
views and the server logs are not available, and the interface does not offer
them — `GET /api/mode` reports `"mode": "reader"`, and the frontend keeps those
pages out of reach rather than letting them fail.

Everything else is the interface you already know: the library, the folder tree,
search and filters, the book detail page, and the reader itself.

## Reading a shelf on read-only storage

Because the reader never writes, it works against storage that will not accept
writes at all:

```bash
# A read-only mount
./plainshelf-read /mnt/backup/Books

# A snapshot
./plainshelf-read /home/you/.snapshots/daily/Books
```

A full server pointed at the same path would fail at startup, or would keep
failing to take its lock file.

!!! note "One shelf per process"
    The reader serves the single folder it was given. To browse several shelves
    at once, run one copy per shelf; each picks its own port.

## Where to get it

The prebuilt server archives contain `plainshelf-read` alongside
`plainshelf-srv`; see [Installation](installation.md). To build it from a
checkout:

```bash
npm --prefix frontend ci
npm --prefix frontend run build
go build -o plainshelf-read ./cmd/plainshelf-read
```

The frontend has to be built first: the Go binary embeds `frontend/dist`.
