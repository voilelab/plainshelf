# PlainShelf documentation

PlainShelf is a local-first, single-user reading library for plain text and
Markdown content. The shelf on disk is the source of truth; the application
adds a web interface, desktop integration, and an experimental Android client.

!!! warning "Before 1.0"
    The on-disk format freezes at `1.0.0-rc1`; until that release it can still
    change, and APIs and UI behavior stay changeable after it. Keep a current
    backup of the shelf and application store, especially before upgrades. See
    [Data Format Versioning](concepts/data-format-versioning.md) for what the
    on-disk format does and does not guarantee.

## Choose a path

### Use PlainShelf

1. [Install a release](installation.md) with Homebrew, a server archive, or Docker.
2. [Start a library](getting-started.md) and import a TXT, Markdown or EPUB book.
3. Configure a [local shelf](configuring-local-shelf.md), or review the
   best-effort [SMB setup](configuring-smb-shelf.md). Every key the config file
   accepts is listed in the
   [Configuration reference](reference/configuration.md).
4. Review [EPUB Import](epub-import.md) for how EPUB files are converted.
5. Review [Logs](logs.md) for reading the application log and for how long it
   is kept.

### Understand the storage model

- [Architecture](concepts/architecture.md) shows how the clients, the server
  and the shelf fit together, and what reading state is kept off the shelf.
- [Data Model](concepts/data-model.md) explains what is stored under a shelf.
- [Data Format Versioning](concepts/data-format-versioning.md) explains the
  on-disk schema version and the compatibility policy.
- [Backup and Restore](backup-and-restore.md) covers what to copy, what a
  shelf-only copy leaves behind, and how to put a backup back.
- [Folders](concepts/folders.md) explains the nested folder hierarchy.
- [Shelf Cache and Disk I/O](concepts/shelf-cache-and-io.md) explains scanning,
  cache freshness, and network-filesystem tuning.

### Contribute

- [Local Development Setup](development/setup.md)
- [Android Development](development/android.md)
- [Docker](development/docker.md)
- [JSON Encoding](development/json-encoding.md)
- [Known Issues](known-issue.md)

## Project boundaries

PlainShelf prioritizes readable local files, stable internal IDs, backup-friendly
storage, and a focused reading experience. PDF, comic archives, DRM, OCR,
multi-user accounts, cloud sync, public sharing, and plugins are not part of the
current scope. The Android client can read a shelf held on pCloud
([Android Development](development/android.md#read-a-shelf-from-pcloud)), but
that is a read-only storage backend, not sync: nothing is written back and no
other client is aware of it.

EPUB is an import format, not a storage format. An imported EPUB is converted to
plain text or Markdown and stored like any other book; the original `.epub` is
not retained. What survives the conversion, illustrations included, is in [EPUB
import](epub-import.md). Everything on the shelf stays readable in a text editor.

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
