# Android Development

This page covers building and running the experimental Android client from
source, and how its native parts are wired. What the app does and how to set
up each kind of shelf is in [Android app](../android-app.md).

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

`build-mobile-android` first runs `just mobile-sync`, which rebuilds the
frontend and copies it and the native plugins into the Android project, and
`just android-lib`, which binds the embedded
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

Without overrides, Gradle uses `0.0.0-dev` and version code `1`.

Every build packages only `arm64-v8a` native code (`abiFilters` in
`app/build.gradle`), because the embedded server is bound for `android/arm64`
alone. x86_64 emulators, most Chromebooks and 32-bit phones cannot install it.

## Release builds

The release workflow builds `assembleRelease` and attaches
`plainshelf-android_<tag>_arm64.apk` to the GitHub Release
([Releasing](releasing.md)). It differs from a local debug build in three ways.

**Version code.** `scripts/resolve-version.sh android-code <tag>` derives it
from the tag, so it rises with every release whichever branch the release is
built on:

```text
MAJOR × 1,000,000 + MINOR × 10,000 + PATCH × 100 + slot
```

The slot is `99` for a stable tag and `N` for an `rcN` or `rc.N` prerelease
(N from 1 to 98), so `v1.0.0-rc1` is `1000001` and `v1.0.0` is `1000099`.
MINOR and PATCH must stay below 100. Any other prerelease, such as `beta.1`,
has no code. Prepare release refuses such a tag, and a tag whose code is not
above the latest release's (`v1.0.0-rc1` after `v1.0.0-rc.2`, or `rc2` after
`rc10`, both newer by SemVer), instead of opening a release PR
(`resolve-version.sh validate-next-android`).
The commit count a local debug build uses is far below every release code, so
a release always installs over one, once the signature matches.

**Signing.** Android installs nothing unsigned and refuses an update signed by
a different key. A debug build is signed with the build machine's own
`~/.android/debug.keystore`, which differs from machine to machine and from
one CI run to the next, so a debug APK cannot be upgraded to a release one: the
user uninstalls first, which deletes local shelves unless the uninstall dialog
keeps the app's data ([Uninstalling](#uninstalling)).

A release APK is signed with one long-lived key. Gradle signs `release` builds
when these variables are set, and leaves the APK unsigned otherwise:

| Variable | Value |
|---|---|
| `PLAINSHELF_KEYSTORE_FILE` | path to the `.jks` keystore |
| `PLAINSHELF_KEYSTORE_PASSWORD` | keystore password |
| `PLAINSHELF_KEY_ALIAS` | key alias |
| `PLAINSHELF_KEY_PASSWORD` | key password |

The release workflow fills them from repository secrets; see
[One-time setup](releasing.md#one-time-setup). The key cannot be replaced:
losing it means no installed copy can ever be updated again, so keep the
keystore and its passwords in at least two places outside the repository.
`*.jks` and `*.keystore` are ignored under `frontend/android`.

**Debuggable.** Debug builds are debuggable, which lets `adb shell run-as` read
the app's private storage, local shelves included. Release builds are not.

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

The server must listen on a LAN-reachable address instead of `127.0.0.1`; see
[Connect to a PlainShelf server](../android-app.md#connect-to-a-plainshelf-server)
for the steps on the phone.

## How local shelves are wired

- `mobile/` is the server as a library, bound with gomobile into
  `lab.voile.plainshelf.gobind.mobile`. The UI reaches it through no port;
  the only listener is the [LAN share](../android-app.md#share-local-shelves-on-the-local-network),
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
  The settings store uses `settings.json` on every platform. The frontend is
  not embedded in the Android library, since the APK already carries it.
- The embedded Go runtime sets `TMPDIR` to
  `<files dir>/local-shelves/tmp/` before opening shelves. Batch illustration
  archives are spooled there and removed after the response; Android's default
  `/data/local/tmp` is not writable by the app.
- `scripts/build-android-lib.sh <out-dir>` builds the arm64 AAR and reports
  each `.so`'s size and 16 KB page alignment. It needs the Android SDK, an NDK
  in `ANDROID_NDK_HOME` and a JDK, on Linux or macOS (where it reads the
  alignment with the NDK's `llvm-readelf`). The library is arm64 only, so a
  local shelf needs an arm64 device or emulator image.

## How the LAN share is wired

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

## QR scanner dependency and permissions

The native `PlainShelfQRScanner` plugin uses
[`zxing-android-embedded` 4.3.0](https://github.com/journeyapps/zxing-android-embedded/tree/v4.3.0)
(Apache-2.0), including ZXing core (Apache-2.0). It bundles the decoder rather
than depending on Google Play services, Google Code Scanner or a downloaded
ML model. Its default ZXing core 3.4.1 requires API 24, matching this app's
unchanged minimum; its Java 8 requirement is covered by the existing JDK 21
Android build.
Their license text and attribution ship in
`frontend/public/licenses/android-qr-Apache-2.0.txt`.
It does not add an npm dependency. Barcode image saving and the completion
beep are disabled.

The scanner activity requests `CAMERA` at use time. Camera and autofocus
features are explicitly optional in the manifest, so installing or using the
app on a camera-less device remains possible. No media/storage permission is
needed for scanning. Inspect the merged manifest when building the APK to
confirm these optional feature declarations survive dependency merging.

LAN source reads use the embedded Go HTTP client and existing `INTERNET`
permission, not browser camera or WebView fetch access. The app targets API 36.
[Android 16 local network protection](https://developer.android.com/privacy-and-security/local-network-permission)
is an opt-in compatibility test at that target; this app does not enable that
opt-in or require `NEARBY_WIFI_DEVICES` just to import a QR code. Devices running
that compatibility experiment must grant the experiment's nearby-device
permission for LAN reads; camera permission alone does not grant network access.
Recheck the local-network runtime permission requirements before raising the
target SDK to 37. QR import does not change cleartext-network configuration;
validate the merged manifest and actual HTTP LAN reachability on a device.

### L5 verification record (required before claiming device validation)

Status for this change: **not run**. The development environment has no Android
SDK, emulator or attached physical device. No camera, permission-dialog or LAN
connection result, or device screenshot, has been collected.

Record device/model, Android API, WebView version, build commit, Play services
presence, LAN topology and publisher version in the PR. Run these steps and
attach screenshots of the preview/results and native scan/permission screens
with tokens and QR contents redacted:

1. On the same Wi-Fi as a desktop publisher, scan a v:1 QR containing multiple
   shelves. Check the preview, confirm, open the imported shelf and read a book.
2. Cancel with Back during scanning, then cancel the preview. Restart the app
   and confirm neither created entries nor credentials.
3. Deny camera permission (including permanent denial), and test unavailable
   camera hardware where possible. Confirm manual setup still works.
4. Scan unknown-version, missing-field and non-HTTP(S) QR payloads. Verify the
   reason is visible and no partial entry or secret is stored.
5. Stop one source or use a wrong token; confirm successful sources are saved
   and failed sources are absent. Repeat import and verify existing names and
   credentials remain unchanged.
6. Inspect Preferences and exported shelf metadata for absence of the token;
   restart and verify the imported credentials still resolve from Keystore.
7. Include an API 24 device and a device without Play services, plus API 36
   with normal LAN access. If testing local-network protection's compatibility
   experiment, record its settings and denied/granted behavior separately.
