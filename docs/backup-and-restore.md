# Backup and Restore

PlainShelf tells you to back up before upgrading. This page is the part that
makes that instruction usable: **what to copy, and how to put it back.**

For what those files actually are, see [Data Model](concepts/data-model.md); for
what an upgrade does to their format, see
[Data Format Versioning](concepts/data-format-versioning.md). This page does not
repeat either — it assumes you want a working library back, not an explanation
of the layout.

!!! warning "Pre-alpha"
    Until 1.0, an upgrade can change the on-disk format in a way an older build
    will not read. A backup taken immediately before each upgrade is the only
    supported way back to the previous release. See
    [Rolling back to an older release](#rolling-back-to-an-older-release).

---

## What a backup has to cover

Your library is not in one place. Four things hold state, and they are not
equally important or equally replaceable:

| What | Where it lives | Holds | If you lose it |
|---|---|---|---|
| **Shelf directory** | `lib_root` in the config file | `books/` (your library) and `trash/` (deleted books not yet emptied). `app/` is rebuildable cache. | Your books are gone. Nothing rebuilds them. |
| **Application store** | `app_conf.store_path` | Three server settings: cover-to-JPG conversion, the EPUB import preset, and log retention. | Those settings return to the values in your config file, or to the built-in defaults. Nothing else. |
| **Config file** | wherever you pass `-conf` | Shelf list, bind address, security mode, log settings. | You reconfigure by hand — and until you do, PlainShelf does not know where your shelf is. |
| **Client device data** | the browser profile, the desktop app's data directory, or the phone | Reading progress, read history, reading time. **Not on the server.** | Every book reopens at the beginning on that device. |

Only the first row is irreplaceable, and only the first row is what most people
picture when they hear "back up the shelf".

### Do they have to be from the same moment?

Mostly no, and that is a deliberate consequence of the filesystem-first design.

- **The shelf and the store are independent.** Nothing in the store refers to a
  book, a folder, or an ID. Restoring last week's store beside today's shelf
  gives you last week's three settings and today's library; nothing breaks.
- **The config file is independent too**, as long as its `lib_root` and
  `store_path` still point at where you restored those directories.
- **The shelf directory must be restored as one unit.** `books/` and `trash/`
  reference each other: `trash.json` records the path a book came from, and
  restoring a book from the trash puts it back there. Copying `books/` from one
  backup and `trash/` from another leaves the trash pointing at paths that no
  longer mean what it thinks.
- **Client device data is per device and per origin,** so there is no shared
  moment to be consistent with in the first place. See
  [Reading progress, history, and time](#reading-progress-history-and-time).

---

## What a shelf-only backup loses

This is the trap. "The filesystem is the source of truth" is true, and it makes
people conclude that copying the shelf directory is a complete backup. It is
not, and the gap is not in the books — it is in everything the shelf was never
holding.

Restore only the shelf directory and you get back, intact and byte-for-byte:
every book, every book **ID**, the folder tree including folders that hold no
book, the trash with its original paths, and every source file. What you do
**not** get back:

- **Reading progress, read history, and reading time.** These are per-device and
  have not been on the server since v0.9.0. A server-side backup has never
  contained them and restoring one will not bring them back. This is the single
  most common surprise.
- **The three stored server settings.** They are in the application store, not
  the shelf. After a shelf-only restore they silently revert — the value in your
  config file if it sets one, the built-in default otherwise.
- **Your configuration**, if the config file lived outside the shelf, which it
  normally does.

None of that is a corruption; a shelf-only restore gives you a working library.
It just gives you a slightly different one than you had, and it does so without
an error message.

---

## Taking a backup

### Stop PlainShelf first

Stop the server or quit the desktop app before you copy anything. This is not a
formality:

- **The application store is a database directory,** not a plain file. A copy
  taken while the process is running can catch it mid-compaction, and the copy
  is not a point-in-time snapshot of the whole directory.
- **The shelf lock does not help you.** `app/library.lock` coordinates
  PlainShelf's own writers against each other. It does not stop `cp` or `rsync`
  from reading a file, and it is not a snapshot mechanism. What it *does* give
  you is that each individual file is written atomically — a temp file renamed
  over its destination — so a copy never catches a half-written `book.json`.
  See [Write safety](concepts/shelf-cache-and-io.md#write-safety).

So the risk of copying a running shelf is not a torn file; it is a torn
*library*: a book copied before an edit and its source copied after, or a book
whose move between folders is half-reflected in the copy. On an idle shelf that
window is empty and a running copy is usually fine. Before an upgrade, when the
backup is the thing you are relying on, take the ten seconds and stop the
process.

If you cannot stop it — a shared server, a scheduled job — use a filesystem
snapshot (LVM, ZFS, APFS, a NAS snapshot) and copy from the snapshot. That gives
you the point-in-time consistency the copy itself cannot.

### Server (binary or built from source)

```sh
# 1. Stop the server.
# 2. Copy, dated so you can tell backups apart.
cp -a /path/to/shelf  /path/to/backup/shelf-2026-09-03
cp -a /path/to/store  /path/to/backup/store-2026-09-03
cp    /path/to/config.yaml /path/to/backup/config-2026-09-03.yaml
```

`rsync -a` is equivalent and better for repeated backups:

```sh
rsync -a --delete /path/to/shelf/ /path/to/backup/shelf/
rsync -a --delete /path/to/store/ /path/to/backup/store/
```

`lib_root` and `app_conf.store_path` in your config file are the two paths to
substitute. See [Local Shelf File Source](configuring-local-shelf.md).

You may exclude `app/` — it is rebuilt on the next start — but excluding it
saves little and risks a filter that also drops `trash/`. Copying it is simpler
and safe.

!!! warning "Do not skip dotfiles"
    Older shelves keep the trash in a hidden `.trash/` directory, so a backup
    command that skips dotfiles silently drops every book you deleted but had
    not emptied. `cp -a` and `rsync -a` as shown include it. See [`trash/` was
    `.trash/` before](concepts/data-model.md#trash-was-trash-before).

!!! warning "Do not back up the shelf with Git"
    Git tracks files, not directories, so a folder that holds no book is not in
    the commit and is not there after a checkout. See
    [Git does not back up empty folders](#git-does-not-back-up-empty-folders).

### Docker

The container keeps everything under one volume, `/data`
(`/data/shelf`, `/data/store`, `/data/logs`), with the config mounted separately
at `/etc/plainshelf/config.yaml`. Back up the volume, not paths inside the
container:

```sh
docker stop plainshelf

docker run --rm \
  -v plainshelf-data:/data:ro \
  -v "$PWD:/backup" \
  ubuntu:24.04 tar czf /backup/plainshelf-data-2026-09-03.tar.gz -C /data .
```

Then start PlainShelf again with your usual `docker run` command. The documented
one uses `--rm`, so stopping the container removes it; the volume and its
contents are unaffected.

If you mounted your own config file, back up that file from the host as well —
it is not inside the volume.

### macOS desktop app

The desktop app keeps its data in your user config directory, not next to the
shelf:

```text
~/Library/Application Support/PlainShelf/
├─ shelves.json            the shelves you added, and their settings
├─ store/                  the application store
├─ logs/
├─ reading_progress.json   per-device — this machine's reading positions
├─ read_history.json       per-device
└─ reading_stats.json      per-device — reading time
```

The shelf directories themselves are wherever you pointed the app; `shelves.json`
lists them.

```sh
# Quit PlainShelf.app first.
cp -a ~/Library/Application\ Support/PlainShelf \
      ~/Backups/PlainShelf-appdata-2026-09-03
cp -a /path/to/your/shelf ~/Backups/shelf-2026-09-03
```

Unlike the server and Docker cases, this one **does** capture your reading
progress: the desktop app stores those three documents as JSON files in that
directory rather than in the WebView's storage, precisely so they survive.

### Git does not back up empty folders

Git tracks files, not directories, so a [folder](concepts/folders.md) that holds
no book — one you created ahead of time, or one whose books you have since moved
out — is not in the commit and is not there after a checkout. Books themselves
are directories full of files and come back intact; what a Git backup loses is
the shape of the shelf around them.

PlainShelf hits the same property internally, which is why it records the folder
list in its own right instead of deriving it from the books: an empty folder
holds no book, so nothing can rebuild it from the library.

Two ways to live with that:

- **Accept it,** and re-create the empty folders by hand after a restore. They
  are only directories, so `mkdir` under `books/` or the app's own folder
  creation is enough. [Restoring a shelf](#restoring-a-shelf) below says how to
  spot which ones are gone.
- **Put a `.gitkeep` (or any placeholder file) in each folder you want
  preserved.** Git then has a file to track and keeps the directory.
  PlainShelf's scanners look only at directories under `books/`, so such a file
  never shows up as a folder or a book — but it is still a file you did not put
  in your library, and while it is there PlainShelf refuses to delete that
  folder, reporting `cannot delete non-empty folder`; remove the file first.
  PlainShelf neither creates nor removes these files, and does not plan to:
  `books/` holds your files, not the app's.

Use `cp -a` or `rsync` when you want a backup with none of these caveats.

---

## Restoring

### Restoring a shelf

1. **Stop the server or desktop app.** The shelf lock is not a substitute for
   stopping the process, and the application store cannot be opened by two
   processes at once — see [When a restore
   fails](#when-a-restore-fails-to-start).
2. **Move the current directories aside rather than deleting them.** If the
   restore turns out to be the wrong backup, you still have what you had.
3. **Restore the shelf directory, and the application store if you backed it
   up.**
4. **Restore the config file** if you are rebuilding on a new machine, and check
   that its `lib_root` and `store_path` point at where you just put things.
5. **Start PlainShelf.**

```sh
# server
mv /path/to/shelf /path/to/shelf.old
mv /path/to/store /path/to/store.old
cp -a /path/to/backup/shelf-2026-09-03 /path/to/shelf
cp -a /path/to/backup/store-2026-09-03 /path/to/store
./plainshelf-srv -conf config.yaml
```

```sh
# docker — replaces the volume's contents; take the backup above first
docker stop plainshelf
docker run --rm \
  -v plainshelf-data:/data \
  -v "$PWD:/backup" \
  ubuntu:24.04 sh -c 'find /data -mindepth 1 -delete && tar xzf /backup/plainshelf-data-2026-09-03.tar.gz -C /data'
# then start PlainShelf again with your usual `docker run` command
```

```sh
# macOS desktop — quit PlainShelf.app first
mv ~/Library/Application\ Support/PlainShelf \
   ~/Library/Application\ Support/PlainShelf.old
cp -a ~/Backups/PlainShelf-appdata-2026-09-03 \
      ~/Library/Application\ Support/PlainShelf
mv /path/to/your/shelf /path/to/your/shelf.old
cp -a ~/Backups/shelf-2026-09-03 /path/to/your/shelf
```

`cp -a src dst` copies *into* `dst` when `dst` already exists, which is why each
snippet moves the current directory aside first.

You can skip `app/library.lock`, `app/tmp/`, `app/scan-cache.json`, and
`app/book-cache-*.json` when restoring, or drop `app/` entirely; the server
recreates all of it on the next startup. Restore `books/` and `trash/` in full:
both hold books, and a restored `.trash/` from an older backup is renamed to
`trash/` on the next start.

Book IDs, folders, and the trash come back exactly as they were — an ID is a
value inside `book.json`, not something derived from a path, so a restore cannot
change one. Everything keyed on a book ID, including reading progress on a
device that still has it, matches up again.

If you restored from a Git checkout rather than a file copy, check the folder
tree before you start filing books again: every folder that was empty at commit
time is missing, and no error says so — the library simply comes back one or
more folders shallower. Compare the folder list in the app (or
`find books/ -type d`) with what you expect, and re-create the ones that are
gone. No book can go missing this way: a folder that still holds a book holds
files, so Git kept it.

### Restoring reading progress

Reading progress is not in the shelf backup, so it is restored separately and
only on the device that recorded it:

- **Desktop:** it is in the app data directory you restored above
  (`reading_progress.json`, `read_history.json`, `reading_stats.json`). Nothing
  else to do.
- **Browser:** it is in that browser profile's local storage for the origin you
  open PlainShelf on. Restoring a browser profile backup restores it. Clearing
  site data for that origin destroys it, and no server-side restore brings it
  back.
- **Android:** progress is per-book on the device; reinstalling the app without
  a device backup loses it.

### When a restore fails to start

If the old process is still running, the new one refuses to start rather than
opening the store twice:

```text
Error starting plainshelf-srv: … store.New: Cannot acquire directory lock on
"/path/to/store". Another process is using this Badger database.
```

That is the correct behavior, not a corrupted backup. Stop the other process —
including a container you thought you had stopped — and start again.

### Verifying a backup before you rely on it

You do not have to overwrite your live shelf to find out whether a backup is
good. Point a second PlainShelf at the backup with `read_only: true` and look at
it:

```yaml
app_conf:
  shelves:
    - id: backup_check
      name: Backup check
      lib_root: /path/to/backup/shelf-2026-09-03
      read_only: true
  store_path: /tmp/plainshelf-backup-check-store
```

`read_only` turns off every write a shelf normally takes at startup — the lock
file, the scan cache, the exported book cache, the legacy trash migration — so
the backup is read exactly as it was found and comes out byte-identical. Give it
a throwaway `store_path` so you do not open your real store at the same time.
Then check the books, the folder tree, and the trash before you rely on the
backup. See [Opening a shelf
read-only](concepts/shelf-cache-and-io.md#opening-a-shelf-read-only).

---

## Reading progress, history, and time

Since v0.9.0 these three records live on the device that did the reading, not on
the server. That is what makes them invisible to every server-side backup, so it
is worth knowing exactly where they are:

| Client | Where | Notes |
|---|---|---|
| Browser | `localStorage` keys `plainshelf.readingProgress`, `plainshelf.readHistory`, `plainshelf.readingStats` | Per browser profile **and per origin**. |
| Desktop | `reading_progress.json`, `read_history.json`, `reading_stats.json` in the app data directory | Deliberately files, not WebView storage, so clearing WebView data does not take them. |
| Android | On-device, per book | Not covered by a shelf backup. |

Two consequences:

- **An origin is a scheme, host, and port.** The browser scopes storage to the
  origin the UI was served from, and PlainShelf keys each record by the shelf
  within it. Move the server to a different port or hostname and the browser
  sees a different origin: the library is unchanged, and that browser's reading
  positions start from zero. Nothing was lost — they are still under the old
  origin — but nothing follows you either.
- **Backing up a browser profile is the only way to back up web reading
  progress.** PlainShelf has no export for these records today.

---

## Rolling back to an older release

The reason to take a backup before upgrading is that an upgrade can write a
format the previous release will not touch. When that happens, a book written by
the newer build becomes read-only under the older one, and the API answers
`409 Conflict` rather than modifying it — the refusal is the protection.

To go back to an older release:

1. Stop PlainShelf.
2. Install the older release.
3. Restore the shelf from the backup taken **before** the upgrade, following
   [Restoring a shelf](#restoring-a-shelf).
4. Start the older release.

Restoring is required, not optional: downgrading the binary alone leaves the
newer on-disk data in place, and the older build will refuse to write it. If
only one or two books were touched, restoring just those books' folders from the
backup is enough.

See [When PlainShelf refuses to
write](concepts/data-format-versioning.md#when-plainshelf-refuses-to-write) for
what the refusal looks like, and
[Data Format Versioning](concepts/data-format-versioning.md) for which formats
have changed.

---

## Verified

The backup and restore procedures on this page were run end to end on the `dev`
branch at commit `dae5b8f`, after v0.10.0, on Linux, in the server
configuration. What was checked:

- A full backup (`cp -a` of the shelf and store with the server stopped),
  followed by deleting both directories and restoring them, returned an
  identical library: the same book IDs, titles, folder placement, and source
  content; the folder tree including a folder that held no book; the trashed
  book with its `original_path`; and both stored settings.
- Restoring the shelf alone, without the store, returned the same books, IDs,
  folders and trash, and silently reverted the two stored settings to their
  config and built-in defaults — the loss described in
  [What a shelf-only backup loses](#what-a-shelf-only-backup-loses).
- Restoring without `app/` returned the identical library and rebuilt `app/` at
  startup.
- Web reading progress was recorded in a browser, confirmed absent after a full
  server restore in a fresh browser profile, and restored to the same offset by
  restoring that profile's local storage.
- Opening a backup with `read_only: true` listed every book, its ID, and the
  trashed book, and left the backup byte-identical.

Docker and macOS desktop commands follow the same procedure against the paths
those installations use; they were not executed on this run.
