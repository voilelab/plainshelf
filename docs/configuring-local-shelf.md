# Configure a local shelf file source

A local shelf keeps the library on a disk directly attached to the machine running PlainShelf — an internal drive, an external USB drive, a local data volume. This is the setup PlainShelf works best on.

Use it for libraries you care about, unless you specifically need an experimental network-backed shelf.

## Before you start

You need:

- A directory on a local filesystem that the PlainShelf process can read and write.
- Enough free disk space for imported books, cover images, metadata, and temporary files.
- One PlainShelf server process using the shelf at a time.

PlainShelf stores user-owned library data and rebuildable runtime state under the shelf root. See the [data model](concepts/data-model.md) for the full shelf layout.

## 1. Create a shelf directory

Create a directory for the shelf and make sure the user running PlainShelf owns it or can write to it:

```bash
mkdir -p ./workspace/shelf
```

You can place the directory anywhere that is stable across restarts. For example:

- A project-local development directory such as `./workspace/shelf`.
- A user data directory such as `$HOME/PlainShelf/shelf`.
- A persistent server directory such as `/var/lib/plainshelf/shelf`.
- A Docker volume mounted at `/data/shelf`.

### On the desktop app

The desktop app has no config file, so a shelf is created in **Settings → Shelves → Add shelf**. Leave **Directory path** empty and the app creates the shelf in its own data directory, under `shelves/<shelf id>` — beside the `shelves.json` that records it:

| Platform | Default shelf directory |
|---|---|
| macOS | `~/Library/Application Support/PlainShelf/shelves/<shelf id>` |
| Linux | `~/.config/PlainShelf/shelves/<shelf id>` |
| Windows | `%AppData%\PlainShelf\shelves\<shelf id>` |

The dialog shows the exact path before you create the shelf, and that path is what is recorded. To put the shelf somewhere else — an external drive, a directory you back up separately — type it in or pick it with **Browse**; a directory you chose is kept even if you then change the name.

## 2. Configure `lib_root`

Edit your PlainShelf config file and set the shelf `lib_root` to the local directory:

```yaml
app_conf:
  shelves:
    - id: default_shelf
      name: Default Shelf
      lib_root: ./workspace/shelf
```

`lib_root` may be relative or absolute:

| Path type | Behavior |
|---|---|
| Relative path | Resolved relative to the process working directory when PlainShelf starts. |
| Absolute path | Uses that exact path regardless of the process working directory. |

For production or service deployments, prefer an absolute path so startup behavior does not depend on the working directory.

## 3. Configure the application store

PlainShelf also uses `app_conf.store_path` for server settings outside the shelf itself:

```yaml
app_conf:
  store_path: ./workspace/store
```

Keep `store_path` on reliable local storage. It does not replace `lib_root`; both paths should be configured and persisted. Reading progress, history, and time are stored by each client rather than here.

## 4. Run PlainShelf

Start PlainShelf with the config file that points to your local shelf:

```bash
go run ../cmd/plainshelf-srv/main.go -conf config.yaml
```

Then open the web UI and import or create books as usual.

## Docker notes

The default container config stores shelf data at `/data/shelf` and application store data at `/data/store`. Use a named volume or bind mount so data persists after the container exits:

```bash
docker run --rm \
  --name plainshelf \
  -p 127.0.0.1:20000:20000 \
  -v plainshelf-data:/data \
  plainshelf
```

If you mount a custom config file, make sure its `lib_root` and `store_path` match paths that exist inside the container.

## Backup guidance

The shelf is plain files, so backing it up is a directory copy:

1. Stop PlainShelf or make sure no imports, edits, moves, or deletes are running.
2. Back up the entire `lib_root` directory.
3. Back up `store_path` if you want to preserve server settings. Back up client profiles separately for reading progress, history, and time.

Do not back up only individual `.bookpkg` folders unless you intentionally want a partial library backup.

## Open a shelf read-only

To browse a shelf without writing anything to it — a restored backup, a read-only mount, an archived snapshot — set `read_only` on that shelf:

```yaml
app_conf:
  shelves:
    - id: archive_shelf
      name: Archive (read-only)
      lib_root: /mnt/backup/plainshelf-shelf
      read_only: true
```

PlainShelf then creates no folders, writes no lock or cache files under `app/`, and refuses every edit with an error instead of touching the shelf. `lib_root` must already exist. See [Shelf Cache and Disk I/O](concepts/shelf-cache-and-io.md#opening-a-shelf-read-only) for exactly what is skipped.

While you are browsing a read-only shelf, the app hides the controls that would write to it — importing, editing metadata, sources and covers, creating and moving folders, deleting, and restoring from the trash — and shows a banner saying the shelf is read-only. Browsing, reading, searching and rescanning stay available. Switching to a writable shelf brings those controls back without a reload; the shelves you can write to are unaffected by the ones you cannot.

To do the same for every shelf at once, set `read_only` next to `shelves` instead:

```yaml
app_conf:
  read_only: true
  shelves:
    - id: archive_shelf
      lib_root: /mnt/backup/plainshelf-shelf
```

PlainShelf then answers every write request with HTTP 403 and opens each shelf — including one added after startup — as if it carried `read_only: true`. Rescanning a shelf is still allowed, because it only walks the shelf and rebuilds the in-memory cache.

### From the desktop app

The desktop app keeps its shelves in its own data directory rather than in a config file, so the same setting is a **Read-only shelf** toggle in **Settings → Shelves** — in the dialog that adds a shelf, and in **Modify** for a shelf that already exists. Switching it closes that shelf and opens it again in the new mode, without restarting the app.

The toggle works in both directions, including on a shelf that is already read-only: what `read_only` stops is writing to the *shelf*, and the desktop shelf list lives outside every shelf.

## Troubleshooting

### PlainShelf cannot create or update books

Confirm the user running PlainShelf can create, rename, and delete files in `lib_root`:

```bash
touch ./workspace/shelf/.write-test
mv ./workspace/shelf/.write-test ./workspace/shelf/.write-test-renamed
rm ./workspace/shelf/.write-test-renamed
```

### Books disappear after restart

Check whether `lib_root` was configured as a relative path and PlainShelf was started from a different working directory. Use an absolute path for service deployments.

### Docker data disappears after container exit

Make sure `/data` or both `/data/shelf` and `/data/store` are backed by a Docker volume or bind mount. Container-local files are removed when the container is deleted.
