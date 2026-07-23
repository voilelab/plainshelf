# PlainShelf

[![Go Reference](https://pkg.go.dev/badge/github.com/voilelab/plainshelf.svg)](https://pkg.go.dev/github.com/voilelab/plainshelf)
[![License](https://img.shields.io/badge/license-BSD_3--Clause-brightgreen.svg?style=flat)](LICENSE)
[![Lint](https://github.com/voilelab/plainshelf/actions/workflows/ci.yml/badge.svg)](https://github.com/voilelab/plainshelf/actions/workflows/ci.yml)

PlainShelf is a local-first, single-user reading library for lightweight text
content. It stores the library in human-readable files and provides web,
desktop, and experimental Android clients.

> **Pre-alpha:** APIs, data layout, and UI behavior may change. Back up your
> shelf before upgrading.

![PlainShelf library preview](image.png)

## What it does

- Imports and reads plain text and Markdown books
- Organizes books in nested, filesystem-backed folders
- Keeps stable book IDs when titles or folders change
- Supports covers, metadata, bookmarks, reading history, and reading stats
- Runs as a local server, a macOS desktop app, or an experimental Android client

PlainShelf is not a Calibre replacement. EPUB, PDF, comic archives, DRM, OCR,
multi-user accounts, cloud sync, public sharing, and plugins are outside the
current scope.

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
- [Data model](docs/concepts/data-model.md)
- [Known issues](docs/known-issue.md)

## License

[BSD 3-Clause](LICENSE)
