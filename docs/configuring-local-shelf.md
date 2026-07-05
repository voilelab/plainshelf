# Configure a local shelf file source

PlainShelf is designed to work best with a local filesystem shelf. A local shelf keeps the library on a disk that is directly attached to the machine running PlainShelf, such as an internal drive, external USB drive, or local data volume.

Use this setup for important libraries unless you specifically need an experimental network-backed shelf.

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

PlainShelf also uses `app_conf.store_path` for application-level state outside the shelf itself:

```yaml
app_conf:
  store_path: ./workspace/store
```

Keep `store_path` on reliable local storage. It does not replace `lib_root`; both paths should be configured and persisted.

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

Because PlainShelf is filesystem-first, backing up a local shelf is straightforward:

1. Stop PlainShelf or make sure no imports, edits, moves, or deletes are running.
2. Back up the entire `lib_root` directory.
3. Back up `store_path` if you want to preserve application-level state.

Do not back up only individual `.bookpkg` folders unless you intentionally want a partial library backup.

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
