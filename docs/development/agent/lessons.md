# 50 — Verified project pitfalls

Read the relevant section before working in that area. Add entries according to
[the maintenance policy](maintenance.md); keep them concise and grouped by root cause.

## Build and runtime

- **Embedded frontend:** Go builds require `frontend/dist`; rebuild the frontend
  before Go or end-to-end runs when it is missing or stale. A branch switch
  leaves the previous branch's bundle in place, so the suite quietly tests the
  wrong frontend against the new specs, and a running Go process keeps the
  assets it was compiled with until restarted. Keep `//go:embed all:dist`; plain
  `dist/*` silently omits underscore-prefixed Rolldown chunks and produces a
  white-screen SPA. (`frontend/web.go`)
- **Restricted dependency installs:** if `sharp` cannot download libvips, use
  `npm --prefix frontend ci --ignore-scripts` for web-only work. Do not use that
  install for Capacitor asset generation.
- **Cold end-to-end startup:** the first `go run` may exceed the e2e server wait
  while downloading/building modules; build the frontend and run `go build ./...`
  once before diagnosing a timeout as a test failure.
- **Preinstalled golangci-lint is too old:** the container's binary refuses this
  repo with "Go language version used to build golangci-lint is lower than the
  targeted Go version", and `go install` reproduces it. Download a release build
  whose own Go *language* version is at least `go.mod`'s target instead; a patch
  difference does not count. A patch bump can still break an older build in a
  different way: on Go 1.27.2 (2026-10) v2.13.1 fails every package with
  `typecheck` "export data version 5 is greater than maximum supported version
  4"; v2.14.0 works. Match the version to `go.mod`:
  `curl -sSL https://github.com/golangci/golangci-lint/releases/download/v2.14.0/golangci-lint-2.14.0-linux-amd64.tar.gz | tar xz`.
  CI enables `unused`, so a helper left without callers fails the build even
  when `go vet` and `go test` pass. `govulncheck` has the same trap: `go
  install …@latest` builds with the older *base* `go` and dies with `requires
  newer Go version go1.27 (application built with go1.26)`, which reads like a
  repo break; prefix the install with `GOTOOLCHAIN=go<go.mod's version>`. A full
  local scan is still impossible: the network policy 403s `vuln.go.dev`.
  (`.golangci.yml`, `.github/workflows/ci.yml`)
- **Frontend suite needs a supported Node:** ten unrelated suites fail at import
  with `TypeError: Cannot read properties of undefined (reading 'getItem')`
  pointing at `src/api/client.ts`. From Node 26 the runtime defines its own
  experimental `localStorage` global, which leaves `window.localStorage`
  `undefined` inside Vitest's jsdom environment while `'localStorage' in window`
  stays true. Guarding one access does not fix it: other suites store and read
  values for real. Run the suite on a Node version `docs/development/setup.md`
  names as supported. (`docs/development/setup.md`)
- **Test support in a package is not a `_test.go` file:** moving shared test
  helpers into an importable package (`server/contract/apitest`) takes them out
  of every rule that keys on the `_test.go` suffix. `internal/repocheck`'s CJK
  check then reads a CJK fixture string as shipped text, and staticcheck's
  `unused` stops reporting the now-exported helpers. Allowlist the fixture file
  with a reason and keep a caller check for the exported helpers; that check
  counts functions only, not types, constants or methods.
  (`internal/repocheck/cjk_test.go`, `internal/repocheck/contract_test.go`)
- **Server tests race the initial shelf scan:** a read issued before it finishes
  is answered 503 `ErrShelfInitializing`. Test envs must wait via
  `WaitReady`; do not rely on unrelated startup work to mask it.
  (`server/contract/apitest/env.go`)

## Frontend and Reka UI

- **Popover styling:** Reka popper content crosses a wrapper/portal boundary;
  follow the global `[data-reka-popper-content-wrapper]` pattern instead of
  relying on a component-scoped selector. (`frontend/src/styles.css`)
- **Primitive DOM defaults:** confirm the element rendered by Reka primitives;
  default `ul`/`ol` styles can consume layout space. Prefer an explicit `as` or
  reset list styles where the semantic element is not wanted.
- **Hidden tab/collapsible panels:** Reka keeps every panel's wrapper element in
  the DOM and only sets the `hidden` attribute; any author `display` rule on the
  panel class beats the user-agent `[hidden]` default, so inactive panels stay
  as layout boxes and their parent's grid/flex gap shifts the visible one. Pair
  such a rule with `.panel-class[hidden] { display: none; }`. The persistent
  wrapper also hides that `TabsRoot` defaults `unmountOnHide` to true, so an
  inactive tab's *slot content* is destroyed and rebuilt on every visit. A tab
  body that owns state, such as an in-flight request, a fetched list, or a
  cache, silently resets and refetches. Set `:unmount-on-hide="false"` when
  moving stateful markup into a per-tab component, and keep the `[hidden]` rule
  above, which is then what stops the retained content from rendering in full.
  (`frontend/src/features/settings/pages/SettingsPage.vue`,
  `reka-ui/src/Tabs/TabsRoot.vue`)
- **Scoped styles and fragments:** styles targeting nodes rendered through a
  fragment may need a stable parent plus `:deep(...)`; verify selected and
  unselected computed styles, not only `data-state`.
- **Splitter scrolling:** `SplitterPanel` owns `overflow: hidden`; put scrollable
  content in an inner full-height wrapper.
- **Stable object props:** hoist object/array props passed to Reka controls such
  as `hitAreaMargins`; new identities during a drag can re-register controls and
  leave pointer events disabled. (`frontend/src/layouts/MainLayout.vue`)
- **Desktop zoom:** viewport-filling `vh`/`vw` values inside the zoomed app must
  account for `--app-zoom`; verify overflow and popper placement in
  `?desktop-shell-preview=1`. (`frontend/src/styles.css`)
- **Browser APIs:** TypeScript success does not prove IndexedDB/runtime behavior;
  run browser-backed tests for browser-only APIs. Key cursors cannot delete
  records; use a value cursor when deletion is required.
- **Mock cover noise:** blocked `picsum.photos` requests in mock mode can be
  environmental noise; compare against an unchanged baseline before assigning
  them to the current diff.

## CodeMirror source editor

- **Theme selectors:** `&light` / `&dark` prefixes belong to
  `EditorView.baseTheme`; `EditorView.theme` throws `Unsupported selector` on
  them. Style a theme unprefixed and declare `{ dark }` in its options.
  (`frontend/src/features/sources/composables/useSourceCodeMirror.ts`)
- **Search without the panel:** `@codemirror/search` highlights matches only
  while its own panel is open, and its commands *open that panel* when the query
  is missing or invalid. A custom find bar therefore needs its own highlight
  `ViewPlugin` and must check `SearchQuery.valid` before dispatching.
- **Line endings:** a document is split on any line break and rendered back with
  `\n`, so a CRLF source is rewritten end to end unless the state carries
  `EditorState.lineSeparator` and the text is read with `doc.sliceString(0, len,
  separator)`. A break is one *position* whatever it was written with, so a
  string offset is not a document offset. Build the `Text` before deriving any
  position from it. The chapter dimming, the outline's jumps and the
  rename/merge edits all drifted on CRLF sources by one character per preceding
  line break, because the page indexes `content` as a plain string
  (`markdownChapters.ts`, shared with the reader's persisted progress) while
  CodeMirror counts positions. The adapter contract is stated in source-text
  offsets and `useSourceCodeMirror` converts at its own edge; never hand a
  `markdownChapters` offset straight to a CodeMirror API, and cover a change
  here with a CRLF case, because an LF-only test cannot see the difference.
  (`frontend/src/features/sources/types/editorAdapter.ts`)
- **Dimming under highlighting:** a `Decoration.mark` that dims by `color` is
  overridden wherever syntax highlighting paints a nested span. Dim with
  `opacity`, which applies to the whole subtree.
- **Caret on load:** a textarea parks its caret at the end once its value is
  assigned; a fresh `EditorState` starts at zero. Anything that acts on the
  caret, such as the chapter outline's Add button, silently changes target.

## Mobile and end-to-end tests

- **Preview routing:** top-level mobile preview navigations must preserve
  `?mobile-shell-preview=1`; use `reopenMobileAt` at layout/reload boundaries.
  (`e2e/tests/support/mobile.ts`)
- **Offline simulation:** use the support helper that blocks API routes while
  retaining the embedded shell; browser context offline mode prevents the page
  itself from reopening. (`e2e/tests/support/mobile.ts`)
- **Mobile covers:** WebView `<img src="http://…">` requests do not use
  Capacitor's native HTTP bridge and may hit mixed-content blocking; use the
  fetch-to-blob cover path. (`frontend/src/composables/useCoverSrc.ts`)
- **Android networking:** emulator host loopback is `10.0.2.2`; physical devices
  need a LAN-reachable server. Validate the merged manifest rather than only the
  source manifest when checking generated cleartext settings.
- **Android WebView automation:** Playwright browser-context APIs are not fully
  supported by WebView DevTools; use raw CDP for native WebView-only checks.
- **Large-data UI races:** when fixed mock data cannot express the required
  pagination state, intercept the Vite module or API in the test instead of
  changing production fixtures globally.
- **Cloud e2e browser revision:** the pinned Playwright may expect a newer
  browser revision than the preinstalled one. Do not run `playwright install`;
  run with a throwaway local config that sets `launchOptions.executablePath:
  '/opt/pw-browsers/chromium'`. Put that config *inside* `e2e/`: Playwright
  resolves `globalSetup` and reporter paths relative to the config file, so a
  copy in a scratch directory dies with a bare `MODULE_NOT_FOUND` naming
  Playwright's own internals rather than the path. Only chromium is installable:
  the egress policy 403s `cdn.playwright.dev` and
  `playwright.download.prss.microsoft.com`, so `npx playwright install webkit`
  fails locally and the nightly matrix (`E2E_BROWSERS`) runs only on a runner.
  Dispatch `nightly.yml` on the branch: a `workflow_dispatch` checks out the ref
  it was dispatched from, and only the scheduled run files the failure issue.
  (`e2e/playwright.config.ts`, `.github/workflows/nightly.yml`)
- **WebKit has no constructible `Touch`:** `new Touch({…})` throws "Illegal
  constructor" there, so a spec that synthesises touch events passes on chromium
  and dies on webkit. Use `document.createTouch`/`createTouchList` (page
  coordinates, a real `TouchList`) behind a presence check, the same branch
  Playwright makes inside its own `dispatchEvent`. Keep the whole gesture in one
  `evaluate`: a tap needs touchstart and touchend within
  `MOBILE_READER_TAP_DURATION_MS`. (`e2e/tests/mobile-reader.spec.ts`)
- **Editing the source editor:** it is not a form control, so `fill`,
  `inputValue` and `selectionStart` do not apply, and only the lines near the
  viewport exist in the DOM. Drive it through `e2e/tests/support/sourceEditor.ts`,
  which reaches CodeMirror's own view the way `EditorView.findFromDOM` does.
- **Platform-dependent editor keys:** `Control+z` is not undo on macOS, and
  `End` runs `cursorLineBoundaryForward`, which stops at the end of the *visual*
  row. The editor pane is a few hundred pixels wide with wrapping on, so whether
  a line fits in one row comes down to font metrics and differs from CI. Both
  passed on Linux and failed on a Mac. Use `ControlOrMeta` for CodeMirror's
  `Mod-` bindings, and place the caret with `setEditorCaret` rather than walking
  it with arrow keys. (`e2e/tests/source-editor.spec.ts`)
- **Teardown ENOTEMPTY:** whole-suite runs fail a handful of unrelated specs
  with `ENOTEMPTY … rmdir '<tmp>/shelf/app'`. The temp shelf is deleted while
  the just-signalled server still writes into it, so the failure is
  teardown-only and lands on different specs each run; re-run the spec alone
  before charging it to the diff. `dispose()` now retries the removal five times
  with a backoff, so a surviving ENOTEMPTY means a leaked server process still
  held the directory after half a second, not this race.
  (`e2e/tests/support/server.ts`)
- **A green pull request is not a green E2E suite:** `ci.yml` runs `--grep
  @smoke` (13 of 37 cases) and `nightly.yml` runs the rest on `dev`. A change
  outside the smoke set is unproven until `just test-e2e` runs locally or the
  night after it merges reports; a nightly failure opens one issue rather than
  landing on whoever pushes next. The whole suite took 1m21s wall clock in the
  cloud container (2 workers, measured 2026-09-04), plus about 30s of install
  and server build, so run it rather than waiting on the nightly.
  (`docs/development/testing-levels.md`)
- **E2E server ports are per worker, not per kernel:** `getFreePort()` takes a
  port from a band derived from `TEST_PARALLEL_INDEX` instead of asking for
  port 0. Asking for 0 lets two parallel workers be handed the same number, and
  the loser then finds a *healthy* `/health` on it (the other worker's server,
  over the other worker's shelf), so it attaches and both specs mutate one
  shelf. Verified by pinning one start onto a port a second `plainshelf-srv`
  already held. (`e2e/tests/support/server.ts`)

## Filesystem and API

- **JSON encoding:** marshal through `internal/jsonopt`, not bare
  `encoding/json/v2` options. v2 leaves map order unspecified, which defeats the
  three "unchanged, do not rewrite" checks (`fingerprint/cache.go`'s byte
  compare, `bookCacheDigest`, `scanCacheDigest`) with no compile error and no
  test failure, costing a re-upload per scan on pCloud or SMB.
  (`internal/jsonopt/jsonopt.go`)
- **No v1 read compatibility:** the shelf does not promise that a file an older
  build wrote still reads the same, so do not reach for the v1-compat decode
  options. json/v2's strict defaults are the decision. A hand-edited `"Title"`
  now reads as absent, and `setMeta` rewrites `book.json` whole, so the next
  save drops it; that is accepted, and a planned unknown-member passthrough is
  what changes it. Proposing a compatibility layer here has already been
  rejected once. (`docs/concepts/data-format-versioning.md`)
- **Network shelves:** SMB latency amplifies directory walks and stat calls;
  preserve scan/check intervals, finite lock timeouts, atomic writes, and clear
  error propagation. See `docs/concepts/shelf-cache-and-io.md`.

## Documentation

- **Section inserted by text replacement:** replacing an existing heading with
  "new section + that heading" has dropped the heading twice. Only
  `configuration.md` has a test that notices (`server/conf_docs_test.go`), and
  no PR gate builds the docs (`docs.yml` runs at release). Run `mkdocs build
  --strict`, which fails on a broken in-page anchor since `validation.anchors:
  warn`, and read every removed line with `git diff origin/dev -- docs | grep
  '^-[^-]'`. (`mkdocs.yml`)
