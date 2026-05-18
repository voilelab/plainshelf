# PlainShelf

[![Go Reference](https://pkg.go.dev/badge/github.com/voilelab/plainshelf.svg)](https://pkg.go.dev/github.com/voilelab/plainshelf)
[![License](https://img.shields.io/badge/license-BSD_3--Clause-brightgreen.svg?style=flat)](https://github.com/voilelab/plainshelf/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/voilelab/plainshelf)](https://goreportcard.com/report/github.com/voilelab/plainshelf)

PlainShelf is a local-first personal reading library for plain text books.

It is designed for single-user local usage, with a filesystem-first data model and a web-based reading interface.

> Status: pre-alpha / early development  
> APIs, data layout, and UI behavior may still change.

![mock data preview](image.png)

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

The following are not planned for the current v1 scope:

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
```

The current primary development focus is:

1. `shelf`
2. `server`
3. `frontend`

---

## Data Model

PlainShelf is filesystem-first.

A typical vault may look like this:

```text
lib/
├─ books/
└─ app/
```

### `books/`

Source of truth.

This contains user-owned data such as book metadata,
text files, covers, notes, and other long-lived files.

### `app/`

Runtime state.

---

## Development

### Run Only Frontend

```bash
cd frontend
npm install

# use mock data
VITE_USE_MOCK_API=true npm run dev
```

### Run server

```bash
# build frontend
cd frontend
npm install
npm run build
cd ..

# run server
mkdir workspace
cp cmd/plainshelf-srv/conf/config.yaml workspace/
cd workspace
go run ../cmd/plainshelf-srv/main.go -conf config.yaml
```

The default development config listens on `127.0.0.1:20000`, stores shelf and mark data under the current working directory, and enables `local_token` security for mutating `/api` requests. The server generates an ephemeral token at startup, injects it into the served frontend, and accepts it via `X-PlainShelf-Token` or `Authorization: Bearer <token>`.


### Run Electron desktop shell (experimental)

PlainShelf can also be launched through an experimental Electron shell. The Electron main process starts a separate `plainshelf-gui-sidecar` binary instead of reusing the daemon entrypoint, so GUI lifecycle, dynamic port selection, and desktop profile paths stay separate from `plainshelf-srv`.

```bash
cd frontend
npm install
npm run electron:dev
```

`npm run electron:dev` builds the Vue frontend, builds `cmd/plainshelf-gui-sidecar` into `bin/plainshelf-gui-sidecar`, then launches Electron. The sidecar binds `127.0.0.1:0`, emits a single JSON `ready` event on stdout with the actual URL and local API token, and serves the embedded frontend from that dynamic loopback URL. Logs are written to stderr.

Useful development overrides:

```bash
# Use a disposable desktop profile directory
PLAINSHELF_PROFILE=/tmp/plainshelf-gui npm run electron:dev

# Use a specific sidecar binary or address
PLAINSHELF_SIDECAR=/path/to/plainshelf-gui-sidecar \
PLAINSHELF_SIDECAR_ADDR=127.0.0.1:0 \
npm run electron:dev
```

The sidecar creates a desktop profile layout containing `shelf/`, `store/`, `logs/`, `backups/`, and `tmp/`. By default it uses an OS-specific application data directory and enables token protection for both read and mutating `/api` requests.

### Run server with Docker

Build the Ubuntu 24.04-based container image from the repository root:

```bash
docker build -t plainshelf .
```

Start the server on <http://localhost:20000> with persistent application data in a Docker volume:

```bash
docker run --rm \
  --name plainshelf \
  -p 127.0.0.1:20000:20000 \
  -v plainshelf-data:/data \
  plainshelf
```

The image uses `docker/config.yaml`, which listens on `0.0.0.0:20000`
inside the container, stores data in `/data/shelf` and `/data/store`, and explicitly sets `app_conf.security.mode: "none"` for compatibility with local-only port publishing. Keep the documented `127.0.0.1:20000:20000` port binding or put the container behind a trusted authentication boundary before exposing it beyond the local machine.
To use a custom server config, mount it over `/etc/plainshelf/config.yaml`:

```bash
docker run --rm \
  --name plainshelf \
  -p 127.0.0.1:20000:20000 \
  -v plainshelf-data:/data \
  -v "$PWD/path/to/config.yaml:/etc/plainshelf/config.yaml:ro" \
  plainshelf
```

### Run tests

```bash
npm --prefix frontend run build
go test ./...
```

---

## License

BSD 3-Clause
