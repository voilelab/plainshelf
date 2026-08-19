# Changelog

All notable changes to PlainShelf are documented in this file.

This project is currently in pre-alpha / early development. APIs, data layout,
and UI behavior may still change between releases.

## [Unreleased]

### Added

- Added a character-count range filter to the book list. The library toolbar now carries a minimum and a maximum character count; leaving both empty keeps the list unfiltered, and setting either one narrows it. The bounds are held in the URL as `minChars` and `maxChars`, so a filtered view can be bookmarked and shared, and they combine with the folder, search, and sort controls already on the page. Bounds entered in reverse order are stored in order. A book whose character count cannot be read still counts as zero, and while the filter is on the toolbar reports how many of the listed books those are and offers the content-statistics sweep that computes them. Character counts are fetched only while a range is set, because asking the API for them makes the server open every book's current source. The filter is not offered on the Android client, whose pCloud mode would have to read every book's source metadata over the network to answer the same request.

### Changed

- Changed how a new book's ID is generated. It used to be the first 8 hex characters of a hash of the book's layer path and title, which is narrow enough that a shelf of 10,000 books had a 1.2% chance of two books sharing one, and the check meant to catch that could only see books the server already knew about — so a book added from another machine sharing the shelf, or copied in with a file manager, could end up with an ID a book already had, and the API and the Android client would then treat the two as one book. New books get a random 16-character ID instead, wide enough that a collision is not something a shelf can realistically produce. Existing books keep their IDs exactly as they are: nothing is migrated or recomputed, both forms work side by side in the same shelf, and reading progress, bookmarks, and Android downloads are unaffected.
- Changed source deletion so a book always keeps a usable current source. Deleting the current source hands `current_source` over to the newest remaining source before the folder is removed, and deleting a book's only source leaves an empty plain-text source behind, so `book.json` is never left pointing at a source that no longer exists. Deleting any other source is unchanged and still does not touch `book.json`. Because the hand-over writes `book.json`, deleting the current source of a book written by a newer PlainShelf is now refused with `409 Conflict`, the same as any other write to that book; other sources of such a book can still be deleted.

### Fixed

- Fixed a book becoming unreadable after its current source was deleted. The pointer was left naming the removed source, which made the book's content and split-config endpoints fail, blanked the book detail page down to an error message, and left the reader unable to load anything. Alongside the deletion fix, reads now fall back to the newest source a book still has whenever `current_source` cannot be resolved — which also repairs books already in this state, and books left there by a hand edit or a sync tool. The fallback does not rewrite `book.json`: the filesystem stays the source of truth. A book with no source at all is now reported as a missing source (`404`) instead of a server error (`500`).

### Removed

- Removed the **Maintenance → Low Character Count** page (`/books/maintenance/low-char-count`), replaced by the character-count range filter on the book list. The page could only express "at most N characters" and could not be combined with the folder, search, or sort controls; its "update content statistics" action is now offered by the new filter instead. The underlying API is unchanged.

## [v0.9.0] - 2026-08-15

### Added

- Added EPUB import. An `.epub` file selected in the import dialog, dropped on the library, or picked from the desktop file dialog is converted to text and stored as an ordinary book; the original archive is not kept. Reading-order text, table-of-contents chapter names, cover image, title, authors, language, description, identifiers, and publication date are carried over. Requires no new third-party dependency.
- Added a choice of EPUB output layout, offered in the import dialog once the selection contains an EPUB and defaulted from a new **Settings → Import** tab. The Markdown layout writes `##` chapter headings and stores the book as Markdown, so the reader's chapter list shows real chapter names; the plain-text layout writes bare title lines, which carry no chapter structure, so the reader treats the whole source as a single section. A second option controls whether the book description is written at the start of the text as well as into the book metadata.
- Added the `epub_import_strategy` server setting (`GET`, `POST`, and `DELETE /api/setting/epub_import_strategy`) and the matching `epub_import_strategy` config key, holding the default EPUB layout. It is applied server-side during import, so it also governs the desktop app's file picker, which imports without opening the dialog.
- Added a `strategy` field to the book import endpoint, letting one request override the configured default.
- Added [EPUB Import](docs/epub-import.md) documenting what is kept, what is dropped, how the two layouts differ, and the size limits.
- Added a pCloud connection mode to the Android client, which reads a shelf directly from a pCloud folder with no PlainShelf server involved. A phone cannot mount cloud storage the way a host can, so a server keeping its shelf on a mounted cloud drive was never any help to the phone. You supply your own pCloud application: PlainShelf registers none and ships no app key. Authorization runs in the system browser through pCloud's `poll_token` flow, which needs no redirect URL and no app secret, and reports which account and region it reached, so no region has to be chosen. You then name the shelf folder — the one containing `books/` — and the app checks it before saving. The mode is read-only like the rest of the Android client and has no access token or `protect_read` equivalent because there is no server. Downloads, reading progress, read history, and reading time stay on the device and are kept separate per shelf. The mode is experimental.
- Added a `schema_version` field to `book.json`, establishing schema v1 as the first versioned on-disk book format. Libraries created before this release have no version marker, are read as v1, and are upgraded lazily: opening a library never rewrites it, and the version is written to a book only the next time that book is modified.
- Added a compatibility policy and upgrade documentation covering the on-disk schema version, backing up and restoring a shelf, and what to do when PlainShelf refuses to write a book (`docs/concepts/data-format-versioning.md`).
- Added a "Low Character Count" maintenance page listing books whose character count is at or below a threshold set from the page header, reusing the shared maintenance book list, view modes, and pagination. Books with an unknown count are listed and reported separately in the header.
- Added source illustrations. A source can hold images in an `assets/` directory beside `source.txt`, reachable through `GET`, `PUT`, and `DELETE /api/shelves/:id/books/:book_id/sources/:source_id/assets/:asset_name`. A Markdown book renders a line that is nothing but `![alt](assets/…)` as a figure, so the link is an ordinary relative path that resolves the same way in a text editor; plain text has no image syntax and is unaffected. Nothing indexes the directory — the filesystem is the list — so `book.json` is unchanged and an older build reads such a shelf exactly as before. The directory is flat, uploads are capped at 20 MB, and reads are streamed rather than buffered. There is no UI for adding or removing an image yet.
- Added EPUB illustration import: an imported EPUB now stores the images its chapters referenced beside the converted text and links each one where it appeared, for the Markdown layout. The plain-text layout drops them as before. Whatever cannot be kept is still counted and recorded on the source, shown as "Import note" on the book detail page: formats the shelf will not serve (SVG among them), images inside an `<svg>` canvas, and anything over 8 MB alone or 64 MB for the book. A `keep_images` field on the EPUB import strategy turns the behavior off; it is reached through the config file or the API and has no settings control.
- Added illustrations to the Android client: downloading a book now stores the images its text renders, so an offline reader sees the same page, and a shelf read from pCloud resolves them from the recursive folder listing it already performs. A book downloaded before this shows alt text offline until it is downloaded again.
- Added reader font selection, with two bundled licensed fonts alongside the system default, chosen from the reader's side actions and stored per device. Their licenses are viewable in-app from Settings and are verified by a build-time check.
- Added rendering of a sanitized subset of HTML in the Markdown reader, so inline and block HTML written in a source appears as formatting instead of literal text. Scripts, event handlers, and external resource references are stripped.
- Added a chapter outline to the source editor, with focused editing of one Markdown chapter at a time instead of the whole file, and format conversion actions: TXT to Markdown (manual, by regex, or by fixed line count), Markdown to plain text, and an upgrade of a legacy split-configured source to Markdown chapters. Every conversion writes a new source and leaves the original intact.
- Added multi-select to the library, with batch move and batch delete run as a background task chain behind a progress modal, on both wide and narrow viewports. `POST /api/shelves/:id/book-batches` is the backing endpoint.
- Added Empty trash as a background task with a progress modal, because the sweep can take a while on a large or network-backed shelf. `POST /api/shelves/:id/trash/empty` returns 202 with a task chain ID and deletes books one at a time rather than holding the shelf lock for the whole sweep; a sweep already in flight for that shelf answers 409 with the running chain's ID so the client attaches to it. Task chain progress is read through `GET /api/taskchains/:id`.
- Added a shelf-wide content statistics refresh to the Low Character Count page, recomputing the line and character counts of every book whose current source reports none. `POST /api/shelves/:id/content-stat-refreshes` runs it as a task chain; the button is hidden on read-only runtimes and disabled when no book has an unknown count.
- Added an "Update content stats" action to the book detail page, backed by `POST /api/shelves/:id/books/:book_id/sources/:source_id/refresh`, which re-reads the source file and recomputes its MD5, line count, and character count. This makes the counts correctable after `source.txt` is edited outside PlainShelf, without re-uploading the content.
- Added a global default section split rule, set from a new **Settings → Reader** tab and stored through `GET`, `POST`, and `DELETE /api/setting/default_split_config`. A legacy source with no split configuration of its own falls back to it. The boundary type is rejected, and regex and line-count values are validated on write.
- Added `format` to the fields accepted by the book PATCH endpoint, so an import's guess can be corrected — most often a `.txt` file that actually holds Markdown. Both formats read the same bytes on disk, so the switch rewrites only metadata and is reversible in either direction.
- Added an immersive mobile reader: the chrome hides while reading, a tap in the middle brings it back, and the edges page forward and back. Font size, line height, and the reading font are set from a mobile reader settings sheet.
- Added multiple shelves to the Android client. The device now keeps a list of shelf entries, each carrying its own source type — a PlainShelf server or a pCloud folder — and its own credentials, so several libraries can sit side by side instead of each new connection overwriting the last. One entry is active at a time and is switched from the sidebar. Offline downloads keep the scope key they already used, so an upgraded install keeps them, and per-entry tokens moved into Keystore-backed storage.
- Added a persisted pCloud book listing and an "Update book list" button on the library toolbar. A launch now rebuilds the library from the stored copy with no request at all, and the listing is refreshed only when asked; an update re-downloads only the `book.json` files whose size or modification time changed.
- Added an exported book cache, `app/book-cache-{writer-id}.json`, written by the server and the desktop app. It holds every book's `book.json`, its package path, and the shelf's layers, so a client that reads the shelf directly — the Android client on pCloud — downloads one file instead of paying two requests per book, and re-reads a book individually only when its `book.json` is newer than the recorded walk. Exports are driven from the paths that already read or write the shelf rather than from a timer, and an export whose content matches the last one does not touch the disk.
- Added split-configuration caching to the Android offline download, so a downloaded book keeps its chapter list without reaching the server.

### Changed

- Changed the project scope to include EPUB as an import format. It remains excluded as a storage and rendering format: nothing on the shelf becomes a binary blob, and every imported book stays readable in a text editor. PDF, comic archives, DRM, and OCR are still out of scope.
- Books whose `book.json` was written by a newer PlainShelf build are now read-only rather than silently rewritten. They remain visible and readable, `schema_version` is reported in book API responses, and any attempt to modify them fails with `409 Conflict` instead of overwriting fields the running build does not understand. The refusal is checked before any filesystem change, so a rejected cover upload, cover deletion, or layer move leaves the book untouched.
- Changed the sidebar's collapse toggle into an explicit rail mode, replacing a collapse path that never completed and left the toggle stuck: collapsing now yields a fixed 48px icon rail with tooltipped navigation links, and both the mode and the last expanded width are restored on reload. The narrow-viewport drawer still shows the full sidebar.
- Changed the sidebar's "Add layer" control from an inline single-field form to a dialog with a layer name field and a parent-layer select, so nesting no longer requires knowing the slash-path syntax; a successful create navigates into the new layer. The control is now unavailable until the layer list has loaded, so the dialog cannot offer parents belonging to a shelf the user has already left.
- **Breaking (pre-1.0):** changed reading history to per-device storage: each client now records recently read books itself — browser `localStorage` on the web, a `read_history.json` file next to `shelves.json` in the desktop app's config directory, and app-private storage (needing no extra permission) on Android — instead of sending them to the server. Histories are kept per shelf, the retention limit became a per-device setting stored alongside them, and clearing history now works on read-only servers and on the Android client because it no longer writes to the shelf. Server-side history recorded by v0.8 is not migrated and no longer appears after upgrading.
- **Breaking (pre-1.0):** changed reading time to per-device storage: each client now records the daily reading seconds behind the dashboard heatmap and streak itself — browser `localStorage` on the web, a `reading_stats.json` file next to `shelves.json` in the desktop app's config directory, and app-private storage on Android — instead of reporting it to the server. The heatmap therefore keeps filling in while offline and against read-only servers, where the report was previously rejected, and totals are kept per shelf and trimmed to the last 400 days. Because the Android client now issues no write requests at all, its access token is only needed when the server sets `protect_read`. Reading time recorded by v0.8 is not migrated and no longer appears after upgrading.
- **Breaking (pre-1.0):** changed saved reading progress to per-device storage instead of the server application store. Web clients use `localStorage`, the desktop app writes `reading_progress.json` next to `shelves.json`, and Android keeps its existing app-private per-book `progress.json` files. Progress is independent per device and shelf. Existing server-side bookmarks are not migrated: Web and desktop progress starts at zero after upgrading, while existing Android progress is unaffected.
- Reading progress is now saved automatically on every client instead of through a bookmark button. Changed positions are buffered in memory and persisted at most once every 10 seconds, with a final flush when normally leaving the reader; unchanged positions produce no writes. Desktop keeps using atomic replacement for `reading_progress.json`, so closing the process between intervals can lose at most the latest interval rather than causing a partial file.
- **Breaking (pre-1.0):** changed Markdown chapter structure to live in the text itself instead of in a split configuration. Every ATX H2 line (`## Title`) outside a fenced code block begins a chapter and its text is the chapter name; content before the first H2 is an opening section, named by its first H1 when there is one. H3 and lower headings, `---`, and `***` never split. A TXT source is now deliberately unstructured and always reads as a single section. Sources written before this remain legacy sources: they keep their stored regex, line-count, or boundary split configuration and the legacy global default, and opening or saving one never changes its chapter semantics — the source editor offers an explicit conversion instead.
- Changed `format` to a source-level field. New `sources/{id}/meta.json` files carry an authoritative `format` and their own `schema_version: 1`; `book.json.format` remains as a compatibility mirror of the current source for older clients, and readers resolve the current source's `meta.json` first, then legacy `book.json.format`, then `txt`. Source schema v1 is written only when a source is created, including imports and conversions. A source whose schema version is newer than the running build stays readable, but its content, comment, split, asset, and delete operations are refused before any file is touched, matching the guarantee already made for `book.json`.
- **Breaking (pre-1.0):** changed the Android client into a read-only reading client. The whole write surface — metadata editing, source editing, cover upload, import, layer management, trash, and the server setting toggles — is no longer offered on the phone: mutating requests are rejected in the client before they leave the device, edit-only routes are blocked by the router guard, and the matching sidebar entries, including layers and logs, are hidden. Write capability became a concept separate from the server's `read_only` config, so the read-only banner stays reserved for a read-only server rather than firing on every Android session.
- Changed the book detail page layout: reading progress and the primary reading action are kept in the first viewport on both wide and narrow screens, secondary actions moved behind "Cover options" and "More" menus, and the metadata below is grouped into Publication, Content, and Notes sections.
- Changed the dashboard to put the reading heatmap at the top of the page, above the stats cards.
- Changed invalid request data on several routes from a server error to a client error: an invalid layer name, previously answered 409 on the layer routes and 500 on book creation, import, and move, and a malformed BCP 47 language tag, previously answered 500, are now both 400 with an explanatory message. Domain errors are mapped to HTTP statuses from a single table rather than per-route catch-alls.
- Changed the remaining hardcoded English error strings in frontend composables to go through i18n, so a zh-Hant user no longer sees English when one of them surfaces. English values are unchanged; 16 keys with no call site left were removed.
- Changed the library toolbar to wrap below 760px instead of overflowing sideways, which had squeezed the "Update book list" button to one character per line and parked it off-screen on a phone.
- Upgraded Capacitor from 6.2.1 to 8.4.2 for the Android client, which raises the JDK requirement for Android builds to 21.
- Updated the Go toolchain to 1.26.6 across both modules, the Dockerfile, CI, and the setup documentation.

### Removed

- Removed the server-side reading-history API (`GET`, `POST`, and `DELETE /api/shelves/:id/read_history`) together with its storage in the application store, and the `read_history_limit` server setting (`/api/setting/read_history_limit`; the `read_history_limit` config key is now ignored if left in an existing config file).
- Removed the server-side reading-activity API (`GET` and `POST /api/shelves/:id/reading_activity`) together with the shelf's reading-time storage under `app/stats/reading/{YYYY-MM}.json` and the background flush that maintained it. Existing files are left in place and can be deleted; nothing reads them any more.
- Removed the server-side reading-progress API (`GET` and `POST /api/shelves/:id/marks/:book_id`) and all server-side bookmark handling. Existing server-side values are not migrated or read.

### Fixed

- Fixed the reader's chapter list showing the Markdown heading marker in section names. A regex split matches the heading line itself, so a book split on `^## ` listed sections as "## Chapter One"; the leading marker is now stripped when deriving the name.
- Fixed deleting a book from its detail page returning to the unfiltered book list, dropping the layer the reader was browsing. The view now returns to the deleted book's own layer.
- Fixed dropdown, select, and context menus growing past the bottom of the window with long option lists, which made the lower entries unreachable; popper menus are now capped to the available height (at most 320px) and scroll.
- Fixed book data writes that could leave the shelf inconsistent after an abrupt shutdown or a failed request: covers stage through a temp file instead of being truncated in place, `trash.json` uses the same atomic path, concurrent writers to one book no longer collide on a shared temp filename, and a newly created or imported book becomes visible only once its source, current-source pointer, and metadata are all written.
- Fixed a cover upload race in which two overlapping uploads with different extensions could delete the image the book had just been pointed at, leaving `book.json` referencing a missing file.
- Fixed Android offline downloads being shared between shelves: because a book ID is only unique within one shelf, books with the same title and layer path on two shelves — or on two servers that both name a shelf `default_shelf` — cached to the same location and overwrote each other's text, sources, cover, and reading position, and the downloads list showed every shelf's books at once. Downloads are now stored per server and shelf, matching how reading history and reading time are already kept. Books downloaded before this release are adopted by the shelf the app is connected to the first time it opens; downloads made against any other shelf need to be downloaded again.
- Fixed the sidebar's root-layer node listing nested books. The route builder normalized `/` to the empty path, collapsing an explicit root filter into the unfiltered "All books" route; `/` is now preserved as a filter value.
- Fixed the shelf-wide statistics sweep overwriting a source's metadata with a stale snapshot. It held each opened source from the initial scan until that book's turn, so a split configuration or comment stored by another request in between was lost; the book and its source are now resolved again immediately before writing.

### Security

- Updated `golang.org/x/text`, `golang.org/x/net`, `golang.org/x/crypto`, and related Go dependencies, and pinned `postcss` to 8.5.18, to pick up fixes for reported vulnerabilities.
- Moved the Android client's pCloud access token out of plaintext preferences into Keystore-backed secure storage, keyed per shelf entry, and excluded it from Android backup and device-transfer extraction. The PlainShelf server token stored by the multi-shelf list is kept the same way.
- Changed cover and source-asset responses to declare `Cache-Control: private` when `protect_read` is set. The token travels in a header no shared cache keys on, so a stored copy could otherwise answer a later request that never reached the token gate. The cover route had always declared `public`.
- Bounded the read paths a client can drive: source assets are streamed rather than read fully into memory, asset uploads are capped, asset names are validated identically on read and write against a flat directory so an encoded separator cannot traverse out of it, and pCloud downloads are bounded and cancellable.
- Resolved the reported vulnerabilities in the frontend dependency tree, including a production `npm audit` pass, and took Dependabot bumps for `nanoid` (3.3.12 → 3.3.18) and `github.com/go-git/go-git/v5` (5.19.1 → 5.19.2) in the desktop module.
- Documented why PlainShelf registers no pCloud application and ships no app key, so the Android client's pCloud mode requires the user's own application credentials.

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
