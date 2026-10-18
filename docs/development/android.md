# Android Development

The Android client is experimental. It wraps the Vue frontend with Capacitor and
reads a library from one of three places: a separately running PlainShelf
server, a shelf folder held on pCloud, or a [local shelf](#local-shelves) kept
inside the app. Downloaded books, covers, illustrations,
reading progress, read history, and reading time are stored locally in the app's
private storage
(`Directory.Data`, no runtime permission required) and never leave the device.
The one exception is **Export file** on a book's page: it writes a copy of the
book's text into the shared `Documents/PlainShelf/` folder so it can be opened
from the Files app or handed to another app. On Android 11+ the Filesystem
plugin reaches its own files in Documents with no permission; on Android 10 and
below it needs both `READ_EXTERNAL_STORAGE` and `WRITE_EXTERNAL_STORAGE`
(declared with `maxSdkVersion="29"`).

On a server or pCloud shelf the app is **read-only**: it browses, reads, and
downloads books for offline use, but it never modifies the library and issues
no write requests at all. Editing pages (metadata, sources, trash, and the
maintenance views) are not reachable there, and write requests are rejected on
the device before they are sent. A local shelf is the exception: it is edited
in place, as on the desktop.

Reading a book from a server or pCloud requires it to be downloaded first, so every book
in the library list carries a marker saying whether it is *Not downloaded* or
*Downloaded*. Long-pressing a book starts a selection, and the bar at the bottom
downloads everything selected in one go; the markers update when it finishes.

## The shelf list

The device keeps a list of shelves under **Settings → Shelves**, and each
entry carries its own source type. One list can hold a couple of PlainShelf
servers, a couple of pCloud folders and local shelves at once:

| Source type | Where the library lives | What the entry stores |
|---|---|---|
| PlainShelf server | A server you run, reachable over the network | Its URL, which shelf on it, and an access token only when the server sets `protect_read` |
| pCloud | A shelf folder in your pCloud account | Your own pCloud app key, the region learned during authorization, and the folder path |
| Local shelf | The app's own private storage | Its name and the embedded server's shelf id |
| Other device | Another PlainShelf's [LAN share](#share-local-shelves-on-the-local-network), or any WebDAV server such as a NAS | The source kind, the shelf folder's URL and, for WebDAV, the account; the token or password is kept in Keystore-backed storage |

One entry is one shelf. Two shelves on the same server are two entries, added
the same way. Access tokens are kept in Android Keystore-backed storage, one per
entry, so two entries never share credentials.

Each entry's downloads and reading data are filed under where its library
lives, so two entries never read each other's books. For pCloud that includes
which account authorized it, since two accounts in the same region can both
hold a folder of the same name. An entry authorized by an older build has no
account recorded and keeps the filing it already had; re-authorizing it records
one, which means its existing downloads are left behind — the same thing that
happens when you point an entry at a different server or folder.

The pCloud source needs no server at all, which is the point of it: a phone
cannot mount cloud storage the way a host can, so a server keeping its shelf on
pCloud through an rclone mount is no help to the phone.

**Switching shelves.** Exactly one entry is in use at a time. Pick another from
the shelf dropdown in the sidebar, or from the list itself. Switching restarts
the app, because what the app reads from is decided once at startup.

**Removing a shelf** deletes the books downloaded from it, freeing that space
on the device. Reading progress, read history, and reading time are kept —
they are the only records that cannot be rebuilt from the library, so adding
the same shelf back picks up where you left off. Removing a *local* shelf
deletes the shelf itself with every book in it, after a confirmation that says
so.

Server, pCloud and other-device entries are read-only. The rest of this page
covers the server source first, then [Read a shelf from pCloud](#read-a-shelf-from-pcloud),
[Read a shelf from another device](#read-a-shelf-from-another-device) and
[Local shelves](#local-shelves).

## Prerequisites

- The base [local development](setup.md) toolchain
- Android SDK, with an NDK in `ANDROID_NDK_HOME` for the embedded server
- JDK 21 (Capacitor 8's Android libraries target Java 21)
- Android Studio or an Android emulator/device configured for `adb`

The `just` Android recipes use JDK 21 from `JAVA_HOME` or `STUDIO_JDK` when
configured. On macOS they also detect the JDK bundled with Android Studio, and
on Linux they detect standard Android Studio installations under `/opt` or
`/usr/local`.

## Build the app

The native project is committed under `frontend/android`. Build and synchronize
the web assets with:

```bash
npm --prefix frontend ci
just build-mobile-android
```

`build-mobile-android` first runs `just android-lib`, which binds the embedded
server with `scripts/build-android-lib.sh` and copies `plainshelf.aar` into
`frontend/android/app/libs/`. Gradle refuses to build without it; a direct
Gradle build needs that step run once, and again after `mobile/` or the server
code changes. CI's *Android build* job takes the AAR from the *Android Go
library* job instead of binding it a second time.

The debug APK is written to:

```text
frontend/android/app/build/outputs/apk/debug/app-debug.apk
```

`just build-mobile-android` derives `versionName` from Git and uses the current
commit count as `versionCode`. Direct Gradle builds can override both values:

```bash
cd frontend/android
PLAINSHELF_VERSION_NAME=0.8.0-beta.1 \
PLAINSHELF_VERSION_CODE=8001 \
./gradlew assembleDebug
```

Without overrides, Gradle uses `0.0.0-dev` and version code `1`. Android
release artifacts and signing are not part of the repository release workflow.

Open the project in Android Studio with:

```bash
just open-mobile-android
```

If the native project is ever regenerated from scratch, run
`just mobile-add-android` once before building.

## Run with a local emulator

With a server shelf.

```bash
just run-android-app
```

This starts the local server, waits for it to become healthy, boots an available
emulator when needed, builds and launches the app, and prints the ephemeral
access token. In the emulator, connect to `http://10.0.2.2:20000`; Android maps
`10.0.2.2` to the host loopback address. The printed token only needs to be
entered into the shelf entry when the server sets `protect_read: true`.

## Connect a physical device

With a server shelf. The server must listen on a LAN-reachable address instead
of `127.0.0.1`. From the phone, verify that `http://<server-ip>:20000/health`
returns `1`, then add a shelf with that server URL under
**Settings → Shelves**.

Browsing, reading, and refreshing the book list require no access token — the
refresh walks the shelf and writes nothing, so the server treats it as a read.
A token is needed only when the server sets `protect_read: true`, which requires
one for reads as well. The token does not enable editing — the client rejects
write requests regardless. The app uses Capacitor's native HTTP bridge, so
plain-HTTP API requests do not require adding the app origin to
`allowed_origins`. See
[what `local_token` protects](../deployment-and-threat-model.md#exactly-which-requests-need-the-token)
for the full list of what the token gates.

## Read a shelf from pCloud

!!! warning "Experimental"
    pCloud support is newer than the rest of the Android client and has seen
    little use. Keep a shelf you care about reachable another way.

For a pCloud shelf the app talks to pCloud directly; no PlainShelf server on
another machine is involved. It reads the folder through the same shelf engine
the server uses, built into the app, so it reads the on-disk shelf layout the
server writes — see the [data model](../concepts/data-model.md) — and the folder
you point it at must be a shelf directory, the one containing `books/`. The
pCloud token stays in the Keystore: the app's native code hands it to the
engine, and it never passes through the web view. Which books count as adult
content follows the shelf's own marks, and whether they are shown follows the
device setting, not the setting a local shelf uses.

### 1. Create a pCloud application

PlainShelf registers no pCloud application and ships no app key, so you supply
your own. Create an application in pCloud's developer console and copy its **app
key**.

The key is not a secret. PlainShelf uses pCloud's `poll_token` authorization
flow, which needs only the app key: there is no app secret to protect, and no
redirect URL to register.

If it is not a secret, why not ship one? Because a shared key is a shared point
of failure. Everyone using a bundled key authorizes through the same pCloud
application, so a single rate limit, suspension, or policy change at pCloud's
end disconnects every user at once, and the fix is a new build rather than a
setting. rclone bundles a pCloud key and has had exactly that happen. The
`poll_token` flow also has no registered redirect URL tying a token back to the
application that asked for it, which makes a widely known key belonging to a
recognisable app more useful to somebody forging an approval link than a key
nobody has seen before. A key you registered yourself keeps the approval page,
the revocation, and the blast radius inside your own account.

### 2. Authorize

Add a pCloud shelf under **Settings → Shelves**, enter the app key, and tap
**Authorize with pCloud**. The approval page opens in the system browser; the
app waits for you to approve it, for up to five minutes.

Sign in as the account that holds the shelf. The system browser keeps its own
session, so it may already be signed in as somebody else — after authorizing,
the app shows which account it reached, and that is worth checking.

Which pCloud region serves the account is discovered during authorization, not
configured. Nothing needs to be chosen for a US or EU account.

### 3. Point it at the shelf

Tap **Browse pCloud…** and walk to the shelf folder. The picker lists one level
at a time — a tap opens a folder, the breadcrumb at the top jumps back to any
level above — and reads only the level on screen, so browsing costs one folder
listing per tap. **Use this folder** is offered on the folder that holds
`books/`, which is the shelf directory itself, not `books/` and not the folder
above it. Picking one fills the path in and runs the shelf check for you.

The path field stays editable if you would rather type or paste one, such as
`/PlainShelf/default-shelf`; tap **Check shelf** afterwards. Either way the
check reads the folder and reports how many books it found, skipping the
folders the shelf's `shelf.json` ignores, and it has to contain `books/` or it
is not a shelf. Save once the check passes. The picker and the check run in the
app's built-in shelf engine, so they work only in the Android app, not in the
browser preview.

### The book list is updated by hand

Reading the shelf means listing the whole folder tree and downloading every
book's `book.json` — one recursive listing plus a request per book. That is far
too expensive to repeat on a phone every time the app opens, so the app does not
repeat it.

The listing is scanned **once**, right after you save the connection, and then
kept on the device in full: every book's metadata and the file references needed
to open it. From then on the library opens from that copy with no network access
at all. Use **Update book list** on the library toolbar after adding, removing,
or renaming books on pCloud. The button shows when the list was last updated.

An update is cheaper than the first scan: a book whose `book.json` has not
changed size or modification time is not downloaded again, so a typical update
costs one folder listing plus only the books that actually changed.

### If a server or the desktop app also uses the shelf

The per-book downloads above are the expensive part, and they can be skipped
entirely. A PlainShelf server or desktop app that opens the same shelf writes a
copy of its book listing to `app/book-cache-{writer-id}.json`, and the Android
client reads that instead: one download for the whole shelf rather than two
requests per book. Nothing needs to be configured on the phone — the file is
used when it is there.

The first scan is where this matters most, but **Update book list** uses it too,
for the books the update actually has to read — a newly added book comes from
the file rather than from a download of its own. An update with nothing to read
does not fetch the file at all, so it stays as cheap as it was before.

Books changed since the file was written are still read individually, so a cache
that has fallen behind costs a few requests rather than showing stale metadata.

The file is refreshed automatically, but on the machine that owns the shelf, not
on the phone. If books were just added and the phone should see them now, use
**Settings → Shelves → Mobile book cache → Update now** there first. See
[Shelf cache and disk I/O](../concepts/shelf-cache-and-io.md#the-exported-book-cache).

A shelf kept on pCloud with no PlainShelf server anywhere has no such file, and
the app scans as described above.

Book *contents* are unaffected — opening a book always reads it from pCloud
unless the book has been downloaded for offline reading.

### What this mode does not have

- **No access token and no `protect_read`.** Those are server settings, and
  there is no server here. Access is controlled by the pCloud authorization
  alone.
- **No writing, ever.** Every mutating operation is refused by the client, on
  top of the read-only rules the Android client already applies.
- **Nothing to enumerate.** A server can offer several shelves; here the folder
  you named is the shelf. The device's own shelf list is unaffected — it can
  hold as many pCloud folders as you add.

Illustrations work here too: a source's `assets/` directory is part of the same
folder listing the shelf is read through, so a Markdown book shows its pictures
without an extra lookup.

Downloads, reading progress, read history, and reading time work exactly as for
a server shelf: they are stored on the device, kept separate per shelf entry,
and never written back.

## Read a shelf from another device

**Other device** reads a shelf over WebDAV through the embedded Go server, with
no server of your own: another PlainShelf's LAN share (the desktop app's, the
server's [`dav_export`](../reference/configuration.md#dav_export), or another
phone's), or a WebDAV server holding a shelf directory.

1. Pick **Another PlainShelf** or **WebDAV**, and enter the URL of the shelf
   folder, the one that holds `books/`. A LAN share shows it as
   `http://<address>:20001/dav/<shelf id>/`.
2. Enter the token (another PlainShelf) or, for WebDAV, the account and its
   password or token if the server asks for one.
3. **Check shelf** must pass before the entry can be saved. A refused token
   says so (`SOURCE_UNAUTHORIZED`); an address that cannot be reached says it
   cannot connect (`SOURCE_UNAVAILABLE`).

The secret is stored in Keystore-backed storage under
`plainshelf.mobile.secret.<entry id>`, the same key a pCloud token uses. Go reads
it through the `host:<entry id>` credential reference, so it never crosses the
JavaScript bridge after setup. The shelf opens read-only, and its books can be
downloaded for offline reading like a pCloud shelf's.

## Local shelves

A **local shelf** needs neither a server nor a network: it lives in the app's
private storage and is served by the PlainShelf server compiled into the app,
as the desktop client does. Add one under **Settings → Shelves → Add a shelf →
Local shelf**, give it a name, and it opens empty. Books are created, imported
from EPUB or text, edited and read as on the desktop, in airplane mode too.
Reading progress, read history, and reading time are kept on the device, as for
every other shelf.

A local shelf is deleted with the app, and removing it from the shelf list
deletes its books. **Export** on the shelf saves it as a zip through the system
file picker, and **Import from zip** makes a new local shelf from one; see
[Backup and Restore](../backup-and-restore.md#android-local-shelf).

### Uninstalling

Local shelves are stored in the app's private files directory (see
[How it is wired](#how-it-is-wired)), which Android deletes on uninstall. The
app guards against that in these ways, and syncs nothing automatically:

- The manifest sets `android:hasFragileUserData="true"`, so on Android 10 and
  later the uninstall dialog offers to keep the app's data. Kept, a reinstall
  finds every local shelf and its entry in the shelf list as they were.
- While the device has a local shelf, the shelf list says local shelves are
  deleted with the app and shows each one's last export time, or "Not backed up
  yet".
- With no shelf at all — a fresh install, or a reinstall without kept data —
  and no local shelf on disk, the shelf list first offers **Restore from a
  backup zip**, the same import as above, which then opens the restored shelf.
  **Skip** stores a flag in Capacitor Preferences and the offer does not come
  back. Preferences are excluded from Android backup, so the flag goes with an
  uninstall unless the data is kept.
- Google cloud backup leaves local shelves out (`local-shelves/` is excluded
  in `backup_rules.xml` and the `cloud-backup` section of
  `data_extraction_rules.xml`): PlainShelf does no cloud sync, and a shelf
  larger than Auto Backup's 25 MB quota would stop every later backup of the
  app. A restore from Google therefore brings no local shelf back, and the
  shelf list offers the zip restore above.
- Device-to-device transfer (Android 12 and later) keeps them: it copies
  directly between phones, with a 2 GB limit instead of 25 MB; past it the
  transfer is refused, so export first. Preferences are excluded from both, so
  a transferred app finds local shelves with no entry for them. The shelf list
  then adds an entry for each and opens the first, instead of offering a
  restore.

Neither is a backup PlainShelf guarantees, and on Android 11 and lower a
transfer follows the cloud rules and leaves local shelves out too. To back up,
export each local shelf before uninstalling; to restore, pick the zip from that
first-launch offer or from **Import from zip** later.

### How it is wired

- `mobile/` is the server as a library, bound with gomobile into
  `lab.voile.plainshelf.gobind.mobile`. The UI reaches it through no port;
  the only listener is the [LAN share](#share-local-shelves-on-the-local-network),
  open only while the user runs it. Shelves are created,
  renamed and removed through the package's own functions, reached only from
  the `PlainShelfLocalShelf` Capacitor plugin; the HTTP API has no route for
  any of that.
- `Handle` takes the request body as a file path and writes the response body
  to one, so a book never crosses JNI as a byte array.
- While a local shelf is active the API base is
  `https://localhost/__plainshelf_local`, the WebView's own origin.
  `LocalShelfWebViewClient` answers GET and HEAD under that prefix from
  `Handle`, streaming the response file — the API client's reads and every
  `<img src>` alike. The WebView gives `shouldInterceptRequest` no request
  body, so every other method goes through the plugin: the frontend
  (`providers/localShelf.ts`) stages the body into a cache file in 1 MiB chunks,
  then asks the plugin to send it.
- Export and import go through the Storage Access Framework
  (`ACTION_CREATE_DOCUMENT`, `ACTION_OPEN_DOCUMENT`). The plugin moves the zip
  between a cache file and the picked document; Go only sees the cache path, so
  the zip crosses neither the bridge nor JNI. Export writes the zip before the
  picker opens, so a busy shelf (`ErrBusy`) fails before any document exists,
  and records `last_export` with `MarkExported` only after the copy. The 512 MiB
  import cap (`internal/shelfzip`) is kept for the app: the zip is read from a
  file rather than memory here, but one limit for both callers keeps a backup's
  importability the same in the demo and the app.
- Shelves live under `<files dir>/local-shelves/shelves/<id>/` with
  `lock_mode: none`, their list in `<files dir>/local-shelves/shelves.json`.
  On `android` the settings store is the JSON file the wasm build uses rather
  than badger, and the frontend is not embedded in the library, since the APK
  already carries it.
- The embedded Go runtime sets `TMPDIR` to
  `<files dir>/local-shelves/tmp/` before opening shelves. Batch illustration
  archives are spooled there and removed after the response; Android's default
  `/data/local/tmp` is not writable by the app.
- `scripts/build-android-lib.sh <out-dir>` builds the arm64 AAR and reports
  each `.so`'s size and 16 KB page alignment. It needs the Android SDK, an NDK
  in `ANDROID_NDK_HOME` and a JDK, on Linux or macOS (where it reads the
  alignment with the NDK's `llvm-readelf`). The library is arm64 only, so a
  local shelf needs an arm64 device or emulator image.

## Share local shelves on the local network

**Settings → Shelves → Share on local network** lets another device on the same
Wi-Fi read the local shelves you tick, read-only, without exporting a zip first.
It is a temporary share, not a server:

- Off by default, and off means the app opens no network port. Nothing is
  ticked until you tick it, and nothing is remembered across a restart except
  the token.
- **Start sharing** binds the phone's Wi-Fi (or Ethernet) IPv4 address on port
  `20001` and shows the address (`http://<phone IP>:20001/dav/`, one shelf per
  `/dav/<shelf id>/`) and the token every request must carry. It is the same
  read-only WebDAV export as the server's
  [`dav_export`](../reference/configuration.md#dav_export) and the desktop's
  [share](../configuring-local-shelf.md#share-shelves-on-your-local-network-desktop-app):
  a request without the token is `401`, and any write is `405`.
- While it runs a notification shows the address and a **Stop** button, and the
  share is announced over mDNS as `_webdav._tcp` with `path=/dav/`.
- It stops by itself, with a notification saying why, after 30 minutes without
  a read the token admitted, or when the Wi-Fi network changes or drops. A
  request without the token does not keep it open.
- The token is `<files dir>/local-shelves/lan_share.token`, excluded from
  Android backup; it stays the same between shares, so a device that saved it
  keeps working.
- Off a Wi-Fi or Ethernet network, for example on mobile data or as a hotspot,
  it does not start.

The export is plain HTTP, so share only on a network you trust; see
[the threat model](../deployment-and-threat-model.md#the-read-only-webdav-export).

### How the share is wired

- Go binds and closes the listener (`StartLANShare`, `StopLANShare` in
  `mobile/lan_share.go`), so a busy port or a refused address fails the
  **Start sharing** call itself. It binds only an IPv4 unicast address, never
  `0.0.0.0` or loopback.
- `LanSharePlugin` starts the share, then `LanShareService`, a foreground
  service that owns the notification, the mDNS record, the idle check
  (`LANShareIdleMillis`, once a minute) and a default-network callback. Ending
  the service for any reason, including the system stopping it, ends the share,
  and it is `START_NOT_STICKY`, so a killed app comes back not sharing.
- Android 13+ asks for the notification permission on the first start. Refused,
  the share still runs; Android then shows it only in the task manager.

### Foreground service type

The service is `specialUse`. `dataSync` is the near miss, but from Android 15
the system stops a `dataSync` service after 6 hours in any 24, and serving books
to another device is not syncing with the cloud. `connectedDevice` is for
interacting with an external device such as a Bluetooth or USB accessory, which
a reader on another phone is not.
`specialUse` has no runtime prerequisite and no time limit; Play Console reviews
its stated use case, which the manifest carries as
`PROPERTY_SPECIAL_USE_FGS_SUBTYPE`. The text to submit there:

> PlainShelf lets the user temporarily share their own on-device book library,
> read-only, with their other devices on the same local Wi-Fi network. The user
> starts it from the app's settings; while it runs, an ongoing notification
> shows the address and a Stop button. It serves files only to a device that
> presents the token shown in the app, and stops itself after 30 minutes
> without a request or when the Wi-Fi network changes. The service must keep
> running while the app is in the background because the user reads on the
> other device, not on the phone; none of the other foreground service types
> covers serving files on the local network.

## App icons and splash screens

Source artwork lives in `frontend/assets/`. After changing it, regenerate the
Android assets from the `frontend` directory:

```bash
npx capacitor-assets generate --android
```
