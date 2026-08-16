# PlainShelf — Claude project guide

PlainShelf is a pre-alpha, local-first reading library. The Go server embeds a
Vue frontend and is also used by Wails desktop and experimental Capacitor
Android clients.

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
| `shelf/` | filesystem-backed library core |
| `server/` | HTTP API, security, and runtime |
| `frontend/` | Vue UI and Capacitor Android project |
| `desktop/` | Wails desktop client |
| `e2e/` | Playwright end-to-end tests |
| `docs/` | user and contributor documentation |

## Commands

The Go build embeds `frontend/dist`; build the frontend before Go builds or
tests when that directory is absent or stale.

| Scope | Command |
|---|---|
| Install frontend deps | `npm --prefix frontend ci` |
| Frontend unit tests | `npm --prefix frontend test` |
| Frontend type-check + build | `npm --prefix frontend run build` |
| Main Go module | `go test ./...` |
| Desktop Go module | `cd desktop && go test ./...` |
| Go lint (both modules) | `golangci-lint run` and `cd desktop && golangci-lint run` |
| All Go tests with frontend build | `just test-go` |
| End-to-end tests | `just test-e2e` |
| Mock frontend | `VITE_USE_MOCK_API=true npm --prefix frontend run dev` |

`just` uses `zsh`. If either is unavailable, run the underlying commands from
the `justfile`. In restricted environments where `sharp` cannot download its
optional binary, `npm --prefix frontend ci --ignore-scripts` is sufficient for
web builds; do not use it for Capacitor asset generation.

## Working rules

1. Inspect the relevant code, tests, and nearby documentation before editing.
2. Preserve unrelated working-tree changes and avoid broad refactors unless
   they are part of the request.
3. Add or update tests for behavior changes. Run the narrowest relevant check
   while iterating and the full affected-area check before completion.
4. For server API changes, read the matching
   `server/contract/api_*_contract_test.go` and preserve the `local_token`
   security boundary.
5. Update user-facing docs when setup, configuration, storage, or behavior
   changes. Update `CHANGELOG.md` only when the task calls for release notes.
6. Report checks that were not run or could not pass; do not imply verification.

## Rule routing

Read only the rule needed for the task:

- Collaboration and optional delegation: `.claude/rules/10-delegation.md`
- Scope, escalation, and completion: `.claude/rules/20-judgment.md`
- Reusable task briefs: `.claude/rules/30-prompt-templates.md`
- Maintaining these rules: `.claude/rules/40-maintenance.md`
- Verified project-specific pitfalls: `.claude/rules/50-lessons.md`
- Historical context only: `.claude/rules/00-diagnosis.md` and
  `.claude/rules/90-letter.md`
