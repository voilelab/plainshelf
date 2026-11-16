# Android app

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

**Without a shelf.** The app opens normally with an empty list, as the desktop
client does when it has none. Every page except Settings says no shelf is
selected and links to **Settings → Shelves**. When the entry in use lacks its
token or part of its settings, the notice links to that entry's editor instead.

**Switching shelves.** Exactly one entry is in use at a time. Pick another from
the shelf dropdown in the sidebar, or from the list itself. Switching restarts
the app, because what the app reads from is decided once at startup.

**Removing a shelf** deletes the books downloaded from it, freeing that space
on the device. Reading progress, read history, and reading time are kept —
they are the only records that cannot be rebuilt from the library, so adding
the same shelf back picks up where you left off. Removing a *local* shelf
deletes the shelf itself with every book in it, after a confirmation that says
so.

Server, pCloud and other-device entries are read-only. The sections below
cover each source type in turn.

## Connect to a PlainShelf server

The server must listen on a LAN-reachable address instead
of `127.0.0.1`. From the phone, verify that `http://<server-ip>:20000/health`
returns `1`, then add a shelf with that server URL under
**Settings → Shelves**.

Browsing, reading, and refreshing the book list require no access token. The
refresh walks the shelf and writes nothing, so the server treats it as a read. A
token is needed only when the server sets `protect_read: true`, which requires
one for reads as well. The token does not enable editing, because the client
rejects write requests regardless. The app uses Capacitor's native HTTP bridge,
so plain-HTTP API requests do not require adding the app origin to
`allowed_origins`. See
[what `local_token` protects](deployment-and-threat-model.md#exactly-which-requests-need-the-token)
for the full list of what the token gates.

## Read a shelf from pCloud

!!! warning "Experimental"
    pCloud support is newer than the rest of the Android client and has seen
    little use. Keep a shelf you care about reachable another way.

For a pCloud shelf the app talks to pCloud directly; no PlainShelf server on
another machine is involved. It reads the folder through the same shelf engine
the server uses, built into the app, so it reads the on-disk shelf layout the
server writes (see the [data model](concepts/data-model.md)), and the folder
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
at a time, where a tap opens a folder and the breadcrumb at the top jumps back
to any level above. It reads only the level on screen, so browsing costs one
folder listing per tap. **Use this folder** is offered on the folder that holds
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
[Shelf cache and disk I/O](concepts/exported-book-cache.md).

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

**Another device** reads a shelf over WebDAV through the embedded Go server, with
no server of your own: another PlainShelf's LAN share (the desktop app's, the
server's [`dav_export`](reference/configuration.md#dav_export), or another
phone's), or a WebDAV server holding a shelf directory.

1. Pick **Another PlainShelf** or **WebDAV**, and enter the URL of the shelf
   folder, the one that holds `books/`. A LAN share shows it as
   `http://<address>:20001/dav/<shelf id>/`.
2. Enter the token (another PlainShelf) or, for WebDAV, the account and its
   password or token if the server asks for one.
3. The form checks the shelf once the address and token are filled in and
   shows how many books it found; **Add shelf** checks again if needed and saves
   nothing if the check fails. A refused token says so (`SOURCE_UNAUTHORIZED`);
   an address that cannot be reached says it cannot connect
   (`SOURCE_UNAVAILABLE`).

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
[Backup and restore](backup-and-restore.md#android-local-shelf).

### Uninstalling

Local shelves are stored in the app's private files directory (see
[How local shelves are wired](development/android.md#how-local-shelves-are-wired)),
which Android deletes on uninstall. The app guards against that in these ways, and syncs nothing automatically:

- The manifest sets `android:hasFragileUserData="true"`, so on Android 10 and
  later the uninstall dialog offers to keep the app's data. Kept, a reinstall
  finds every local shelf and its entry in the shelf list as they were.
- While the device has a local shelf, the shelf list says local shelves are
  deleted with the app and shows each one's last export time, or "Not backed up
  yet".
- With no shelf at all (a fresh install, or a reinstall without kept data) and
  no local shelf on disk, **Settings → Shelves** offers **Restore from a backup
  zip**, the same import as above, which then opens the restored shelf. **Skip**
  stores a flag in Capacitor Preferences and the offer does not come back.
  Preferences are excluded from Android backup, so the flag goes with an
  uninstall unless the data is kept.
- Google cloud backup leaves local shelves out (`local-shelves/` is excluded
  in `backup_rules.xml` and the `cloud-backup` section of
  `data_extraction_rules.xml`): PlainShelf does no cloud sync, and a shelf
  larger than Auto Backup's 25 MB quota would stop every later backup of the
  app. A restore from Google therefore brings no local shelf back, and
  **Settings → Shelves** offers the zip restore above.
- Device-to-device transfer (Android 12 and later) keeps them: it copies
  directly between phones, with a 2 GB limit instead of 25 MB; past it the
  transfer is refused, so export first. Preferences are excluded from both, so
  a transferred app finds local shelves with no entry for them. At startup it
  adds an entry for each and opens the first, instead of offering a restore.

Neither is a backup PlainShelf guarantees, and on Android 11 and lower a
transfer follows the cloud rules and leaves local shelves out too. To back up,
export each local shelf before uninstalling; to restore, pick the zip from that
first-launch offer or from **Import from zip** later.

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
  [`dav_export`](reference/configuration.md#dav_export) and the desktop's
  [share](configuring-local-shelf.md#share-shelves-on-your-local-network-desktop-app):
  a request without the token is `401`, and any write is `405`.
- The panel shows the same [v1 QR payload](configuring-local-shelf.md#the-qr-code)
  as the desktop: the root URL, token, and each selected shelf's ID, name and
  URL. Selection, address and token changes refresh the QR. Stopping sharing
  removes the address, token and QR from the panel. If the payload exceeds QR
  capacity, the panel explains why and keeps the address and token available;
  share fewer shelves or enter their connection details manually.
- While it runs a notification shows the address and a **Stop** button, and the
  share is announced over mDNS as `_webdav._tcp` with `path=/dav/`.
- It stops by itself, with a notification saying why, after 30 minutes without
  a read the token admitted, or when the Wi-Fi network changes or drops. A
  request without the token does not keep it open.
- The token is `<files dir>/local-shelves/lan_share.token`, excluded from
  Android backup; it stays the same between shares, so a device that saved it
  keeps working. **Replace token** saves a new token and immediately updates the
  panel and QR. The old token's next request returns `401`; devices must obtain
  the new token. Stopping and restarting sharing keeps the replacement token.
- Off a Wi-Fi or Ethernet network, for example on mobile data or as a hotspot,
  it does not start.

The export is plain HTTP, so share only on a network you trust; see
[the threat model](deployment-and-threat-model.md#the-read-only-webdav-export).

## Scan a LAN share QR code

On **Add a shelf**, choose **Scan a QR code to add them all** below the source
list, tap **Scan a LAN share QR code**, and allow camera access.
Scan the desktop publisher's QR code, check the listed names and URLs, then
confirm **Import shelves**. Previewing makes no network request and saves no
credentials. Each confirmed shelf is checked through `LocalShelf.checkSource`
before becoming a read-only `plainshelf` entry. Results stay visible until you
choose **Open imported shelf** or scan again. Failed sources are not saved;
scan and confirm again to retry them. Existing PlainShelf URLs (ignoring trailing
slashes) are skipped without changing their names or tokens. The last successful
import becomes active, as with manual setup. Names can repeat; device-local IDs
are freshly generated.

Cancelling scanning or the preview stores nothing. An unavailable scanner,
missing camera permission, or scan failure leaves manual URL/token setup
available. Invalid JSON, unsupported versions, missing fields and unsafe URLs
are rejected as a whole by the same `internal/lanshare` parser used by desktop.
The transient QR payload is discarded on cancel, import or leaving the form.
Imported tokens go only through the existing SecureStorage/Android Keystore
path, separately from the entry metadata in Preferences; they are not put into
URLs, logs or `shelves.json`.
