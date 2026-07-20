# Local Development Setup

Use this page to build, test, and run PlainShelf from source. For release
installation, see [Installation](../installation.md).

## Prerequisites

| Tool | Minimum version | Purpose |
|---|---:|---|
| Go | 1.26.1 | server, core library, desktop backend |
| Node.js | 22 | frontend and end-to-end tests |
| npm | bundled with Node.js | JavaScript dependencies |
| just | recent | repository task runner |
| zsh | recent | shell used by the `justfile` |

Desktop development also needs the
[Wails platform dependencies](https://wails.io/docs/gettingstarted/installation).
Android development has [additional prerequisites](android.md).

## Install dependencies

From the repository root:

```bash
npm --prefix frontend ci
```

The end-to-end package installs its own dependencies when `just test-e2e` runs.

## Frontend-only development

Use mock data when backend behavior is not needed:

```bash
VITE_USE_MOCK_API=true npm --prefix frontend run dev
```

Vite serves the UI at <http://localhost:5173>.

## Run the local server

The Go server embeds the built frontend, so build it first:

```bash
just build-server-frontend
mkdir -p workspace
cp cmd/plainshelf-srv/conf/config.yaml workspace/config.yaml
cd workspace
go run ../cmd/plainshelf-srv/main.go -conf config.yaml
```

Open <http://127.0.0.1:20000>. The sample config stores data below the current
working directory and protects mutating API requests with an ephemeral local
token injected into the served frontend.

## Run checks

| Scope | Command |
|---|---|
| Frontend unit tests | `npm --prefix frontend test` |
| Frontend type-check and build | `npm --prefix frontend run build` |
| Go server and desktop tests | `just test-go` |
| End-to-end tests | `just test-e2e` |

Run the narrowest relevant check while iterating, then run the full check for
every area changed before opening a pull request.

## Desktop app

```bash
just run-desktop
```

Create a release-style desktop build with `just build-desktop`.

## Android app

The Android client is experimental and reuses the Vue frontend through
Capacitor. See [Android Development](android.md) for setup and device networking.

## Docker image

See [Docker](docker.md) to build and run the repository's container image.

## Code style

- Format Go with `gofmt` and keep `go test` green.
- Validate Vue and TypeScript with the frontend build.
- Add or update tests when behavior changes.
- Keep user-facing behavior in `docs/` and release notes in `CHANGELOG.md`.
