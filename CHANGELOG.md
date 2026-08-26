# Changelog

All notable changes to PlainShelf are documented in this file.

This project is currently in pre-alpha / early development. APIs, data layout,
and UI behavior may still change between releases.

## [Unreleased]

### Removed

- Removed the one-off `cmd/migrate-legacy-sources` tool and its `internal/legacyupgrade` support code, along with the now-dead `SplitConfig`/`SplitType` types and the `split_config` field on source metadata, so `split_config` exists only as an ignored unknown key in old `meta.json` files. A legacy source — one whose `meta.json` predates source-level format versioning — still lists and reads exactly as before, rendering as a single plain-text section; nothing about how it is read has changed. To give such a source chapters again, use the source editor's TXT → Markdown conversion (by regex or fixed line count), which writes a new schema v1 source and leaves the legacy original intact.

## [v0.10.0] - 2026-08-26

### Added

- Added a minimum/maximum character-count range filter to the library toolbar, held in the URL as `minChars` and `maxChars` so a filtered view can be bookmarked and combined with the folder, search, and sort controls. It is not offered on the Android client, whose pCloud mode would have to read every book over the network.
- Added case-sensitive, whole-word, and regular-expression matching to the source editor's find and replace, with a highlight over every match, a "match x of N" count, and `$1`–`$9`/`$&` capture references in replacements. The "current chapter" scope now filters matches so find, replace, and highlight all agree on the same range.
- Added a working **Export file** action to the Android book detail page, which now saves the book's text into a shared `Documents/PlainShelf/` folder via the Filesystem plugin (Android 11+ needs no permission; Android 10 and below need both external-storage permissions, declared capped at API 29). It previously did nothing there, because the web build's blob download is ignored by the Android WebView.
- Added a device-local reader launch preference in a new **Settings → Reading** tab, choosing whether **Read** opens a new reader — a new browser tab on web, the standalone reader app on desktop — or navigates the current window in place. The default stays opening a new reader, and the mobile and standalone shells always read in place.
- Added aggregate progress and an abort control to multi-file imports: the import dialog shows an "N / M" count with the current filename and an **Abort** button that stops after the in-flight file and marks the rest cancelled. Desktop imports now run one file per call through the same dialog, which opens and auto-starts instead of freezing until the whole batch finished, and a failed or cancelled batch keeps its per-file results and error banner on screen instead of blanking.

### Changed

- **Breaking (pre-1.0):** renamed the "layer" concept to "folder" across the API, URL, and UI. The routes are now `/folders`, `/folder-moves`, `/folder-transfers`, and `/folders/{path}` (the old `/layers` paths now `404`), the book JSON key is `folder`, and the library filters on `?folders=`. Two on-disk caches (`trash.json`, `app/book-cache-*.json`) bump to schema v2 with renamed fields; the `books/` layout and `book.json` are unchanged, so an existing shelf opens with no migration.
- **Breaking (pre-1.0):** renamed the trash directory from the hidden `.trash/` to a visible `trash/`. The rename runs automatically and destroys nothing the first time a newer build opens an older shelf — it is detected by the presence of `.trash/`, with no on-disk version marker — but it does not reverse: reopen the shelf with a build old enough to still expect `.trash/` and the trashed books, now under `trash/`, drop out of that build's trash view until they are moved back by hand. See [Data Format Versioning](docs/concepts/data-format-versioning.md#shelf-layout-changes-are-not-versioned).
- Changed a full shelf scan to skip re-listing folders whose modification time (and their parent's) is unchanged, replacing the listing with a single stat; on a 3061-folder fixture the second scan issues no listings and takes about a third of the time, with a larger saving on network mounts. The record is kept in `app/scan-cache.json`, and the new `scan_cache: off` setting disables it for mounts whose directory times cannot be trusted.
- Changed `GET /books?include=char_count` to answer from the in-memory book cache instead of opening every book's source per request, so asking for counts adds no disk operation to a listing; every route that rewrites content or moves the current-source pointer refreshes the cached count. In exchange, a full scan now reads each book's current-source `meta.json`. The response shape is unchanged.
- Changed each book folder's hint file to `CURRENT_SOURCE.txt` with English content (was `CURRENT_VERSION_LOCATION.txt` with hardcoded Traditional Chinese). Nothing reads it back — `current_source` in `book.json` remains authoritative — and the new file replaces the old one the next time a book's current source changes, so existing shelves need no migration.
- Changed the source editor to CodeMirror 6, which holds the whole source at once so find results, chapter jumps, and the caret land where asked; focusing a chapter now only dims the rest rather than removing it, preserving undo history. Markdown sources are syntax-highlighted and lists/quotes continue on Enter, and CRLF endings are preserved. The editor page's JavaScript grows to 184 kB compressed but loads only when opened.
- Changed new book IDs from an 8-hex-character hash of the layer path and title — narrow enough to collide across machines sharing a shelf — to a random v4 UUID. Existing books keep their IDs with no migration, and the older form works alongside UUIDs in the same shelf.
- Changed the Android client to require a book to be downloaded before reading: the reader route is refused for a not-yet-downloaded book and redirects to its detail page to prompt a download, while already-downloaded books (including ones with a pending update) open as before. The download now writes `manifest.json` last, so a book counts as downloaded only once its content is on disk.
- Changed source deletion so a book always keeps a usable current source: deleting the current source hands `current_source` to the newest remaining source, and deleting a book's only source leaves an empty plain-text source behind. Because this writes `book.json`, deleting the current source of a book written by a newer PlainShelf is refused with `409 Conflict`.
- Reworked the dashboard into a **Home** page that routes into the library rather than only summarizing it. It now lives at `/home` (with `/` and `/dashboard` redirecting and preserving the query), leads with a "recently reading" list and a "recently added" row, turns the headline counts and tag-cloud chips into links into the matching book lists, and replaces "added this month" with an "in progress" count read from device-local reading progress. An empty shelf shows getting-started guidance instead of a wall of zeroes, loading renders a skeleton so the layout does not shift, and the reading heatmap now fills the row width.
- Changed the similar-books duplicate-detection endpoint to gate on a work budget — the total sketch length actually walked — instead of a raw book-count limit, so a shelf of short works is measured by its real comparison cost rather than being blocked or waved through by count alone. An over-budget shelf now returns `200` with an explicit rejection body instead of a `202` that promised a result never produced, and the frontend message explains the budget rather than suggesting a "narrow the shelf" step that does not exist.

### Fixed

- Fixed a book becoming unreadable after its current source was deleted, which blanked the detail page and reader. Reads now fall back to the newest remaining source whenever `current_source` cannot be resolved (repairing books already in this state, without rewriting `book.json`), and a book with no source at all now returns `404` instead of `500`.
- Fixed the Android client's pCloud mode listing helper directories a NAS or sync client leaves in a shelf (`@eaDir`, `#recycle`, `$RECYCLE.BIN`, `lost+found`, dot-prefixed dirs) as folders, and books inside them as ordinary books. The server had always skipped these; the pCloud walker now applies the same rule.
- Fixed the reader's arrow-key chapter navigation going dead after clicking any button — the side actions, the desktop chapter controls, or a button left focused when a chapter or font modal closed. The keys now move a caret only while a text field is focused and otherwise always turn the page.

### Removed

- Removed the **Maintenance → Low Character Count** page (`/books/maintenance/low-char-count`), replaced by the character-count range filter, which can also combine with the folder, search, and sort controls. The underlying API is unchanged.

## [v0.9.0] - 2026-08-15

### Added

- Added EPUB import: an `.epub` selected in the import dialog, dropped on the library, or picked from the desktop file dialog is converted to text and stored as an ordinary book, carrying over reading-order text, chapter names, cover, title, authors, language, description, identifiers, and date. The original archive is not kept and no new third-party dependency is required.
- Added a choice of EPUB output layout — Markdown with `##` chapter headings, or plain text with no chapter structure — offered in the import dialog and defaulted from a new **Settings → Import** tab, plus an option to also write the book description into the text.
- Added the `epub_import_strategy` server setting (`GET`/`POST`/`DELETE /api/setting/epub_import_strategy`) and config key holding the default EPUB layout, applied server-side so it also governs the desktop file picker.
- Added a `strategy` field to the book import endpoint, letting one request override the configured default.
- Added [EPUB Import](docs/epub-import.md) documenting what is kept, what is dropped, how the two layouts differ, and the size limits.
- Added a pCloud connection mode to the Android client, reading a shelf directly from a pCloud folder with no PlainShelf server involved. You supply your own pCloud application (PlainShelf ships no app key), authorization runs in the system browser through pCloud's `poll_token` flow, and you name the shelf folder containing `books/`. The mode is read-only and experimental, with downloads, progress, and history kept per shelf on the device.
- Added a `schema_version` field to `book.json`, establishing schema v1 as the first versioned on-disk book format. Libraries created before this have no marker, are read as v1, and are upgraded lazily — the version is written only the next time a book is modified.
- Added a compatibility policy and upgrade documentation covering the on-disk schema version, backing up and restoring a shelf, and what to do when PlainShelf refuses to write a book (`docs/concepts/data-format-versioning.md`).
- Added a "Low Character Count" maintenance page listing books whose character count is at or below a threshold, reusing the shared maintenance book list; books with an unknown count are listed and reported separately.
- Added source illustrations: a source can hold images in an `assets/` directory beside `source.txt`, reachable through `GET`/`PUT`/`DELETE /api/shelves/:id/books/:book_id/sources/:source_id/assets/:asset_name`, and a Markdown book renders a line that is only `![alt](assets/…)` as a figure. Nothing indexes the directory, so `book.json` is unchanged; uploads are capped at 20 MB and reads are streamed, and there is no UI for adding images yet.
- Added EPUB illustration import: an imported EPUB stores the images its chapters referenced and links each where it appeared, for the Markdown layout (plain text drops them). Whatever cannot be kept — unservable formats such as SVG, images inside an `<svg>`, anything over 8 MB alone or 64 MB total — is counted and shown as "Import note" on the book detail page. A `keep_images` field turns it off.
- Added illustrations to the Android client: a downloaded book stores the images its text renders, and a pCloud shelf resolves them from its folder listing. A book downloaded before this shows alt text until downloaded again.
- Added reader font selection, with two bundled licensed fonts alongside the system default, chosen from the reader's side actions and stored per device; their licenses are viewable in-app and verified by a build-time check.
- Added rendering of a sanitized subset of HTML in the Markdown reader, so inline and block HTML appears as formatting instead of literal text; scripts, event handlers, and external resource references are stripped.
- Added a chapter outline to the source editor, with focused editing of one Markdown chapter at a time and format-conversion actions (TXT to Markdown by regex or fixed line count, Markdown to plain text, and upgrading a legacy split-configured source). Every conversion writes a new source and leaves the original intact.
- Added multi-select to the library, with batch move and batch delete run as a background task chain behind a progress modal on both wide and narrow viewports. `POST /api/shelves/:id/book-batches` is the backing endpoint.
- Added Empty trash as a background task with a progress modal: `POST /api/shelves/:id/trash/empty` returns 202 with a task chain ID and deletes books one at a time, and a sweep already in flight answers 409 with the running chain's ID. Progress is read through `GET /api/taskchains/:id`.
- Added a shelf-wide content statistics refresh to the Low Character Count page, recomputing line and character counts of every book whose current source reports none. `POST /api/shelves/:id/content-stat-refreshes` runs it as a task chain; the button is hidden on read-only runtimes.
- Added an "Update content stats" action to the book detail page (`POST /api/shelves/:id/books/:book_id/sources/:source_id/refresh`), which re-reads the source and recomputes its MD5, line count, and character count — correcting the counts after `source.txt` is edited outside PlainShelf.
- Added a global default section split rule, set from a new **Settings → Reader** tab and stored through `GET`/`POST`/`DELETE /api/setting/default_split_config`, which a legacy source with no split configuration falls back to. Regex and line-count values are validated on write.
- Added `format` to the fields accepted by the book PATCH endpoint, so an import's guess can be corrected — most often a `.txt` that holds Markdown; the switch rewrites only metadata and is reversible.
- Added an immersive mobile reader: the chrome hides while reading, a middle tap brings it back, and the edges page forward and back. Font size, line height, and the reading font are set from a mobile reader settings sheet.
- Added multiple shelves to the Android client: the device keeps a list of shelf entries, each with its own source type (PlainShelf server or pCloud folder) and credentials, switched from the sidebar. Per-entry tokens moved into Keystore-backed storage, and existing offline downloads are kept.
- Added a persisted pCloud book listing and an "Update book list" button on the library toolbar. A launch rebuilds the library from the stored copy with no request, and an update re-downloads only the `book.json` files whose size or modification time changed.
- Added an exported book cache, `app/book-cache-{writer-id}.json`, written by the server and desktop app. It holds every book's `book.json`, package path, and the shelf's layers, so a client reading the shelf directly (Android on pCloud) downloads one file instead of two requests per book; an unchanged export does not touch disk.
- Added split-configuration caching to the Android offline download, so a downloaded book keeps its chapter list without reaching the server.

### Changed

- Changed the project scope to include EPUB as an import format; it remains excluded as a storage and rendering format, and PDF, comic archives, DRM, and OCR are still out of scope.
- Books whose `book.json` was written by a newer PlainShelf build are now read-only rather than silently rewritten: they stay visible and readable, `schema_version` is reported in book API responses, and any modification fails with `409 Conflict` before any filesystem change.
- Changed the sidebar's collapse toggle into an explicit rail mode, replacing a collapse path that never completed: collapsing now yields a fixed 48px icon rail with tooltipped links, and the mode and last expanded width are restored on reload. The narrow-viewport drawer still shows the full sidebar.
- Changed the sidebar's "Add layer" control from an inline single-field form to a dialog with a name field and a parent-layer select, so nesting no longer requires the slash-path syntax; it is unavailable until the layer list has loaded.
- **Breaking (pre-1.0):** changed reading history to per-device storage — browser `localStorage`, a `read_history.json` in the desktop config directory, and app-private storage on Android — instead of sending it to the server. Clearing history now works on read-only servers and Android, and server-side history from v0.8 is not migrated.
- **Breaking (pre-1.0):** changed reading time behind the dashboard heatmap and streak to per-device storage (`localStorage`, `reading_stats.json`, or Android app-private storage), so it keeps filling in offline and against read-only servers. Totals are per shelf, trimmed to 400 days; v0.8 reading time is not migrated, and because Android now issues no write requests its access token is only needed when the server sets `protect_read`.
- **Breaking (pre-1.0):** changed saved reading progress to per-device storage (web `localStorage`, desktop `reading_progress.json`, existing Android per-book `progress.json`) instead of the server store. Web and desktop progress starts at zero after upgrading; existing Android progress is unaffected, and server-side bookmarks are not migrated.
- Reading progress is now saved automatically on every client instead of through a bookmark button, buffered in memory and persisted at most once every 10 seconds with a final flush on leaving the reader. Desktop uses atomic replacement, so a crash between intervals loses at most the latest interval.
- **Breaking (pre-1.0):** changed Markdown chapter structure to live in the text itself: every ATX H2 line (`## Title`) outside a fenced code block begins a chapter, content before the first H2 is an opening section, and a TXT source always reads as a single section. Sources written before this stay legacy — they keep their stored split configuration, and the source editor offers an explicit conversion.
- Changed `format` to a source-level field: new `sources/{id}/meta.json` files carry an authoritative `format` and their own `schema_version: 1`, while `book.json.format` remains a compatibility mirror of the current source. A source whose schema version is newer than the running build stays readable, but its writes are refused before any file is touched.
- **Breaking (pre-1.0):** changed the Android client into a read-only reading client — metadata editing, source editing, cover upload, import, layer management, trash, and setting toggles are no longer offered, and mutating requests are rejected before they leave the device. Write capability became separate from the server's `read_only` config, so the read-only banner is reserved for a read-only server.
- Changed the book detail page layout: reading progress and the primary reading action stay in the first viewport on wide and narrow screens, secondary actions moved behind "Cover options" and "More" menus, and metadata is grouped into Publication, Content, and Notes sections.
- Changed the dashboard to put the reading heatmap above the stats cards.
- Changed invalid request data on several routes from a server error to a client error: an invalid layer name and a malformed BCP 47 language tag are now `400` with an explanatory message, mapped from a single domain-error table rather than per-route catch-alls.
- Changed the remaining hardcoded English error strings in frontend composables to go through i18n, so a zh-Hant user no longer sees English when one surfaces; 16 unused keys were removed.
- Changed the library toolbar to wrap below 760px instead of overflowing sideways, which had pushed the "Update book list" button off-screen on a phone.
- Upgraded Capacitor from 6.2.1 to 8.4.2 for the Android client, which raises the JDK requirement for Android builds to 21.
- Updated the Go toolchain to 1.26.6 across both modules, the Dockerfile, CI, and the setup documentation.

### Removed

- Removed the server-side reading-history API (`GET`/`POST`/`DELETE /api/shelves/:id/read_history`), its application-store storage, and the `read_history_limit` setting (the config key is now ignored if left in place).
- Removed the server-side reading-activity API (`GET`/`POST /api/shelves/:id/reading_activity`), the reading-time storage under `app/stats/reading/{YYYY-MM}.json`, and its background flush. Existing files are left in place and can be deleted.
- Removed the server-side reading-progress API (`GET`/`POST /api/shelves/:id/marks/:book_id`) and all server-side bookmark handling. Existing values are not migrated or read.

### Fixed

- Fixed the reader's chapter list showing the Markdown heading marker in section names — a book split on `^## ` listed sections as "## Chapter One" — by stripping the leading marker when deriving the name.
- Fixed deleting a book from its detail page returning to the unfiltered book list, dropping the layer being browsed; the view now returns to the deleted book's own layer.
- Fixed dropdown, select, and context menus growing past the bottom of the window with long option lists; popper menus are now capped to the available height (at most 320px) and scroll.
- Fixed book data writes that could leave the shelf inconsistent after an abrupt shutdown or failed request: covers and `trash.json` stage through a temp file, concurrent writers no longer collide on a shared temp filename, and a new or imported book becomes visible only once its source, current-source pointer, and metadata are all written.
- Fixed a cover upload race in which two overlapping uploads with different extensions could delete the image the book had just been pointed at.
- Fixed Android offline downloads being shared between shelves, because a book ID is only unique within one shelf: downloads are now stored per server and shelf. Books downloaded before this release are adopted by the connected shelf on first open; downloads against any other shelf need re-downloading.
- Fixed the sidebar's root-layer node listing nested books: the route builder collapsed an explicit root filter into "All books", and `/` is now preserved as a filter value.
- Fixed the shelf-wide statistics sweep overwriting a source's metadata with a stale snapshot; the book and its source are now resolved again immediately before writing.

### Security

- Updated `golang.org/x/text`, `golang.org/x/net`, `golang.org/x/crypto`, and related Go dependencies, and pinned `postcss` to 8.5.18, for reported-vulnerability fixes.
- Moved the Android client's pCloud access token out of plaintext preferences into Keystore-backed secure storage, keyed per shelf entry and excluded from backup and device-transfer extraction. The PlainShelf server token is kept the same way.
- Changed cover and source-asset responses to declare `Cache-Control: private` when `protect_read` is set, since the token travels in a header no shared cache keys on.
- Bounded the read paths a client can drive: source assets are streamed rather than read into memory, asset uploads are capped, asset names are validated identically on read and write against a flat directory, and pCloud downloads are bounded and cancellable.
- Resolved reported vulnerabilities in the frontend dependency tree (including a production `npm audit` pass) and took Dependabot bumps for `nanoid` and `github.com/go-git/go-git/v5` in the desktop module.
- Documented why PlainShelf registers no pCloud application and ships no app key, so the Android client's pCloud mode requires the user's own credentials.

## [v0.8.0] - 2026-07-21

### Added

- Added right-click context menus to book card view items, with actions for reading, viewing detail, opening the book folder (desktop only), downloading, editing, and deleting.
- Added Zoom In, Zoom Out, and Reset Zoom commands to the desktop app View menu (⌘=, ⌘-, ⌘0), with zoom level persisted across sessions.
- Added an experimental Android mobile app (a Capacitor shell around the existing frontend) that connects to a self-hosted PlainShelf server: first-run connection setup (server URL, optional access token, shelf selection) with a Settings entry to edit the connection later, persistent on-device caching of downloaded books and reading progress for offline reading (stored as app-private files via the Capacitor Filesystem plugin, exempt from WebView storage eviction), and native HTTP requests so plain-HTTP LAN servers work without CORS configuration.
- Added the native Android project under `frontend/android/`, `just` recipes for building it (`mobile-add-android`, `mobile-sync`, `build-mobile-android`, `open-mobile-android`), and a README section covering prerequisites and server reachability.
- Added PlainShelf launcher icons and splash screens (light and dark) for the Android app, generated from the brand images in `frontend/assets/` via `@capacitor/assets`.
- Added a dashboard home page (now the default landing route, with a sidebar entry) with stats cards (total books, added this month, star distribution, total characters), a tag cloud, a random book pick, and a reading heatmap driven by live daily reading-time data.
- Added server-side daily reading-time tracking (`POST`/`GET /api/shelves/:id/reading_activity`) backing the dashboard heatmap, plus an opt-in `char_count` field on the book list endpoint.
- Added a mobile-friendly book detail layout on narrow viewports: centered hero cover, centered title, a full-width Read button with secondary actions in a two-column grid, and single-column metadata rows.
- Added foldable sidebar sections (Layers, Reading, Maintenance, Admin) that collapse and expand independently.
- Added first/last page buttons and numbered page buttons with ellipsis to the pagination controls, alongside the existing prev/next controls.

### Changed

- Changed the main layout sidebar to an off-canvas drawer with a topbar menu button on viewports up to 768px wide (phones and narrow windows); it closes on backdrop tap or navigation, and wide-viewport splitter behavior is unchanged.
- Changed the book star rating input in the metadata editor to use reka-ui `RatingRoot` and `RatingItemIndicator`, aligning with the reka-ui component migration started in v0.7.0.
- Removed the inline Edit button from book card view items; edit access is now exclusively through the right-click context menu.
- Changed library search to pure frontend filtering instead of a backend query parameter, unifying case-insensitive title/author/tag/comment matching across desktop, mobile offline mode, and mock dev data.
- Changed release archives to also include the README preview image and a version-matched copy of `docs/`, so the README's relative documentation links work from the extracted archive as well as online.

### Fixed

- Fixed the mobile app showing errors instead of downloaded books when the device has connectivity but the PlainShelf server is unreachable (e.g. on mobile data away from the home LAN); book listing, metadata, sources, covers, and reading progress now fall back to the on-device offline cache on transport failures and timeouts, while real server error responses are still surfaced.
- Fixed resizable panel drag handles leaving all panel interactions unresponsive after a drag gesture; moved `hitAreaMargins` from an inline template literal to a module-level constant to prevent reka-ui drag-state corruption mid-drag.
- Fixed scrollable content areas in the sidebar and main content panel after the reka-ui Splitter migration; added inner wrapper elements to work around `SplitterPanel`'s `overflow: hidden` inline style enforcement.
- Fixed mobile book covers failing to load (showing as NO COVER) because Android WebView blocks mixed-content `<img>` requests against the app's `https://localhost` origin; covers are now fetched and rendered via `blob:` object URLs on mobile.
- Fixed the library page landing on the wrong page number when committing or clearing a search with client-side filtering active.
- Fixed the desktop app's Settings repository link opening inside the app window instead of the system browser.
- Fixed the compiled server binary silently dropping underscore-prefixed frontend asset files (e.g. the shared Vue export-helper chunk) from the embedded build, which blanked most pages when served from the binary.

## [v0.7.0] - 2026-07-05

### Added

- Added shelf creation and deletion confirmation modals to the frontend shelf management UI.
- Added `scan_interval` configuration support when creating shelves through the settings UI.
- Added `book_check_interval` shelf configuration option to rate-limit per-book staleness checks, reducing filesystem and network I/O on SMB mounts.
- Added `GET /api/shelves/:id/status` endpoint returning shelf readiness and initialization error state.
- Added ETag and `Cache-Control` HTTP caching headers for book cover responses.
- Added async shelf cache initialization; list endpoints return 503 with `Retry-After` until the cache is ready.
- Added frontend 503 auto-retry with "Shelf is loading…" status during shelf initialization, capped at 10 attempts with a manual retry button on failure.
- Added frontend `AbortController` request timeout on all API fetch calls with distinct timeout error handling.
- Added a shelf modify modal in desktop Settings for editing a shelf's name and scan interval.
- Added resizable sidebar and source-list panels (previously fixed-width) via drag handles.
- Added a Homebrew cask for installing the macOS desktop app (`brew install --cask voilelab/plainshelf/plainshelf`).
- Added a GitHub Actions release workflow publishing prebuilt server binaries (Linux amd64/arm64, macOS arm64) and a Docker image on tagged releases.
- Added application version reporting via `GET /api/version`, startup logs, and a Settings page About section, using build-time version injection.
- Added native fullscreen mode support for the macOS desktop app.

### Changed

- Hardened SMB mount support: configurable flock timeout (default 30 s), atomic writes for source and metadata files, and initialization failures now exposed via `/status` instead of hanging indefinitely.
- Made per-book stat checks asynchronous to reduce round-trips on SMB mounts; list operations serve from the in-memory cache between scheduled checks.
- Changed the Settings page from stacked panels to a tabbed layout (cover, read history, about, shelves).
- Changed native `<select>` inputs and hand-rolled dropdown menus throughout the app to a consistent reka-ui-based menu with shared styling and full keyboard navigation.
- Changed the layer tree to reka-ui Tree, adding keyboard navigation (arrow keys, Home/End, typeahead) and proper ARIA tree semantics.
- Changed the book tag input so Backspace on an empty field selects the last tag chip before deleting it on a second press (previously deleted immediately), and chips can now be selected with arrow keys.
- Changed reader side-action buttons to show styled hover tooltips instead of native browser title tooltips.

### Fixed

- Fixed cache write lock held across filesystem I/O in book cache refresh, blocking concurrent list reads.
- Fixed shelf lock acquisition errors not being propagated to callers.
- Fixed two data races in shelf book-cache scan-interval handling and shelf listing that could corrupt state under concurrent access.
- Fixed source ID collisions when multiple sources for the same book are created within the same second.
- Fixed layer/book path parsing to always split on `/` instead of the OS path separator, which broke on Windows.
- Fixed shelf startup to clear leftover temp files from a previous crashed run, and to skip unreadable/non-directory source entries instead of failing the whole listing.
- Fixed the desktop app crashing when the backend fails to start (e.g. data directory not creatable, port in use); it now shows an error dialog instead of panicking.

## [v0.6.0] - 2026-06-20

### Added

- Added drag-layer visual preview when repositioning layers via drag-and-drop.
- Added `SetCurrentSource` API endpoint and "Set as current" button in the book source editor.
- Added log-file listing and access API endpoints and a frontend admin log viewer with date-picker navigation.
- Added multi-shelf support: API endpoints are now shelf-scoped, with a new `GET /api/shelves` endpoint to list configured shelves.
- Added frontend shelf selector with live switching across configured shelves.
- Added duplicate-page delete action.
- Added drag-and-drop upload for book covers.
- Added `cover_to_jpg` conversion setting with API endpoints and a frontend settings page toggle.
- Added ASCII input support in the UTF-8 re-encoding path.
- Added configurable read history limit with API endpoints and a settings page control.
- Added layer rename and move support via API and frontend UI, including a dedicated rename modal.
- Added desktop shelf management: add new shelves and remove existing shelves from the settings page.
- Added book star ratings to the book detail and edit views.
- Added read-only server mode with write controls disabled in the frontend.

### Changed

- Changed book package directory extension from `.novl` to `.bookpkg`.
- Changed frontend license from ISC to BSD-3-Clause.
- Moved shelf selector from the top bar to the top of the left sidebar, above the layer tree.

### Fixed

- Fixed HTTP status code for book and log content stream responses in desktop mode.
- Fixed book cover upload to infer MIME type from filename extension when not supplied.
- Fixed desktop local import to route through the active shelf.
- Fixed empty active shelf fallback on startup.
- Fixed shelf routes being accessible before shelf data is fully loaded.
- Fixed line counter to handle long lines correctly.
- Fixed desktop shelf loading to retry on failure.
- Fixed desktop shelf persistence migration.
- Fixed book source metadata not refreshing after content update.

### Removed

- Removed single-shelf configuration; shelves are now managed exclusively through the multi-shelf configuration.
- Removed obsolete snapshot-to-source migration tool.

## [v0.5.0] - 2026-05-29

### Added

- Added shelf cache refresh controls and stale-book cache handling improvements.
- Added MkDocs-based project documentation and known-issues pages.
- Added a canvas-based simple book-cover generator in the frontend.
- Added desktop history navigation controls and menu actions.
- Added frontend drag-and-drop support for importing external TXT files.
- Added frontend i18n foundations with locale switching support (`en`, `zh-Hant`).
- Added a soft-delete trash feature for books with original-path retention.
- Added book source create/delete actions in the source editor.
- Added desktop native file-dialog import selection and server-side local-path import support.

### Changed

- Changed CI to include desktop module Go test coverage.

### Fixed

## [v0.4.0] - 2026-05-22

### Added

- Added configurable shelf logging output support through application logging configuration.
- Added experimental Wails GUI support for local desktop usage.
- Added frontend support for creating empty books.
- Added frontend support for editing a book's publish date.

### Changed

- Improved server API error logging to include richer response diagnostics.
- Refined logger argument handling and shelf-close error handling paths for more predictable shutdown behavior.
- Refined shelf configuration.
- Improved tag input UI in metadata editor.

### Fixed

- Fixed shelf logging integration issues after initial logger wiring.
- Fixed a potential race condition from shared error state in the server listen goroutine.
- Fixed log writer lifecycle handling to avoid closing standard I/O outputs while still closing closable writers.

## [v0.3.0] - 2026-05-20

### Changed

- Migrated book content storage from `snapshot` to `source` as the canonical field across server and frontend workflows.
- Updated API and data-model terminology to align with the `source`-based content lifecycle before future migrations.

## [v0.2.0] - 2026-05-18

### Added

- Added GitHub Actions CI coverage for Go tests and frontend builds.
- Added server-side API contract tests for core library and reader workflows.
- Added a book-detail download action with frontend error handling.
- Added an API endpoint for retrieving a specific book snapshot.
- Added current snapshot line and character counts to the book detail view.
- Added maintenance navigation icons for recently read, missing-field, and
  duplicate-content views.

### Changed

- Updated GitHub issue templates to improve issue reporting and triage.
- Aligned the frontend reader split setting with the boundary-based API contract.
- Clarified the supported security release policy.
- Removed the duplicate back button from the book detail view.

### Fixed

- Tightened server-side import validation for uploaded text formats.
- Fixed split configuration contract behavior covered by API tests.
- Fixed omitted security configuration handling and loopback listen-address
  detection.
- Hid the layer delete action when a layer still contains books.
- Fixed download error dismiss and reset behavior.

## [v0.1.1] - 2026-05-16

### Removed

- Removed the experimental GUI implementation so the project can focus on the
  local server and web-based reading workflow.
- Removed the experimental CLI implementation from the application surface.

### Security

- Hardened shelf path handling and upload handling.
- Added a project security policy in `SECURITY.md`.

### Fixed

- Improved handling of oversized request bodies by using typed `MaxBytesError`
  checks.
- Improved error reporting for large request bodies.
- Corrected the `plainshelf-srv` startup error log message.

## [v0.1.0] - 2026-05-16

### Added

- First early-development release of PlainShelf as a local-first personal
  reading library for plain text books.
- Web UI support for browsing, importing, organizing, editing, and reading TXT
  books.
- Filesystem-first shelf layout with stable internal book IDs that are
  independent from display titles.
- Server and frontend workflows for importing TXT files, re-encoding uploaded
  content to UTF-8, creating initial snapshots, and detecting book language when
  possible.
- Browser reader support with split configuration, snapshot viewing/editing,
  reading position persistence, font-size controls, chapter navigation, and
  keyboard navigation improvements.
- Layer-based organization, including layer creation/deletion, moving books
  between layers, layer book counts, and layer tree UI improvements.
- Search, sorting, pagination preferences, and route-query handling for the
  library view.
- Maintenance views for duplicate books, missing metadata, and recent reading
  history.
- Cover upload, retrieval, conversion to JPEG, and deletion support.
- Read history APIs and local store support.
- Library file locking with `gofrs/flock`.
- Docker support with an Ubuntu 24.04 runtime image, default container config,
  and a `/health` health check.
- Documentation for local development, Docker usage, and verification commands.

### Changed

- Renamed the project and packages from the earlier txtlib naming toward
  PlainShelf, including server, shelf, and frontend naming updates.
- Reworked the bookmark/store package organization and related methods.
- Refined reader, modal, toolbar, and frontend style organization.
- Replaced snapshot hashing with MD5 and standardized hash formatting.
- Updated runtime and Docker configuration defaults, including listen address,
  read/write timeouts, store paths, and data paths.
- Removed IndexedDB usage from the frontend.
- Limited import support to `.txt` files.

### Fixed

- Fixed library indexing when importing books.
- Fixed CLI usage and book ID handling during the pre-release development
  period.
- Fixed layer API behavior and filesystem ordering.
- Fixed frontend mock-data fallback behavior.
- Fixed tests and Go module dependency state.

### Known limitations

- PlainShelf is TXT-focused; EPUB, PDF, CBZ/CBR, DRM formats, OCR, cloud sync,
  multi-user support, public sharing links, and plugins are outside the current
  scope.
- Server-side pagination is not implemented yet; the frontend paginates the book
  list client-side.

## Reference tags

These repository tags are non-version markers kept for historical context:
