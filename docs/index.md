# PlainShelf documentation

PlainShelf is a local-first, single-user reading library for plain text and
Markdown content. The shelf on disk is the source of truth; the application
adds a web interface, desktop integration, and an experimental Android client.

!!! warning "Pre-alpha"
    APIs, data layout, and UI behavior may change. Keep a current backup of the
    shelf and application store, especially before upgrades.

## Choose a path

### Use PlainShelf

1. [Install a release](installation.md) with Homebrew, a server archive, or Docker.
2. [Start a library](getting-started.md) and import a TXT or Markdown book.
3. Configure a [local shelf](configuring-local-shelf.md), or review the
   experimental [SMB setup](configuring-smb-shelf.md).

### Understand the storage model

- [Data Model](concepts/data-model.md) explains what is stored under a shelf.
- [Layers](concepts/layers.md) explains the nested folder hierarchy.
- [Shelf Cache and Disk I/O](concepts/shelf-cache-and-io.md) explains scanning,
  cache freshness, and network-filesystem tuning.

### Contribute

- [Local Development Setup](development/setup.md)
- [Android Development](development/android.md)
- [Docker](development/docker.md)
- [Known Issues](known-issue.md)

## Project boundaries

PlainShelf prioritizes readable local files, stable internal IDs, backup-friendly
storage, and a focused reading experience. EPUB, PDF, comic archives, DRM, OCR,
multi-user accounts, cloud sync, public sharing, and plugins are not part of the
current scope.

## Repository map

```text
cmd/plainshelf-srv/  server entry point
shelf/               filesystem-backed library core
server/              HTTP API and server runtime
frontend/            Vue web UI and Capacitor Android project
desktop/             Wails desktop client
internal/            shared internal Go packages
e2e/                 Playwright end-to-end tests
docs/                user and contributor documentation
```
