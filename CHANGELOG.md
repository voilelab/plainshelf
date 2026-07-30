# Changelog

All notable changes to PlainShelf are documented in this file.

This project is currently in pre-alpha / early development. APIs, data layout,
and UI behavior may still change between releases.

## [Unreleased]

### Added

- Added a `schema_version` field to `book.json`, establishing schema v1 as the first versioned on-disk book format. Libraries created before this release have no version marker, are read as v1, and are upgraded lazily: opening a library never rewrites it, and the version is written to a book only the next time that book is modified.
- Added a compatibility policy and upgrade documentation covering the on-disk schema version, backing up and restoring a shelf, and what to do when PlainShelf refuses to write a book (`docs/concepts/data-format-versioning.md`).
- Added a "Low Character Count" maintenance page listing books whose character count is at or below a threshold set from the page header, reusing the shared maintenance book list, view modes, and pagination. Books with an unknown count are listed and reported separately in the header.

### Changed

- Books whose `book.json` was written by a newer PlainShelf build are now read-only rather than silently rewritten. They remain visible and readable, `schema_version` is reported in book API responses, and any attempt to modify them fails with `409 Conflict` instead of overwriting fields the running build does not understand. The refusal is checked before any filesystem change, so a rejected cover upload, cover deletion, or layer move leaves the book untouched.
- Changed the sidebar's collapse toggle into an explicit rail mode, replacing a collapse path that never completed and left the toggle stuck: collapsing now yields a fixed 48px icon rail with tooltipped navigation links, and both the mode and the last expanded width are restored on reload. The narrow-viewport drawer still shows the full sidebar.
- Changed the sidebar's "Add layer" control from an inline single-field form to a dialog with a layer name field and a parent-layer select, so nesting no longer requires knowing the slash-path syntax; a successful create navigates into the new layer. The control is now unavailable until the layer list has loaded, so the dialog cannot offer parents belonging to a shelf the user has already left.

### Fixed

- Fixed dropdown, select, and context menus growing past the bottom of the window with long option lists, which made the lower entries unreachable; popper menus are now capped to the available height (at most 320px) and scroll.
- Fixed book data writes that could leave the shelf inconsistent after an abrupt shutdown or a failed request: covers stage through a temp file instead of being truncated in place, `trash.json` uses the same atomic path, concurrent writers to one book no longer collide on a shared temp filename, and a newly created or imported book becomes visible only once its source, current-source pointer, and metadata are all written.
- Fixed a cover upload race in which two overlapping uploads with different extensions could delete the image the book had just been pointed at, leaving `book.json` referencing a missing file.

### Security

- Updated `golang.org/x/text`, `golang.org/x/net`, `golang.org/x/crypto`, and related Go dependencies, and pinned `postcss` to 8.5.18, to pick up fixes for reported vulnerabilities.

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
