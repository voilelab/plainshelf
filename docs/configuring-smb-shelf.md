# Configure an SMB shelf file source

PlainShelf reads and writes shelf data through the local filesystem. To store a shelf on an SMB/CIFS share, mount the share on the host first, then point the shelf `lib_root` at the mounted directory.

!!! warning "Use a trusted private network"
    SMB is a network filesystem, not a public application protocol. Only mount shares from networks and servers you trust, and do not expose PlainShelf itself to untrusted networks without an authentication boundary.

!!! warning "Experimental SMB support"
    SMB-backed shelves are currently **experimental**. PlainShelf can work with an SMB share that is already mounted as a local filesystem path, but SMB-specific reliability work is still in progress. In particular, transient network errors during move/delete operations and cached book-list behavior while the share is offline still need additional handling. For the most reliable setup today, use a local filesystem shelf as the primary source.

## Before you start

You need:

- An SMB share that the PlainShelf process can read and write.
- Acceptance that this source type is experimental; prefer a local shelf for important libraries.
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

The important fields for SMB shelves are:

| Field | Purpose | SMB guidance |
|---|---|---|
| `lib_root` | Path to the shelf directory. | Use the local mount path, not an `smb://` or `//server/share` URL. |
| `scan_interval` | How often PlainShelf performs a full on-disk scan. | Increase this on large or high-latency shares to reduce network I/O. |
| `book_check_interval` | How often list operations check per-book metadata freshness. | Increase this when browsing feels slow over SMB. |
| `lock_timeout` | Maximum time to wait for the shelf file lock. | Keep a finite timeout so unreliable SMB locking cannot hang indefinitely. |
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

### Requests time out while opening large books

Increase `server_conf.write_timeout`, and verify the SMB mount is healthy from the host with normal file copy commands.

### Lock errors or hangs

Make sure only one PlainShelf server uses the shelf. Keep `lock_timeout` set to a finite duration such as `30s` so lock problems fail with an error instead of hanging forever.
