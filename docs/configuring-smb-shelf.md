# Configure an SMB shelf file source

PlainShelf reads and writes shelf data through the local filesystem. To store a shelf on an SMB/CIFS share, mount the share on the host first, then point the shelf `lib_root` at the mounted directory.

!!! warning "Use a trusted private network"
    SMB is a network filesystem, not a public application protocol. Only mount shares from networks and servers you trust, and do not expose PlainShelf itself to untrusted networks without an authentication boundary.

!!! warning "SMB/NAS support is best-effort"
    PlainShelf works with an SMB/CIFS or NAS share that is already mounted as a local filesystem path, but a network mount is a slower and less predictable filesystem than a local disk. The settings on this page reduce that cost; they do not turn a shared network location into a coordinated multi-writer store. Keep a library you cannot afford to lose an edit from on a local filesystem shelf, with a current backup, and read [Scope](#scope) before you commit a library to a share.

## Scope

An SMB or NAS shelf is a local filesystem shelf whose files happen to sit on a network mount. PlainShelf reads and writes it exactly as it does a local shelf, so everything the on-disk format guarantees still holds — see [Data Format Versioning](concepts/data-format-versioning.md) for what the format promises across upgrades. What a network mount does *not* add is any coordination or change notification on top of that filesystem, so four things are not guaranteed, whatever you set the intervals to:

- **One PlainShelf writes a shelf at a time.** Nothing coordinates two PlainShelf server processes that open the same shelf; SMB file locking is unreliable enough that running two against one shelf can corrupt it (see [Before you start](#before-you-start)). And even within one server, two edits to the same book are last-writer-wins: the second write replaces the first, which is lost without warning. See [Concurrent change handling](known-issue.md#concurrent-change-handling).
- **How soon an external change is seen.** A book added, moved, or removed on the share by anything other than this PlainShelf — another machine, a file manager, a sync client — appears on a timer, not at once, because a network mount sends no change notification PlainShelf could wait on. **Update book list** on the library toolbar is what forces a walk now; shortening the intervals below only narrows the window, it does not close it. See [Rescanning on demand](concepts/shelf-cache-and-io.md#rescanning-on-demand).
- **An edit that leaves size and modification time unchanged.** Whether a cached book is stale is judged from its `book.json` size and modification time, so an external edit that leaves both unchanged is not noticed — and a sync client that preserves or normalizes timestamps makes that realistic rather than rare. **Update book list** does not lift this one: the walk finds the file, sees the same size and time, and keeps what it had. See [Shelf cache limitations](known-issue.md#shelf-cache-limitations).
- **Behavior while the share is flaky or offline.** A move or delete can fail partway on a dropped connection, and the book list can still be served from the in-memory cache while the mount is unreachable. PlainShelf reports the error rather than corrupting the shelf, but it does not hide an unreliable network from you.

None of these is a defect being worked toward a fix: they follow from a shelf being plain files on a filesystem with nothing coordinating access to them. Decide whether they are acceptable for a given library before you move it to a share.

## Before you start

You need:

- An SMB share that the PlainShelf process can read and write.
- Acceptance that SMB/NAS support is best-effort in the sense set out under [Scope](#scope); keep an important library on a local shelf with a current backup.
- A stable mount point on the machine that runs PlainShelf.
- A directory on the share dedicated to one PlainShelf shelf.
- Exclusive access from one PlainShelf server process at a time. Running multiple servers against the same shelf can corrupt data.

PlainShelf creates and updates regular files such as `books/`, `app/library.lock`, book metadata, source text, and temporary files inside `lib_root`. See the [data model](concepts/data-model.md) for the shelf layout.

## 1. Prepare the SMB mount

Mount the share outside PlainShelf using your operating system or infrastructure tooling, then verify that the PlainShelf process sees it as a normal local directory. PlainShelf does not mount SMB shares itself and does not accept `smb://` URLs as shelf roots.

Keep the mount stable across restarts, and choose a mount path that is available before PlainShelf starts. For Docker deployments, mount the SMB share on the Docker host first, then bind-mount that host path into the container.

## 2. Create a shelf directory on the share

Create a directory that PlainShelf can own and write to:

```bash
mkdir -p /mnt/plainshelf/default-shelf
```

If you are migrating an existing shelf, stop PlainShelf first, copy the entire shelf directory to the share, and preserve the `books/` and `app/` structure.

## 3. Configure `lib_root`

Edit your PlainShelf config file and set the shelf `lib_root` to the mounted SMB directory:

```yaml
app_conf:
  shelves:
    - id: default_shelf
      name: Default Shelf
      lib_root: /mnt/plainshelf/default-shelf
      scan_interval: 10m
      book_check_interval: 5m
      lock_timeout: 30s
```

For the cache behavior behind these settings, see [Shelf Cache and Disk I/O](concepts/shelf-cache-and-io.md).

The important fields for SMB shelves are:

| Field | Purpose | SMB guidance |
|---|---|---|
| `lib_root` | Path to the shelf directory. | Use the local mount path, not an `smb://` or `//server/share` URL. |
| `scan_interval` | How often PlainShelf performs a full on-disk scan. | Increase this on large or high-latency shares to reduce network I/O. |
| `book_check_interval` | How often list operations check per-book metadata freshness. | Increase this when browsing feels slow over SMB. |
| `lock_timeout` | Maximum time to wait for the shelf file lock. | Keep a finite timeout so unreliable SMB locking cannot hang indefinitely. |
| `scan_cache` | Whether a scan may skip listing folders whose modification time has not changed. | Leave it on: it turns most of a scan's directory listings into single stats, which is exactly what SMB round trips make expensive. Turn it off only if the share does not update directory times. |
| `server_conf.write_timeout` | Maximum time for the HTTP server to write a response. | Increase this when large books transfer slowly over the network. |

## 4. Tune server timeouts for slow shares

If books are large or the network is slow, increase `server_conf.write_timeout` so responses are not cut off mid-transfer:

```yaml
server_conf:
  addr: "127.0.0.1:20000"
  read_timeout: 60s
  write_timeout: 300s
```

## 5. Run PlainShelf

Start PlainShelf with the config file that points to the mounted share:

```bash
go run ../cmd/plainshelf-srv/main.go -conf config.yaml
```

Then open the web UI and import or browse books as usual.

## Docker notes

When running PlainShelf in Docker, mount the SMB share on the Docker host first, then bind-mount the mounted directory into the container:

```bash
docker run --rm \
  --name plainshelf \
  -p 127.0.0.1:20000:20000 \
  -v /mnt/plainshelf/default-shelf:/data/shelf \
  -v plainshelf-store:/data/store \
  plainshelf
```

Avoid mounting SMB directly from inside the application container unless you have a specific operational reason. Host-level mounts are easier to monitor, remount, and secure.

## Troubleshooting

### PlainShelf cannot start or says the shelf is not writable

Confirm the user running PlainShelf can create, rename, and delete files in `lib_root`:

```bash
touch /mnt/plainshelf/default-shelf/.write-test
mv /mnt/plainshelf/default-shelf/.write-test /mnt/plainshelf/default-shelf/.write-test-renamed
rm /mnt/plainshelf/default-shelf/.write-test-renamed
```

### Browsing is slow

Increase `scan_interval` and `book_check_interval`. SMB round trips can make frequent metadata checks expensive, especially for large shelves.

### Books copied onto the share never appear

Press **Update book list** first; within `scan_interval` PlainShelf has not looked yet.

If they still do not appear, and appear only after restarting the server, the share may not be updating directory modification times — some gateways do not. Set `scan_cache: off` on the shelf and delete `app/scan-cache.json`. See [Shelf Cache and Disk I/O](concepts/shelf-cache-and-io.md#scan_cache).

### Requests time out while opening large books

Increase `server_conf.write_timeout`, and verify the SMB mount is healthy from the host with normal file copy commands.

### Lock errors or hangs

Make sure only one PlainShelf server uses the shelf. Keep `lock_timeout` set to a finite duration such as `30s` so lock problems fail with an error instead of hanging forever.
