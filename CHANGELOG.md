# Changelog

All notable changes to PlainShelf are documented in this file.

The on-disk format freezes at `1.0.0-rc1`: from that release on, `book.json`,
source `meta.json` and `trash.json` take only backward-compatible changes, and
no further `Breaking (pre-1.0)` entry can touch them. Until then they can still
change. APIs and UI behavior are not covered by that freeze and may still change
between releases.

## [Unreleased]

### Added

- Added a **?** button to the mobile reader's controls that replays the gesture hint, which previously showed only once per device.
- Added a [Configuration reference](docs/reference/configuration.md) listing every key the server config file accepts with its type, default and effect, including the previously undocumented `cover_to_jpg`, `lock_mode`, `worker.*` and `logger.*` keys and the reserved-but-unimplemented `security.mode` values `password` and `external`.
- Added a default shelf directory to the desktop **Create shelf** dialog, which now previews the path and the shelf ID for the typed name and creates the shelf under the app data directory's `shelves/<shelf id>` when no directory is picked.
- Added each shelf's read-only state to `GET /api/shelves` (`read_only`), which the UI now uses to hide the write controls on a read-only shelf — importing, metadata/source/cover editing, folder creation and moves, deleting, trash restore — and show a banner saying so, while browsing, reading, searching and rescanning stay available.
- Added a notice on a book's detail page when its `book.json` was saved in a newer format than the reader supports, so a pCloud shelf on Android no longer shows an incomplete book as if it were complete.
- Added a request number to every API response (`X-Request-Id`, repeated as the error envelope's `incident`): eight Crockford base32 characters with no `0`, `O`, `1`, `I` or `L`, quoted in the request log line and, for an error the server did not expect, in a log line naming the code, method, path, shelf and full cause.
- Added the request number of the request that queued a background task chain to that chain's own log lines, so a batch move, empty-trash or fingerprint failure is searchable by the number the `202` returned.
- Added an **Error reference** notice that shows a failed request's number in the corner of the window with a copy button, prefixed `c-` when the interface minted the number itself (an uncaught interface error, or a pCloud shelf read with no server to ask) and absent entirely when there is no number to show.
- Added an adult-content mark that travels with the shelf: `content.nsfw_folders` in `shelf.json` marks a folder and every folder below it (a book's own `"nsfw": false` cannot cancel that), `"nsfw": true` in a `book.json` marks one book, and each entry of the exported book cache carries the assembled answer.
- Added a `show_nsfw` server setting (`GET`/`POST`/`DELETE /api/setting/show_nsfw`, off by default and not yet on the settings page) that hides every book the shelf marks as adult content across the whole API — the library listing, the duplicate and similarity results, the dashboard's totals, the folder tree, the fingerprint coverage count, and the batch and background sweeps — and answers `404` rather than `403` for a book, cover, content or source named directly; the exported book cache stays a complete mirror whatever it is set to.

### Changed

- Changed the similarity threshold slider to reka-ui `SliderRoot` and the character-range, reading-history-limit, log-retention, line-count and scan-interval/book-check-interval number boxes to reka-ui `NumberFieldRoot`, replacing every native `range`/`number` input whose track, thumb and spinner rendered differently in every browser and were absent on touch; the scan-interval amount now saves on blur, Enter or a stepper rather than on each keystroke.
- Changed the documented compatibility promise to freeze the on-disk format at `1.0.0-rc1` rather than `1.0.0`, so `book.json`, source `meta.json` and `trash.json` take only backward-compatible additions from that release on, while the `app/` caches, per-device reading state and unrecognized fields stay outside the promise.
- Changed the desktop **Create shelf** dialog to ask where the shelf goes as a two-way choice: **Create a new folder** (the default) needs only a name and shows the folder PlainShelf will create for it, while **Use a folder I already have** is the only branch offering the read-only toggle and a path to type or browse to.
- Changed a relative path typed into the create-shelf dialog to be refused on the form instead of by the backend's `shelf directory must be an absolute path`.
- Fixed **Update now** under **Mobile book cache** stopping at the first read-only shelf and skipping every shelf after it; the server writes no exported cache for such a shelf, so it is now passed over.
- Changed the cross-shelf transfer pickers to leave out read-only shelves, which could previously be picked as a destination only to be refused with `409`.
- Changed a read-only shelf to still offer **Copy to another shelf**, with the transfer dialog dropping its **Move** option; only a move is refused on a read-only source, because it ends by deleting the original.
- Changed the create-shelf dialog to drop its scan-interval and book-check-interval controls; a new shelf takes the defaults and both stay adjustable in **Modify**.
- Changed delete, empty-trash, and other destructive confirmations to alert dialogs: they announce as `alertdialog`, open with **Cancel** focused, and no longer close on a backdrop click (Esc still cancels).
- Changed the build to Go 1.27.1, whose reimplemented `encoding/json` decodes the shelf's JSON caches roughly 1.6-3x faster on the startup path.
- Changed the Docker image to build on Go 1.27.1 as well, catching up from 1.26.6 so an image build no longer downloads a second toolchain to satisfy `go.mod`.
- **Breaking (pre-1.0):** changed the API refusals that come from the error table from `text/plain` to a JSON body (`{"error":{"code":…,"message":…}}`) carrying a stable error code such as `BOOK_NOT_FOUND`; the routes that still call `http.Error` directly answer plain text unchanged, and a `500` body continues to withhold the cause.
- Changed the web UI to read that error envelope, so a refusal still shows its sentence rather than the raw JSON, while a plain-text refusal keeps its old text and takes its number from `X-Request-Id`.
- **Breaking (pre-1.0):** changed mutating API endpoints to read request bodies under `encoding/json/v2`'s rules, so a body that names the same member twice, carries invalid UTF-8, or spells a member name in the wrong case is now `400` instead of being silently resolved.
- Changed the `400` answer for a malformed request body to name the field the decoder stopped on (`invalid JSON at "folder"`), which the UI shows verbatim; setting writes no longer leak Go type names into the message.
- **Breaking (pre-1.0):** changed the shelf to read `book.json`, a source's `meta.json`, `trash.json` and the caches under `app/` with `encoding/json/v2`'s rules, so a field name must now match exactly (a hand-edited `"Title"` is no longer read as `title`, and the next save through the UI drops it), and a duplicate field name or invalid UTF-8 is refused rather than silently resolved.
- **Breaking (pre-1.0):** changed `shelf.json` to be read under those same rules, so a duplicate field name or invalid UTF-8 in it now falls back to the built-in scan defaults, which an unreadable `shelf.json` already did.
- Changed a refusal from a `book.json`, source `meta.json` or `trash.json` that could not be read to name the file and the member at fault (`books/Dune.bookpkg/book.json: duplicate object member name "title"`) in the log and, where a request reaches one, in a `409` carrying the new `MALFORMED_METADATA` code instead of a bare `500`; one unreadable book still costs only itself and the rest of the shelf lists as usual.
- **Breaking (pre-1.0):** changed every file the shelf writes to encode empty list and map fields as `[]` and `{}` rather than `null` — most visibly `"authors"` in `book.json` — so a reader no longer has to accept two spellings of "empty"; existing shelves open unchanged and are rewritten one file at a time as each is next saved.
- **Breaking (pre-1.0):** changed every API response to encode empty list and map fields as `[]` and `{}` rather than `null` — among them `authors`, `folder`, a task result's `succeeded_ids` and `failures`, and a task chain's `tasks` — so a client no longer needs a null fallback per field; the fields carrying `omitempty` (`tags`, `identifiers`, and the trash listing's `authors` and `original_folder`) stay absent while empty.
- **Breaking (pre-1.0):** changed a folder-transfer `409` to always carry `conflicting_book_ids`, sending `[]` for a folder-name conflict where the field was previously omitted.
- **Breaking (pre-1.0):** changed folder rename (`PATCH /api/shelves/{id}/folders/{path}`) and folder move (`POST /api/shelves/{id}/folder-moves`) to refuse a request body carrying data after the JSON value, which was previously ignored.
- **Breaking (pre-1.0):** changed the EPUB import's `strategy` form field to be read under `encoding/json/v2`'s rules like the request bodies, so a wrong-case or repeated member, or data after the JSON value, is now `400` instead of being silently resolved.
- Changed `book.json`, a source's `meta.json`, `trash.json` and the exported book cache to write `&`, `<` and `>` as themselves instead of as `\u0026`, `\u003c` and `\u003e`, so a title or comment containing them reads as typed in a text editor.
- Changed the desktop create- and modify-shelf forms to give every input a visible label, leaving the placeholders as example values.
- Changed the create-shelf flow to say **Create shelf** everywhere (dialog title, submit button, and the button that opens it).
- Changed the shelves settings section to offer **Create shelf** as its primary button while no shelf is configured.
- **Breaking (pre-1.0):** changed a `log_file` of `type: filename_rotate` with no `dir` to fail startup (`missing log dir: filename_rotate requires dir`) instead of starting a server that silently wrote no log at all; set `dir`, which both shipped example configs already do.

### Fixed

- Fixed the desktop create- and modify-shelf dialogs opening with focus on nothing; the caret now starts in the shelf name field.

### Removed

- Removed the `cmd/migrate-legacy-sources` tool, `internal/legacyupgrade`, and the dead `SplitConfig`/`SplitType` types and `split_config` metadata field; legacy sources still read unchanged and gain chapters via the source editor's TXT → Markdown conversion.

### Security

- Changed the security policy's supported branch from `main` to `dev`, which is where development happens and releases are cut from.
- Added a rate limit to the shelf rescan (`POST /api/shelves/{id}/scans`) of five walks back to back plus one every 10 seconds, refusing the rest with `429` and a `Retry-After` so a LAN device cannot hold the server's CPU and SMB bandwidth with a loop; `409` still means another walk is running, the cross-shelf transfer preflight is exempt, and it is not configurable.
- Updated `golang.org/x/image` to v0.45.0, closing a memory-exhaustion vector (GO-2026-6222) that an untrusted WebP cover could reach through the cover upload and EPUB import paths.
- Changed the shelf rescan (`POST /api/shelves/{id}/scans`) to require the access token only under `protect_read`, since it walks the shelf without writing, so **refresh the book list** no longer answers `401` under the shipped defaults; it stays behind the origin check, so a cross-origin refresh is still rejected.
- **Breaking (pre-1.0):** changed `GET /api/logs` and `/api/logs/{id}/content` to always require the access token, whatever `protect_read` says, since logs carry request paths, access times, and remote addresses; a client reading them without a token now gets `401`, and security mode `none` is unaffected.
- **Breaking (pre-1.0):** changed the Docker image default (`docker/config.yaml`) from `security.mode: "none"` to `local_token`, so mutating `/api` requests require a token and cross-origin (CSRF) writes are rejected; browsers on `127.0.0.1:20000` are unaffected (the token is injected into the served page), but opening the UI from a different host port, LAN IP, or custom domain now needs that exact origin in `app_conf.security.allowed_origins`. This is not an authentication boundary for an exposed port (a client reaching the port can read the token from the page), so keep binding to loopback or front the container with a real boundary.

## [v0.10.0] - 2026-08-26

### Added

- Added a min/max character-count range filter to the library toolbar (`minChars`/`maxChars` in the URL); not offered on Android.
- Added case-sensitive, whole-word, and regex find & replace to the source editor, with match highlighting, a "match x of N" count, and `$1`–`$9`/`$&` capture references.
- Added a working **Export file** action to the Android book detail page, saving the book's text into a shared `Documents/PlainShelf/` folder.
- Added a device-local reader launch preference (**Settings → Reading**) choosing whether **Read** opens a new reader or navigates the current window in place.
- Added aggregate progress and an **Abort** control to multi-file imports; desktop imports now run one file per call through the same dialog instead of freezing until the batch finishes.

### Changed

- **Breaking (pre-1.0):** renamed the "layer" concept to "folder" across API, URL, and UI (`/folders` routes, `folder` book key, `?folders=` filter); caches bump to schema v2 but the `books/` layout is unchanged, so a shelf opens with no migration.
- **Breaking (pre-1.0):** renamed the trash directory from hidden `.trash/` to visible `trash/`, migrated automatically on first open; it does not reverse, so an older build hides the trashed books until they are moved back by hand. See [Data Format Versioning](docs/concepts/data-format-versioning.md#shelf-layout-changes-are-not-versioned).
- Changed full shelf scans to skip re-listing folders whose mtime is unchanged, recorded in `app/scan-cache.json`; disable with the new `scan_cache: off` setting for mounts with untrustworthy directory times.
- Changed `GET /books?include=char_count` to answer from the in-memory cache instead of opening every source; the response shape is unchanged.
- Changed each book folder's hint file to `CURRENT_SOURCE.txt` (English; was `CURRENT_VERSION_LOCATION.txt`); nothing reads it back, so existing shelves need no migration.
- Changed the source editor to CodeMirror 6, holding the whole source so find, chapter jumps, and the caret land where asked; Markdown is syntax-highlighted and CRLF endings are preserved.
- Changed new book IDs from an 8-hex title/path hash to a random v4 UUID; existing IDs are kept with no migration and both forms coexist in one shelf.
- **Breaking (pre-1.0):** changed the per-device reading-progress file to a timestamped format (document v2) so the desktop app and the standalone reader reconcile concurrent writes newest-wins; the older v1 document is not migrated, so web and desktop progress starts at zero once after upgrading (Android's per-book `progress.json` is unaffected).
- Changed the Android client to require a book be downloaded before reading; the reader route redirects a not-yet-downloaded book to its detail page to prompt a download.
- Changed source deletion so a book always keeps a usable current source; deleting the current source of a book written by a newer PlainShelf is refused with `409 Conflict`.
- Reworked the dashboard into a **Home** page at `/home` (`/` and `/dashboard` redirect) that routes into the library, leading with "recently reading" and "recently added" and turning counts and tag chips into links.
- Changed similar-books duplicate detection to gate on a work budget instead of a raw book count; an over-budget shelf returns `200` with an explicit rejection body.

### Fixed

- Fixed a book becoming unreadable after its current source was deleted; reads fall back to the newest remaining source, and a sourceless book returns `404` instead of `500`.
- Fixed the Android pCloud walker listing NAS/sync helper directories (`@eaDir`, `#recycle`, `$RECYCLE.BIN`, `lost+found`, dot-dirs) as folders; it now skips them like the server.
- Fixed reader arrow-key chapter navigation going dead after clicking a button; the keys move a caret only while a text field is focused and otherwise turn the page.

### Removed

- Removed the **Maintenance → Low Character Count** page (`/books/maintenance/low-char-count`), replaced by the character-count range filter; the underlying API is unchanged.

## [v0.9.0] - 2026-08-15

### Added

- Added EPUB import: an `.epub` is converted to text and stored as an ordinary book, carrying over text, chapters, cover, and metadata; the original archive is not kept and no new dependency is required.
- Added a choice of EPUB output layout — Markdown with `##` headings or plain text — in the import dialog and a new **Settings → Import** tab, plus an option to write the description into the text.
- Added the `epub_import_strategy` server setting (`GET`/`POST`/`DELETE /api/setting/epub_import_strategy`) holding the default EPUB layout, applied server-side so it also governs the desktop file picker.
- Added a `strategy` field to the book import endpoint, letting one request override the configured default.
- Added [EPUB Import](docs/epub-import.md) documentation.
- Added a pCloud connection mode to the Android client, reading a shelf directly from a pCloud folder with no PlainShelf server; read-only and experimental, using your own pCloud app credentials.
- Added a `schema_version` field to `book.json`, establishing schema v1; pre-existing books have no marker, are read as v1, and are upgraded lazily on next modification.
- Added a compatibility and upgrade policy covering the on-disk schema version, backup/restore, and write refusals (`docs/concepts/data-format-versioning.md`).
- Added a "Low Character Count" maintenance page listing books at or below a character-count threshold.
- Added source illustrations: a source can hold images in an `assets/` directory (`GET`/`PUT`/`DELETE /api/shelves/:id/books/:book_id/sources/:source_id/assets/:asset_name`), rendered as figures in Markdown books; uploads capped at 20 MB, no add-image UI yet.
- Added EPUB illustration import for the Markdown layout, storing referenced images and reporting whatever cannot be kept as an "Import note"; a `keep_images` field turns it off.
- Added illustrations to the Android client, stored on download or resolved from a pCloud folder listing; a book downloaded before this shows alt text until re-downloaded.
- Added reader font selection with two bundled licensed fonts, chosen from the reader's side actions and stored per device.
- Added rendering of a sanitized subset of HTML in the Markdown reader; scripts, event handlers, and external resource references are stripped.
- Added a chapter outline to the source editor with focused per-chapter editing and format-conversion actions; every conversion writes a new source and leaves the original intact.
- Added multi-select to the library with batch move and batch delete run as a background task chain (`POST /api/shelves/:id/book-batches`).
- Added Empty trash as a background task (`POST /api/shelves/:id/trash/empty`, `202` with a chain ID; `409` if one is already running), with progress via `GET /api/taskchains/:id`.
- Added a shelf-wide content statistics refresh to the Low Character Count page (`POST /api/shelves/:id/content-stat-refreshes`); the button is hidden on read-only runtimes.
- Added an "Update content stats" action to the book detail page (`POST /api/shelves/:id/books/:book_id/sources/:source_id/refresh`), recomputing MD5, line, and character counts after `source.txt` is edited externally.
- Added a global default section split rule (**Settings → Reader**, `GET`/`POST`/`DELETE /api/setting/default_split_config`) that a legacy source with no split configuration falls back to.
- Added `format` to the book PATCH endpoint, so an import's guess (most often a `.txt` holding Markdown) can be corrected reversibly.
- Added an immersive mobile reader: chrome hides while reading, a middle tap restores it, edges page, and font size, line height, and font are set from a settings sheet.
- Added multiple shelves to the Android client, each with its own source type and credentials, switched from the sidebar; per-entry tokens moved into Keystore-backed storage.
- Added a persisted pCloud book listing and an "Update book list" button; a launch rebuilds the library from the stored copy, and an update re-downloads only changed `book.json` files.
- Added an exported book cache, `app/book-cache-{writer-id}.json`, so a client reading the shelf directly (Android on pCloud) downloads one file instead of two requests per book.
- Added split-configuration caching to the Android offline download, so a downloaded book keeps its chapter list without reaching the server.

### Changed

- Changed the project scope to include EPUB as an import format; it remains excluded as a storage/rendering format, and PDF, comic archives, DRM, and OCR are still out of scope.
- Books whose `book.json` was written by a newer build are now read-only rather than silently rewritten; any modification fails with `409 Conflict` before any filesystem change.
- Changed the sidebar's collapse toggle into an explicit rail mode — a fixed 48px icon rail with tooltipped links — with the mode and last expanded width restored on reload.
- Changed the sidebar's "Add layer" control to a dialog with a name field and parent-layer select, so nesting no longer requires slash-path syntax.
- **Breaking (pre-1.0):** changed reading history to per-device storage (browser `localStorage`, desktop `read_history.json`, Android app-private); it now works on read-only servers, and v0.8 server-side history is not migrated.
- **Breaking (pre-1.0):** changed dashboard reading time to per-device storage (`localStorage`, `reading_stats.json`, or Android app-private), per shelf and trimmed to 400 days; v0.8 reading time is not migrated.
- **Breaking (pre-1.0):** changed saved reading progress to per-device storage (`localStorage`, desktop `reading_progress.json`, Android per-book `progress.json`); web/desktop progress starts at zero after upgrading and server-side bookmarks are not migrated.
- Reading progress is now saved automatically instead of via a bookmark button, buffered and persisted at most once every 10 seconds with a final flush on leaving the reader.
- **Breaking (pre-1.0):** changed Markdown chapter structure to live in the text — each ATX H2 line outside a fenced code block begins a chapter — while TXT reads as one section; sources written before this stay legacy and offer an explicit conversion.
- Changed `format` to a source-level field: new `sources/{id}/meta.json` files carry an authoritative `format` and their own `schema_version`, and `book.json.format` mirrors the current source.
- **Breaking (pre-1.0):** changed the Android client into a read-only reading client — editing, upload, import, and management are gone, and mutating requests are rejected on-device, separate from the server's `read_only` config.
- Changed the book detail page layout: the primary reading action stays in the first viewport, secondary actions moved behind "Cover options" and "More" menus, and metadata is grouped into sections.
- Changed the dashboard to put the reading heatmap above the stats cards.
- Changed an invalid layer name and a malformed BCP 47 language tag on several routes from a `500` to a `400` with an explanatory message.
- Changed remaining hardcoded English error strings in frontend composables to go through i18n; 16 unused keys were removed.
- Changed the library toolbar to wrap below 760px instead of overflowing sideways, which had pushed the "Update book list" button off-screen on a phone.
- Upgraded Capacitor 6.2.1 → 8.4.2 for the Android client, raising the JDK requirement for Android builds to 21.
- Updated the Go toolchain to 1.26.6 across both modules, the Dockerfile, CI, and setup documentation.

### Removed

- Removed the server-side reading-history API (`GET`/`POST`/`DELETE /api/shelves/:id/read_history`), its storage, and the `read_history_limit` setting.
- Removed the server-side reading-activity API (`GET`/`POST /api/shelves/:id/reading_activity`) and its `app/stats/reading/{YYYY-MM}.json` storage; existing files can be deleted.
- Removed the server-side reading-progress API (`GET`/`POST /api/shelves/:id/marks/:book_id`) and all server-side bookmark handling; existing values are not migrated.

### Fixed

- Fixed the reader's chapter list showing the Markdown heading marker in section names by stripping the leading marker when deriving the name.
- Fixed deleting a book from its detail page returning to the unfiltered list; the view now returns to the deleted book's own layer.
- Fixed dropdown, select, and context menus growing past the bottom of the window; popper menus are now capped (at most 320px) and scroll.
- Fixed book data writes that could leave the shelf inconsistent after a crash or failed request: covers and `trash.json` stage through a temp file, temp filenames no longer collide, and a new book appears only once fully written.
- Fixed a cover upload race in which two overlapping uploads with different extensions could delete the image the book was just pointed at.
- Fixed Android offline downloads being shared between shelves; downloads are now stored per server and shelf, and pre-existing ones are adopted by the connected shelf on first open.
- Fixed the sidebar's root-layer node listing nested books; `/` is now preserved as a filter value instead of collapsing into "All books".
- Fixed the shelf-wide statistics sweep overwriting a source's metadata with a stale snapshot; the book and source are re-resolved immediately before writing.

### Security

- Updated `golang.org/x/text`, `golang.org/x/net`, `golang.org/x/crypto`, and related Go dependencies, and pinned `postcss` to 8.5.18, for reported-vulnerability fixes.
- Moved the Android client's pCloud and server access tokens out of plaintext preferences into Keystore-backed secure storage, excluded from backup and device-transfer extraction.
- Changed cover and source-asset responses to declare `Cache-Control: private` when `protect_read` is set.
- Bounded client-driven read paths: source assets are streamed, asset uploads are capped, asset names are validated identically on read and write, and pCloud downloads are bounded and cancellable.
- Resolved reported frontend-dependency vulnerabilities (including a production `npm audit` pass) and took Dependabot bumps for `nanoid` and `go-git/v5`.
- Documented why PlainShelf registers no pCloud application and ships no app key, so the Android pCloud mode requires the user's own credentials.

## [v0.8.0] - 2026-07-21

### Added

- Added right-click context menus to book card view items, with actions for reading, viewing detail, opening the book folder (desktop only), downloading, editing, and deleting.
- Added Zoom In, Zoom Out, and Reset Zoom commands to the desktop app View menu (⌘=, ⌘-, ⌘0), with zoom level persisted across sessions.
- Added an experimental Android app (a Capacitor shell around the frontend) connecting to a self-hosted server, with first-run connection setup, on-device caching of books and progress for offline reading, and native HTTP so plain-HTTP LAN servers work without CORS.
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

- Fixed the mobile app showing errors instead of downloaded books when the server is unreachable but the device has connectivity; reads now fall back to the on-device cache on transport failures while real error responses are still surfaced.
- Fixed resizable panel drag handles leaving panel interactions unresponsive after a drag; `hitAreaMargins` moved to a module-level constant to prevent reka-ui drag-state corruption.
- Fixed scrollable content in the sidebar and main panel after the reka-ui Splitter migration by adding inner wrappers around `SplitterPanel`'s `overflow: hidden`.
- Fixed mobile book covers failing to load because Android WebView blocks mixed-content `<img>` requests; covers are now fetched and rendered via `blob:` URLs on mobile.
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
