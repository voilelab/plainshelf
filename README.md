# PlainShelf

[![Go Reference](https://pkg.go.dev/badge/github.com/voilelab/plainshelf.svg)](https://pkg.go.dev/github.com/voilelab/plainshelf)
[![License](https://img.shields.io/badge/license-BSD_3--Clause-brightgreen.svg?style=flat)](LICENSE)
[![Lint](https://github.com/voilelab/plainshelf/actions/workflows/ci.yml/badge.svg)](https://github.com/voilelab/plainshelf/actions/workflows/ci.yml)

PlainShelf is a local-first, single-user reading library for lightweight text
content. It stores the library in human-readable files and provides web,
desktop, and experimental Android clients. Local-first is also a security
stance: no platform — including this one — holds your library or your reading
records, and no PlainShelf server holds your credentials. See
[Deployment and threat model](docs/deployment-and-threat-model.md) for what each
deployment does and does not protect.

> **Pre-alpha:** APIs, data layout, and UI behavior may change. Back up your
> shelf before upgrading — see [Backup and Restore](docs/backup-and-restore.md)
> for what to copy and how to put it back, and
> [Data Format Versioning](docs/concepts/data-format-versioning.md) for what the
> on-disk format does and does not guarantee.

![PlainShelf library preview](image.png)

## What it does

- Imports and reads plain text and Markdown books
- Imports EPUB files by converting them to plain text or Markdown
- Organizes books in nested, filesystem-backed folders
- Keeps stable book IDs when titles or folders change
- Supports covers, metadata, bookmarks, reading history, and reading stats
- Runs as a local server, a macOS desktop app, or an experimental Android client
  that reads from a server or straight from a shelf folder on pCloud

PlainShelf is not a Calibre replacement. PDF, comic archives, DRM, OCR,
multi-user accounts, cloud sync, public sharing, and plugins are outside the
current scope. The Android client can read a shelf held on pCloud, but that is a
read-only storage backend, not sync: nothing is written back.

EPUB is an import format, not a storage format: the text is extracted and stored
as an ordinary plain-text or Markdown book, and the original `.epub` is not kept.
[EPUB Import](docs/epub-import.md) covers what survives the conversion, including
the illustrations a Markdown import keeps.

## Install

On Apple Silicon macOS:

```bash
brew install --cask voilelab/plainshelf/plainshelf
```

Prebuilt server archives and Docker images are also available. See the
[installation guide](docs/installation.md), then follow
[Getting Started](docs/getting-started.md).

## Develop

```bash
npm --prefix frontend ci
npm --prefix frontend run build
go test ./...
```

The Go server embeds `frontend/dist`, so build the frontend before running Go
builds or tests. Full setup, desktop, mobile, Docker, and test instructions are
in the [development guide](docs/development/setup.md).

## Documentation

- [Documentation home](docs/index.md)
- [Installation](docs/installation.md)
- [Getting Started](docs/getting-started.md)
- [Local shelf configuration](docs/configuring-local-shelf.md)
- [SMB shelf configuration](docs/configuring-smb-shelf.md)
- [Deployment and threat model](docs/deployment-and-threat-model.md)
- [Data model](docs/concepts/data-model.md)
- [Known issues](docs/known-issue.md)

## License

[BSD 3-Clause](LICENSE)
