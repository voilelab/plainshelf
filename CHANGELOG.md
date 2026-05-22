# Changelog

All notable changes to PlainShelf are documented in this file.

This project is currently in pre-alpha / early development. APIs, data layout,
and UI behavior may still change between releases.

## [Unreleased]

### Added

- Added configurable shelf logging output support through application logging configuration.
- Added experimental Wails GUI support for local desktop usage.
- Added create empty book functionality to frontend.
- Added edit book's publish date support to frontend.

### Changed

- Improved server API error logging to include richer response diagnostics.
- Refined logger argument handling and shelf-close error handling paths for more predictable shutdown behavior.
- Refine configuration of shelf.
- Improve tag input UI in metadata editor.

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

- `fyne-gui-last` - 2026-05-16
- `cli-last` - 2026-05-16

[Unreleased]: https://github.com/voilelab/plainshelf/compare/v0.3.0...HEAD
[v0.3.0]: https://github.com/voilelab/plainshelf/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/voilelab/plainshelf/compare/v0.1.1...v0.2.0
[v0.1.1]: https://github.com/voilelab/plainshelf/compare/v0.1.0...v0.1.1
[v0.1.0]: https://github.com/voilelab/plainshelf/releases/tag/v0.1.0
