# 50 — Verified project pitfalls

Read the relevant section before working in that area. Add entries according to
`40-maintenance.md`; keep them concise and grouped by root cause.

## Build and runtime

- **Embedded frontend:** Go builds require `frontend/dist`; rebuild the frontend
  before Go or end-to-end runs when it is missing or stale. A branch switch
  leaves the previous branch's bundle in place, so the suite quietly tests the
  wrong frontend against the new specs. (`frontend/web.go`)
- **Hidden build assets:** keep `//go:embed all:dist`; plain `dist/*` silently
  omits underscore-prefixed Rolldown chunks and produces a white-screen SPA.
  (`frontend/web.go`)
- **Stale running server:** rebuilding `frontend/dist` does not update an already
  running Go process because assets are embedded at compile time; restart it.
- **Restricted dependency installs:** if `sharp` cannot download libvips, use
  `npm --prefix frontend ci --ignore-scripts` for web-only work. Do not use that
  install for Capacitor asset generation.
- **Cold end-to-end startup:** the first `go run` may exceed the e2e server wait
  while downloading/building modules; build the frontend and run `go build ./...`
  once before diagnosing a timeout as a test failure.
- **Preinstalled golangci-lint is too old:** the container's binary refuses this
  repo with "Go language version used to build golangci-lint is lower than the
  targeted Go version", and `go install` reproduces it. Download the release
  build CI uses instead:
  `curl -sSL https://github.com/golangci/golangci-lint/releases/download/v2.12.2/golangci-lint-2.12.2-linux-amd64.tar.gz | tar xz`.
  CI enables `unused`, so a helper left without callers fails the build even
  when `go vet` and `go test` pass. (`.golangci.yml`, `.github/workflows/ci.yml`)
- **Server tests race the initial shelf scan:** a read issued before it finishes
  is answered 503 `ErrShelfInitializing`. Test envs must wait via
  `WaitReady`; do not rely on unrelated startup work to mask it.
  (`server/apitest_test.go`)

## Frontend and Reka UI

- **Shared component contracts:** before changing props or emits, search every
  consumer and verify each call site is intentionally wired or excluded.
- **Popover styling:** Reka popper content crosses a wrapper/portal boundary;
  follow the global `[data-reka-popper-content-wrapper]` pattern instead of
  relying on a component-scoped selector. (`frontend/src/styles.css`)
- **Primitive DOM defaults:** confirm the element rendered by Reka primitives;
  default `ul`/`ol` styles can consume layout space. Prefer an explicit `as` or
  reset list styles where the semantic element is not wanted.
- **Hidden tab/collapsible panels:** Reka keeps every panel's wrapper element in
  the DOM and only sets the `hidden` attribute; any author `display` rule on the
  panel class beats the user-agent `[hidden]` default, so inactive panels stay as
  layout boxes and their parent's grid/flex gap shifts the visible one. Pair such
  a rule with `.panel-class[hidden] { display: none; }`.
  (`frontend/src/features/settings/pages/SettingsPage.vue`)
- **Tab content is unmounted, the wrapper is not:** the persistent wrapper above
  hides that `TabsRoot` defaults `unmountOnHide` to true, so an inactive tab's
  *slot content* is destroyed and rebuilt on every visit. A tab body that owns
  state — an in-flight request, a fetched list, a cache — silently resets and
  refetches. Set `:unmount-on-hide="false"` when moving stateful markup into a
  per-tab component, and keep the `[hidden]` rule above, which is then what stops
  the retained content from rendering in full.
  (`reka-ui/src/Tabs/TabsRoot.vue`, `reka-ui/src/Tabs/TabsContent.vue`)
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
  `EditorState.lineSeparator` and the text is read with
  `doc.sliceString(0, len, separator)`. A break is one *position* whatever it was
  written with, so a string offset is not a document offset — build the `Text`
  before deriving any position from it.
- **Editor/page offset boundary:** the chapter dimming, the outline's jumps and
  the rename/merge edits all drifted on CRLF sources by one character per
  preceding line break → the page indexes `content` as a plain string
  (`markdownChapters.ts`, shared with the reader's persisted progress) while
  CodeMirror counts positions. The adapter contract is stated in source-text
  offsets and `useSourceCodeMirror` converts at its own edge; never hand a
  `markdownChapters` offset straight to a CodeMirror API, and cover a change
  here with a CRLF case — an LF-only test cannot see the difference.
  (`frontend/src/features/sources/types/editorAdapter.ts`)
- **Dimming under highlighting:** a `Decoration.mark` that dims by `color` is
  overridden wherever syntax highlighting paints a nested span. Dim with
  `opacity`, which applies to the whole subtree.
- **Caret on load:** a textarea parks its caret at the end once its value is
  assigned; a fresh `EditorState` starts at zero. Anything that acts on "where
  the caret is" — the chapter outline's Add button — silently changes target.

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
  browser revision than the preinstalled one → do not run `playwright install`;
  run with a throwaway local config that sets
  `launchOptions.executablePath: '/opt/pw-browsers/chromium'`.
  (`e2e/playwright.config.ts`)
- **Editing the source editor:** it is not a form control, so `fill`,
  `inputValue` and `selectionStart` do not apply, and only the lines near the
  viewport exist in the DOM. Drive it through `e2e/tests/support/sourceEditor.ts`,
  which reaches CodeMirror's own view the way `EditorView.findFromDOM` does.
- **Platform-dependent editor keys:** `Control+z` is not undo on macOS, and
  `End` runs `cursorLineBoundaryForward`, which stops at the end of the *visual*
  row — the editor pane is a few hundred pixels wide with wrapping on, so
  whether a line fits in one row comes down to font metrics and differs from CI.
  Both passed on Linux and failed on a Mac. Use `ControlOrMeta` for CodeMirror's
  `Mod-` bindings, and place the caret with `setEditorCaret` rather than walking
  it with arrow keys. (`e2e/tests/source-editor.spec.ts`)
- **Teardown ENOTEMPTY:** whole-suite runs fail a handful of unrelated specs with
  `ENOTEMPTY … rmdir '<tmp>/shelf/app'` → the temp shelf is deleted while the
  just-signalled server still writes into it, so the failure is teardown-only and
  lands on different specs each run; re-run the spec alone before charging it to
  the diff. (`e2e/tests/support/server.ts`)

## Filesystem and API

- **Mutating API requests:** preserve the `local_token` boundary and review the
  matching `server/contract/api_*_contract_test.go` whenever routes or request
  handling change.
- **Book identity:** moving or renaming a book must not regenerate its persisted
  ID. The directory name and display title are not identity.
- **Network shelves:** SMB latency amplifies directory walks and stat calls;
  preserve scan/check intervals, finite lock timeouts, atomic writes, and clear
  error propagation. See `docs/concepts/shelf-cache-and-io.md`.
