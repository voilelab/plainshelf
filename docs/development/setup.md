# Local Development Setup

Use this page to build, test, and run PlainShelf from source. For release
installation, see [Installation](../installation.md).

## Prerequisites

| Tool | Version | Purpose |
|---|---:|---|
| Go | 1.27.0 or newer | server, core library, desktop backend |
| Node.js | 22.18 to 23 | frontend and end-to-end tests |
| npm | bundled with Node.js | JavaScript dependencies |
| just | recent | repository task runner |
| zsh | recent | shell used by the `justfile` |

The 22.18 floor comes from Babel 8, which the mutation-testing dev dependency
pulls in; below it `npm ci` warns and fails outright where npm engine
enforcement is on.

Node.js has a ceiling as well as a floor. From Node.js 26 the runtime defines
its own experimental `localStorage` global, which leaves `window.localStorage`
undefined inside Vitest's jsdom environment; ten unit-test files then fail on
import with `Cannot read properties of undefined (reading 'getItem')`. The unit
suite passes on Node.js 22, which CI runs, and on 23; it fails on 26. Node.js 24
and 25 have not been tried.

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

Continuous integration adds three frontend gates to that list:
`npm --prefix frontend run check-boundaries`,
`npm --prefix frontend run check-exports`, and
`npm --prefix frontend run check-licenses`. The export check fails on an `export`
in `frontend/src` that no other non-test file reads, so a module's exports stay
its interface rather than a list of internals. Drop the `export` when nothing
needs it; when a unit test is the only reader and the seam is worth keeping,
record it in the allowlist at the top of `frontend/scripts/check-exports.mjs`.

The book package format has two independent readers — the Go shelf and the
pCloud client the Android app reads a shelf with — so both run the shared
fixtures in `shelf/testdata/conformance/`. A change to how a shelf is read
belongs in that dataset, and the frontend unit tests are where the pCloud side
of it fails. The dataset's own README explains the contract.

[Mutation testing](mutation-testing.md) is available for the reader's helpers and
the shared Markdown modules as an on-demand check of how much the unit tests
actually verify. It is not part of the checks above.

Dependency vulnerability scanning is CI-only and not in the table either.
`govulncheck ./...` runs in each of the three Go modules under both `GOOS=linux`
and `GOOS=darwin`, and gates the merge;
`npm audit --audit-level=high` runs against the `frontend` and `e2e` lockfiles
for information only. To reproduce a `govulncheck` failure locally, install it
with a Go at least as new as the one `go.mod` targets — built by an older Go it
fails while loading packages, with `requires newer Go version`, before it scans
anything. `SECURITY.md` explains why one gates and the other does not.

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

The Homebrew casks record the latest stable release and are updated after the
release artifacts exist by running `scripts/update-cask.sh <tag>`. That single
step pins both `plainshelf.rb` and `bookpkg-reader.rb` from the same tag, so the
desktop cask and the reader cask it depends on stay in sync.

## Desktop app

```bash
just run-desktop
```

On macOS the desktop app's **read** action opens the standalone reader in its own
window (it shells out to it). To test against the reader you are developing rather
than the brew-installed one, build it first with `just build-reader`; `just
run-desktop` then auto-detects `reader/build/bin/PlainShelfReader.app` and opens
reads with it. Override the target explicitly with `just run-desktop
/path/to/PlainShelfReader.app`, or by exporting `PLAINSHELF_READER_APP` (a `.app`
path/name is launched via `open -a`); an already-set `PLAINSHELF_READER_APP` takes
precedence. Because the dev and brew readers share a bundle id, keep only the one
you want registered with LaunchServices if `open -a` activates the wrong copy.

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
