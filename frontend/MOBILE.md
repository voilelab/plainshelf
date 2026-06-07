# Plainshelf Android shell

This frontend includes a minimal Capacitor Android shell for running the existing Vite/Vue app inside an Android WebView.

## Commands

The frontend uses npm with `package-lock.json`.

- Build web assets: `npm run build`
- Sync web assets into Android: `npx @capacitor/cli sync android`
- Build a debug Android APK: `cd android && gradle assembleDebug`

The Capacitor app uses `dist` as its `webDir`, so `npm run build` must produce `dist/index.html` before syncing Android assets. The generated Android copy at `android/app/src/main/assets/public/` is intentionally gitignored to avoid committing built web assets or binary icon files.

## Runtime behavior

Capacitor Android is detected with the native `window.Capacitor` runtime signal. Browser/server mode and Wails desktop mode continue to use their existing providers and HTML5 history router. Android uses hash history to keep deep links inside the bundled WebView assets.

The Android shell selects the existing `MobileBookshelfProvider`, which still delegates online API calls to `ServerBookshelfProvider` and only has the existing in-memory cache. Persistent offline downloads, SQLite/filesystem storage, Android file pickers, and a server settings screen are intentionally not implemented yet.

For remote API testing in Android, build the frontend with the existing `VITE_API_BASE` environment variable pointing at the reachable Plainshelf server. Token handling remains the existing `VITE_PLAINSHELF_TOKEN`/runtime token path.
