# Configure a local shelf file source

A local shelf keeps the library on a disk directly attached to the machine
running PlainShelf — an internal drive, an external USB drive, a local data
volume. This is the setup PlainShelf works best on.

Use it for libraries you care about, unless you specifically need an
experimental network-backed shelf.

## Before you start

You need:

- A directory on a local filesystem that the PlainShelf process can read and
  write.
- Enough free disk space for imported books, cover images, metadata, and
  temporary files.
- One PlainShelf server process using the shelf at a time.

PlainShelf stores user-owned library data and rebuildable runtime state under
the shelf root. See the [data model](concepts/data-model.md) for the full shelf
layout.

## 1. Create a shelf directory

Create a directory for the shelf and make sure the user running PlainShelf owns
it or can write to it:

```bash
mkdir -p ./workspace/shelf
```

You can place the directory anywhere that is stable across restarts. For
example:

- A project-local development directory such as `./workspace/shelf`.
- A user data directory such as `$HOME/PlainShelf/shelf`.
- A persistent server directory such as `/var/lib/plainshelf/shelf`.
- A Docker volume mounted at `/data/shelf`.

### On the desktop app

The desktop app has no config file, so a shelf is created in **Settings →
Shelves → Create shelf**. The dialog asks for a name and for where the shelf
goes; on **Create a new folder**, its default, the app creates the shelf in its
own data directory, under `shelves/<shelf id>` — beside the `shelves.json` that
records it:

| Platform | Default shelf directory |
|---|---|
| macOS | `~/Library/Application Support/PlainShelf/shelves/<shelf id>` |
| Linux | `~/.config/PlainShelf/shelves/<shelf id>` |
| Windows | `%AppData%\PlainShelf\shelves\<shelf id>` |

The dialog shows the exact path before you create the shelf, and that path is
what is recorded. To put the shelf somewhere else, such as an external drive or
a directory you back up separately, choose **Use a folder I already have** and
type the **Folder path** in or pick it with **Browse**; a directory you chose is
kept even if you then change the name.

## 2. Configure `lib_root`

Edit your PlainShelf config file and set the shelf `lib_root` to the local
directory:

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

For production or service deployments, prefer an absolute path so startup
behavior does not depend on the working directory.

## 3. Configure the application store

PlainShelf also uses `app_conf.store_path` for server settings outside the shelf
itself:

```yaml
app_conf:
  store_path: ./workspace/store
```

Keep `store_path` on reliable local storage. It does not replace `lib_root`;
both paths should be configured and persisted. Reading progress, history, and
time are stored by each client rather than here.

For every other key the config file accepts, see the [Configuration
reference](reference/configuration.md).

## 4. Run PlainShelf

Start PlainShelf with the config file that points to your local shelf:

```bash
go run ../cmd/plainshelf-srv/main.go -conf config.yaml
```

Then open the web UI and import or create books as usual.

## Docker notes

The default container config stores shelf data at `/data/shelf` and application
store data at `/data/store`. Use a named volume or bind mount so data persists
after the container exits:

```bash
docker run --rm \
  --name plainshelf \
  -p 127.0.0.1:20000:20000 \
  -v plainshelf-data:/data \
  plainshelf
```

If you mount a custom config file, make sure its `lib_root` and `store_path`
match paths that exist inside the container.

## Backup guidance

The shelf is plain files, so backing it up is a directory copy:

1. Stop PlainShelf or make sure no imports, edits, moves, or deletes are
   running.
2. Back up the entire `lib_root` directory.
3. Back up `store_path` if you want to preserve server settings. Back up client
   profiles separately for reading progress, history, and time.

Do not back up only individual `.bookpkg` folders unless you intentionally want
a partial library backup.

[Backup and restore](backup-and-restore.md) has the full procedure for each
installation, what a shelf-only copy leaves behind, and how to restore.

## Open a shelf read-only

To browse a shelf without writing anything to it (a restored backup, a read-only
mount, an archived snapshot), set `read_only` on that shelf:

```yaml
app_conf:
  shelves:
    - id: archive_shelf
      name: Archive (read-only)
      lib_root: /mnt/backup/plainshelf-shelf
      read_only: true
```

PlainShelf then creates no folders, writes no lock or cache files under `app/`,
and refuses every edit with an error instead of touching the shelf. `lib_root`
must already exist. See [Shelf cache and disk
I/O](concepts/shelf-cache-and-io.md#opening-a-shelf-read-only) for exactly what
is skipped.

While you are browsing a read-only shelf, the app hides the controls that would
write to it (importing, editing metadata, sources and covers, creating and
moving folders, deleting, and restoring from the trash) and shows a banner
saying the shelf is read-only. Browsing, reading, searching and rescanning stay
available. Switching to a writable shelf brings those controls back without a
reload; the shelves you can write to are unaffected by the ones you cannot.

To do the same for every shelf at once, set `read_only` next to `shelves`
instead:

```yaml
app_conf:
  read_only: true
  shelves:
    - id: archive_shelf
      lib_root: /mnt/backup/plainshelf-shelf
```

PlainShelf then answers every write request with HTTP 403 and opens each shelf,
including one added after startup, as if it carried `read_only: true`.
Rescanning a shelf is still allowed, because it only walks the shelf and
rebuilds the in-memory cache.

### From the desktop app

The desktop app keeps its shelves in its own data directory rather than in a
config file, so the same setting is a **Read-only shelf** toggle in **Settings →
Shelves** — in the dialog that adds a shelf, and in **Modify** for a shelf that
already exists. Switching it closes that shelf and opens it again in the new
mode, without restarting the app.

The toggle works in both directions, including on a shelf that is already
read-only: what `read_only` stops is writing to the *shelf*, and the desktop
shelf list lives outside every shelf.

## Share shelves on your local network (desktop app)

**Settings → Shelves → Share on local network** exports the shelves you tick,
read-only, to other devices on the same network. It is the server's
[`dav_export`](reference/configuration.md#dav_export) behind a switch:

- Off by default, with no shelves ticked on a fresh setup. Turning it off
  closes the listener immediately, including active transfers.
- Turned on, no shelf is shared until you tick it. An unticked shelf is 404
  on the export, the same as one that does not exist.
- The page shows the address (`http://<LAN IP>:20001/dav/`, one shelf per
  `/dav/<shelf id>/`), the token every request must carry, and a QR code
  holding both plus every ticked shelf (see [the QR code](#the-qr-code)).
  **Replace token** revokes the old one at once.
- It binds one detected local LAN IPv4 address and refuses `0.0.0.0`; with more than one network
  the page lets you choose which. If that address is gone at the next start,
  the switch stays on and the page says why it is not running.
- The switch, the ticked shelves and the token are kept in the app's data
  directory (`lan_share.json`, `lan_share.token`), not in any shelf, and
  survive a restart. The port is `20001` unless `lan_share.json` sets `port`.

The URL reveals the computer's network address and shelf identifiers; the token
and QR payload grant read access to the selected shelves. Treat the connection
details as sensitive: keep them out of public screenshots, logs and messages.
If the token or QR is disclosed, turn sharing off or replace the token and
update the receiving devices. Replacing it affects subsequent requests; it does
not erase files a device has already downloaded. Saved enabled settings can
start the listener again on restart, so switch sharing off when finished.

For the alternatives, scope and security tradeoffs, see the
[desktop LAN sharing decision record](deployment-and-threat-model.md#desktop-lan-sharing-decision-record).

### The QR code

The QR code is JSON, and its shape is a promise between devices:

```json
{
  "v": 1,
  "url": "http://192.168.1.20:20001/dav/",
  "token": "…",
  "shelves": [
    {"id": "default_shelf", "name": "Default Shelf", "url": "http://192.168.1.20:20001/dav/default_shelf/"}
  ]
}
```

- `v` is the format version. A device reading the code must refuse any value
  other than `1` instead of guessing at the fields.
- `shelves` lists only the ticked shelves, in the order of the shelf list, and
  is `[]` when none is ticked. Each `url` opens that shelf on its own, so a
  scanner needs nothing else; the export root `url` itself lists nothing (404).
- The token is only ever in `token`, never in a URL. Send it as
  `Authorization: Bearer <token>`.
- Ticking or unticking a shelf, replacing the token, or changing the address
  changes the code; scan it again afterwards.
- A QR code holds about 2 KB, roughly 18 shelves with short names; past that
  the page shows an error instead of the code. The address and token are still
  shown, so add the shelves by hand, or share fewer at a time.

#### Import on another desktop

In **Settings → Shelves → Create shelf**, choose **Paste a LAN share QR
payload**, paste the QR's JSON text, then click **Preview shelves**. Check the
names and addresses before confirming with **Import shelves**. Pasting or previewing
does not connect, save entries or store the token. This accepts text, not images.

The shared Go parser (`internal/lanshare`) rejects unknown versions, missing
fields and non-HTTP(S) addresses before any entry is created. URLs must have no
userinfo, query or fragment, and shelf URLs must use the share root's origin.
An empty `shelves: []` is valid but offers nothing to import.

After confirmation, each shelf becomes a read-only `plainshelf` source using
its supplied name and URL. Each source is checked before saving. A failed
connection or refused token leaves that source unsaved; other sources are
still attempted, and the dialog shows each result. Paste and confirm again to
retry failures. An existing PlainShelf source with the same URL (ignoring a
trailing slash) is skipped, including on retries: its name and token are never
replaced. Name collisions receive the usual new shelf ID suffix.

Imported tokens use the existing OS keychain flow, never `shelves.json`, logs
or URLs. The pasted JSON is cleared after import or closing the dialog.
The share publisher's existing `lan_share.token` storage is separate from
imported credentials. For Android, use **Add shelf → Scan a LAN share QR code**,
then preview and confirm the shelves. The Android reader uses this same parser and import
semantics, with tokens in Android Keystore; see
[Android QR import](development/android.md#scan-a-lan-share-qr-code) for camera
permissions, results and device verification.

The first time it is turned on, the operating system's firewall may ask whether
PlainShelf may accept incoming connections; allow it on private networks only.
The export is plain HTTP, so share only on a network you trust; see
[the threat model](deployment-and-threat-model.md#the-read-only-webdav-export).

## Read a shelf from another device (desktop app)

**Settings → Shelves → Create shelf → Read a shelf from another device** adds a
shelf that lives elsewhere, read-only: another PlainShelf's local-network share
(above), the server's [`dav_export`](reference/configuration.md#dav_export), or
any WebDAV server holding a shelf directory, such as a NAS.

- Enter the URL of the shelf folder, the one that holds `books/`. For another
  PlainShelf it is `http://<address>:20001/dav/<shelf id>/`, as its share page
  shows.
- **Another PlainShelf** needs the share's token. **WebDAV** takes an optional
  account and password or token.
- Creating the shelf checks the source first: a refused token and an address
  that cannot be reached are reported differently, and nothing is saved.
- The token is stored in the operating system's keychain (Keychain on macOS,
  Credential Manager on Windows, the Secret Service on Linux), never in
  `shelves.json`. Removing the shelf removes it from the keychain.
- If the keychain cannot answer at startup, for example because it is locked,
  that shelf is skipped and the app says so; your other shelves open as usual.
  It remains visible in **Settings → Shelves**, with its source, URL and a safe
  failure summary. Unlock the keychain or restore the connection, then choose
  **Retry** to open it without restarting, or remove it there (including its
  keychain secret). Virtual shelves have no **Open folder** action or read-only
  toggle.

The same is available in a server's config file as a
[`source` block](concepts/shelf-sources.md#the-source-block).

## Troubleshooting

### PlainShelf cannot create or update books

Confirm the user running PlainShelf can create, rename, and delete files in
`lib_root`:

```bash
touch ./workspace/shelf/.write-test
mv ./workspace/shelf/.write-test ./workspace/shelf/.write-test-renamed
rm ./workspace/shelf/.write-test-renamed
```

### Books disappear after restart

Check whether `lib_root` was configured as a relative path and PlainShelf was
started from a different working directory. Use an absolute path for service
deployments.

### Docker data disappears after container exit

Make sure `/data` or both `/data/shelf` and `/data/store` are backed by a Docker
volume or bind mount. Container-local files are removed when the container is
deleted.
