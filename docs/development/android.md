# Android Development

The Android client is experimental. It wraps the Vue frontend with Capacitor and
reads a library from one of two places: a separately running PlainShelf server,
or a shelf folder held on pCloud. Downloaded books, covers, illustrations,
reading progress, read history, and reading time are stored locally in the app's
private storage
(`Directory.Data`, no runtime permission required) and never leave the device.
The one exception is **Export file** on a book's page: it writes a copy of the
book's text into the shared `Documents/PlainShelf/` folder so it can be opened
from the Files app or handed to another app. On Android 10+ the Filesystem
plugin routes that through scoped storage with no permission; on Android 9 and
below it requests `WRITE_EXTERNAL_STORAGE` (declared with `maxSdkVersion="29"`).

The app is **read-only**: it browses, reads, and downloads books for offline
use, but it never modifies the library and issues no write requests at all.
Editing pages (metadata, sources, trash, and the maintenance views) are not
reachable on Android, and write requests are rejected on the device before they
are sent.

## The shelf list

The device keeps a list of shelves under **Settings → Shelves**, and each
entry carries its own source type. One list can hold a couple of PlainShelf
servers and a couple of pCloud folders at once:

| Source type | Where the library lives | What the entry stores |
|---|---|---|
| PlainShelf server | A server you run, reachable over the network | Its URL, which shelf on it, and an access token only when the server sets `protect_read` |
| pCloud | A shelf folder in your pCloud account | Your own pCloud app key, the region learned during authorization, and the folder path |

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
the same shelf back picks up where you left off.

Every entry is read-only. The rest of this page covers the server source first,
then [Read a shelf from pCloud](#read-a-shelf-from-pcloud).

## Prerequisites

- The base [local development](setup.md) toolchain
- Android SDK
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

Browsing and reading require no access token. One is needed only when the server
sets `protect_read: true`, which requires a token for reads as well. The token
does not enable editing — the client rejects write requests regardless. The app
uses Capacitor's native HTTP bridge, so plain-HTTP API requests do not require
adding the app origin to `allowed_origins`.

## Read a shelf from pCloud

!!! warning "Experimental"
    pCloud support is newer than the rest of the Android client and has seen
    little use. Keep a shelf you care about reachable another way.

For a pCloud shelf the app talks to pCloud directly and no PlainShelf server is
involved. It reads the same on-disk shelf layout the server writes — see the
[data model](../concepts/data-model.md) — so the folder you point it at must be
a shelf directory, the one containing `books/`.

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
check reads the folder and reports how many books it found, and it has to
contain `books/` or it is not a shelf. Save once the check passes.

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

## App icons and splash screens

Source artwork lives in `frontend/assets/`. After changing it, regenerate the
Android assets from the `frontend` directory:

```bash
npx capacitor-assets generate --android
```
