# Android Development

The Android client is experimental. It wraps the Vue frontend with Capacitor and
reads a library from one of two places: a separately running PlainShelf server,
or a shelf folder held on pCloud. Downloaded books, covers, reading progress,
read history, and reading time are stored locally in the app's private storage
(`Directory.Data`, no runtime permission required) and never leave the device.

The app is **read-only**: it browses, reads, and downloads books for offline
use, but it never modifies the library and issues no write requests at all.
Editing pages (metadata, sources, trash, and the maintenance views) are not
reachable on Android, and write requests are rejected on the device before they
are sent.

## Connection modes

Pick one under **Settings → Connection**. The choice is stored on the device and
changing it restarts the app.

| Mode | Where the library lives | What you need |
|---|---|---|
| PlainShelf server | A server you run, reachable over the network | Its URL, and an access token only when the server sets `protect_read` |
| pCloud | A shelf folder in your pCloud account | Your own pCloud app key, and the folder path |

The pCloud mode needs no server at all, which is the point of it: a phone cannot
mount cloud storage the way a host can, so a server keeping its shelf on pCloud
through an rclone mount is no help to the phone.

Both modes are read-only. The rest of this page covers the server mode first,
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

Server mode.

```bash
just run-android-app
```

This starts the local server, waits for it to become healthy, boots an available
emulator when needed, builds and launches the app, and prints the ephemeral
access token. In the emulator, connect to `http://10.0.2.2:20000`; Android maps
`10.0.2.2` to the host loopback address. The printed token only needs to be
entered under **Settings → Connection** when the server sets `protect_read:
true`.

## Connect a physical device

Server mode. The server must listen on a LAN-reachable address instead of
`127.0.0.1`. From the phone, verify that `http://<server-ip>:20000/health`
returns `1`, then enter the same server URL in **Settings → Connection**.

Browsing and reading require no access token. One is needed only when the server
sets `protect_read: true`, which requires a token for reads as well. The token
does not enable editing — the client rejects write requests regardless. The app
uses Capacitor's native HTTP bridge, so plain-HTTP API requests do not require
adding the app origin to `allowed_origins`.

## Read a shelf from pCloud

!!! warning "Experimental"
    pCloud support is newer than the rest of the Android client and has seen
    little use. Keep a shelf you care about reachable another way.

In this mode the app talks to pCloud directly and no PlainShelf server is
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

### 2. Authorize

Enter the app key under **Settings → Connection** and tap **Authorize with
pCloud**. The approval page opens in the system browser; the app waits for you
to approve it, for up to five minutes.

Sign in as the account that holds the shelf. The system browser keeps its own
session, so it may already be signed in as somebody else — after authorizing,
the app shows which account it reached, and that is worth checking.

Which pCloud region serves the account is discovered during authorization, not
configured. Nothing needs to be chosen for a US or EU account.

### 3. Point it at the shelf

Give the folder path of the shelf inside your pCloud account, such as
`/PlainShelf/default-shelf`, then tap **Check shelf**. That reads the folder and
reports how many books it found; it has to contain `books/` or it is not a
shelf. Save once the check passes.

### What this mode does not have

- **No access token and no `protect_read`.** Those are server settings, and
  there is no server here. Access is controlled by the pCloud authorization
  alone.
- **No writing, ever.** Every mutating operation is refused by the client, on
  top of the read-only rules the Android client already applies.
- **No shelf list.** A server can offer several shelves; here the folder you
  named is the shelf.

Downloads, reading progress, read history, and reading time work exactly as in
server mode: they are stored on the device, kept separate per connection, and
never written back.

## App icons and splash screens

Source artwork lives in `frontend/assets/`. After changing it, regenerate the
Android assets from the `frontend` directory:

```bash
npx capacitor-assets generate --android
```
