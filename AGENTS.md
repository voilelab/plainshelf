# PlainShelf — agent project guide

PlainShelf is a local-first reading library approaching its `1.0.0-rc1` release
candidate, at which the on-disk format freezes. The Go server embeds a Vue
frontend and is also used by Wails desktop and experimental Capacitor Android
clients.

## Product constraints

- The shelf filesystem is the source of truth; do not introduce a database as
  the authoritative book store.
- Stable book IDs must survive title changes and moves between folders.
- Keep single-user, local/private operation as the default.
- PDF, comic archives, DRM, OCR, multi-user accounts, cloud sync, public
  sharing, and plugins are outside the current scope unless the user changes it.
- EPUB is an import format only: it is converted to text/Markdown at import and
  the original file is not stored. Do not add EPUB rendering or storage.
- The Android client may read a shelf from pCloud, but only as a read-only
  storage backend. That is not cloud sync: it writes nothing back.
- Treat data-format, public API, and security changes as compatibility-sensitive.

## Repository map

| Path | Responsibility |
|---|---|
| `cmd/` | server and migration entry points |
| `shelf/` | filesystem-backed library core |
| `server/` | HTTP API, security, and runtime |
| `internal/` | shared Go internals: EPUB import, hashing, sketches, version |
| `frontend/` | Vue UI and Capacitor Android project |
| `desktop/` | Wails desktop client |
| `reader/` | Wails standalone `.bookpkg` reader app |
| `mobile/` | gomobile-bound embedded server for the Android app |
| `e2e/` | Playwright end-to-end tests |
| `docs/` | user and contributor documentation |
| `scripts/` | version and toolchain helpers called by `justfile` and CI |

## Commands

The Go build embeds `frontend/dist`; build the frontend before Go builds or
tests when that directory is absent or stale.

| Scope | Command |
|---|---|
| Install frontend deps | `npm --prefix frontend ci` |
| Frontend unit tests | `npm --prefix frontend test` |
| Frontend type-check + build | `npm --prefix frontend run build` |
| Frontend module boundaries | `npm --prefix frontend run check-boundaries` |
| Frontend unused exports | `npm --prefix frontend run check-exports` |
| Frontend dependency licenses | `npm --prefix frontend run check-licenses` |
| Version resolver | `./scripts/test-resolve-version.sh` |
| Main Go module | `go test ./...` |
| Desktop and reader Go modules | `cd desktop && go test ./...`, `cd reader && go test ./...` |
| Go lint (all three modules) | `golangci-lint run` in the root, `desktop`, and `reader` |
| All Go tests with frontend build | `just test-go` |
| End-to-end tests | `just test-e2e` |
| Wasm demo smoke | `just test-wasm-demo` |
| Mock frontend | `VITE_USE_MOCK_API=true npm --prefix frontend run dev` |

The boundary, export, license, version-resolver, and wasm demo smoke checks are
pull-request gates in `.github/workflows/ci.yml`; its Android build and Android
Go library jobs are not gates yet.

`just` uses `zsh`. If either is unavailable, run the underlying commands from
the `justfile`. In restricted environments where `sharp` cannot download its
optional binary, `npm --prefix frontend ci --ignore-scripts` is sufficient for
web builds; do not use it for Capacitor asset generation.

## Working rules

1. Inspect the relevant code, tests, and nearby documentation before editing.
2. Preserve unrelated working-tree changes and avoid broad refactors unless
   they are part of the request.
3. Add or update tests for behavior changes, at the level
   `docs/development/testing-levels.md` selects. Run the narrowest relevant
   check while iterating and the full affected-area check before completion.
4. For server API changes, read the matching
   contract package under `server/contract/` and preserve the `local_token`
   security boundary.
5. For changes to how a shelf is read or written, update the fixtures in
   `shelf/testdata/conformance/`. The Go shelf is the only reader of the format,
   the Android app's pCloud shelf included; the frontend parses none of it.
6. In the frontend, the mobile stack reaches the bundle only through
   `src/shells/mobile`, and a feature directory is private to itself. A stray
   static import silently returns the mobile stack to the embedded web build and
   only `npm --prefix frontend run check-boundaries` notices.
7. Update user-facing docs when setup, configuration, storage, or behavior
   changes. Release notes are pull request titles grouped by label: write the
   title as the user-visible effect and give the PR one category label
   (`docs/development/releasing.md`); a compatibility break also gets
   `breaking`.
8. Report checks that were not run or could not pass; do not imply verification.
9. Reply to the user in Traditional Chinese; keep commit messages, code
   comments, and repository docs in English.

## Required guidance

Read and follow these shared instructions before working in the repository:

- [Scope, judgment, and completion](docs/development/agent/judgment.md)
- [Rule maintenance](docs/development/agent/maintenance.md)
- [Verified project pitfalls](docs/development/agent/lessons.md), focusing on the
  sections relevant to the task

For the corresponding task, also read and follow the complete workflow:

- [Work a tracked ticket](docs/development/agent/workflows/work-ticket.md)
- [Update documentation](docs/development/agent/workflows/update-docs.md)

These linked files are instructions, not optional background. If your tool does
not expand links automatically, open them explicitly. Claude Code imports the
same rules through `CLAUDE.md` and `.claude/rules/`; its skill entry points live
in `.claude/skills/`. See [the loading map](docs/development/agent/index.md).
