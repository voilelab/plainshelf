# Android Development

The Android client is experimental. It wraps the Vue frontend with Capacitor
and connects to a separately running PlainShelf server. Downloaded books,
covers, and reading progress are cached locally for offline reading; reading
progress does not currently sync back to the server.

The app is **read-only**: it browses, reads, and downloads books for offline
use, and it records read history and reading activity on the server, but it
never modifies the library. Editing pages (metadata, sources, trash, and the
maintenance views) are not reachable on Android, and write requests are
rejected on the device before they are sent.

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

```bash
just run-android-app
```

This starts the local server, waits for it to become healthy, boots an available
emulator when needed, and builds and launches the app. In the emulator, connect
to `http://10.0.2.2:20000`; Android maps `10.0.2.2` to the host loopback
address. The access token the recipe prints is not needed by the app, which is
read-only.

## Connect a physical device

The server must listen on a LAN-reachable address instead of `127.0.0.1`. From
the phone, verify that `http://<server-ip>:20000/health` returns `1`, then enter
the same server URL in **Settings → Connection**.

No access token is required: the app only reads. The app uses Capacitor's native
HTTP bridge, so plain-HTTP API requests do not require adding the app origin to
`allowed_origins`.

## App icons and splash screens

Source artwork lives in `frontend/assets/`. After changing it, regenerate the
Android assets from the `frontend` directory:

```bash
npx capacitor-assets generate --android
```
