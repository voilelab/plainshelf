# Local Development Setup

Use this page to build, test, and run PlainShelf from source. For release
installation, see [Installation](../installation.md).

## Prerequisites

| Tool | Minimum version | Purpose |
|---|---:|---|
| Go | 1.26.6 | server, core library, desktop backend |
| Node.js | 22.18 | frontend and end-to-end tests |
| npm | bundled with Node.js | JavaScript dependencies |
| just | recent | repository task runner |
| zsh | recent | shell used by the `justfile` |

The frontend itself runs on any Node.js 22. The 22.18 floor comes from Babel 8,
which the mutation-testing dev dependency pulls in; below it `npm ci` warns and
fails outright where npm engine enforcement is on.

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

[Mutation testing](mutation-testing.md) is available for
`frontend/src/features/reader/utils/` as an on-demand check of how much the unit
tests actually verify. It is not part of the checks above.

## Versioning

Git release tags are the source of truth for the PlainShelf product version.
Tags must use `vMAJOR.MINOR.PATCH` with an optional SemVer prerelease suffix,
such as `v0.8.0` or `v0.8.0-beta.1`. Build scripts derive development versions
from the latest release tag, the number of subsequent commits, and the current
commit hash.

The server and the Settings **About** section expose the full build version. A
macOS bundle uses the numeric `MAJOR.MINOR.PATCH` core required by the platform,
so a `v0.8.0-beta.1` build reports `0.8.0` in its bundle metadata while retaining
the full prerelease version in the app. The private frontend and end-to-end npm
packages stay at `0.0.0`; their package metadata is not a product version.

The Homebrew cask records the latest stable release and is updated after a
release artifact exists by running `scripts/update-cask.sh <tag>`.

## Desktop app

```bash
just run-desktop
```

Create a release-style desktop build with `just build-desktop`.

## Android app

The Android client is experimental and reuses the Vue frontend through
Capacitor. It reads from a PlainShelf server or straight from a shelf folder on
pCloud. See [Android Development](android.md) for connection modes, setup, and
device networking.

## Docker image

See [Docker](docker.md) to build and run the repository's container image.

## Code style

- Format Go with `gofmt` and keep `go test` green.
- Validate Vue and TypeScript with the frontend build.
- Add or update tests when behavior changes.
- Keep user-facing behavior in `docs/` and release notes in `CHANGELOG.md`.
