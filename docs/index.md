# PlainShelf

[![Go Reference](https://pkg.go.dev/badge/github.com/voilelab/plainshelf.svg)](https://pkg.go.dev/github.com/voilelab/plainshelf)
[![License](https://img.shields.io/badge/license-BSD_3--Clause-brightgreen.svg?style=flat)](https://github.com/voilelab/plainshelf/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/voilelab/plainshelf)](https://goreportcard.com/report/github.com/voilelab/plainshelf)

PlainShelf is a local-first personal reading library for plain text books.

It is designed for single-user local usage, with a filesystem-first data model and a web-based reading interface.

!!! warning "Pre-alpha"
    PlainShelf is currently in **pre-alpha / early development**.
    APIs, data layout, and UI behavior may still change.

---

## Goals

- Manage and read TXT books
- Keep user data in local, human-readable files
- Use stable internal book IDs independent from display titles
- Provide a local web UI for browsing, importing, organizing, and reading
- Keep runtime state rebuildable
- Stay friendly to backup tools and Git-based workflows

PlainShelf is currently TXT-focused. Other text-like formats may be explored later, but plain text reading is the primary use case.

---

## Non-goals

The following are **not** planned for the current v1 scope:

- EPUB support
- PDF support
- CBZ / CBR support
- DRM formats
- OCR
- Multi-user support
- Cloud sync
- Public sharing links
- Plugin system

PlainShelf is not intended to be a full Calibre replacement.

---

## Project Structure

```text
cmd/
└─ plainshelf-srv/  # local server entrypoint

shelf/              # core library package
server/             # local HTTP server implementation
frontend/           # Vue web frontend
internal/           # internal shared utilities
desktop/            # experimental Wails desktop client
migration/          # shelf data migrations
```

The current primary development focus is `shelf`, `server`, and `frontend`.

---

## Next Steps

- [Getting Started](getting-started.md) — run PlainShelf locally in minutes
- [Data Model](concepts/data-model.md) — understand the filesystem-first shelf layout
- [Layers](concepts/layers.md) — organize books with a flexible layer hierarchy
