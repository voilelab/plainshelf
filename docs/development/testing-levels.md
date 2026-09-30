# Testing Levels

`CLAUDE.md` asks for a test with every behavior change. It does not say where
that test goes, and the level is the expensive half of the decision: the
cheapest place to *write* a test is rarely the cheapest place to *keep* one.
Writing a UI assertion as an end-to-end case takes ten minutes and costs the
project a browser, a server binary and a real shelf on every pull request,
forever. Nobody makes that trade deliberately; it is what happens when there is
no page like this one.

This page is that page. It defines five levels, says what each one may touch,
and gives seven rules and four budgets that decide between them. It should
answer "which level?" in about thirty seconds. Where it does not, the honest
answer is usually [R2](#rules).

## Why this page exists

The suite this page was written against:

| Level | Files | Cases | Lines |
|---|---:|---:|---:|
| L1 + L2 (Go, all three modules) | 131 | 694 | 28,184 |
| L3 (frontend unit and component) | 155 | 1,521 | 25,450 |
| L4 (end-to-end) | 23 | 37 | 2,122 |
| **Total** | **309** | **2,252** | **55,756** |

Go counted 2026-09-04, the other two 2026-09-03. "Cases" is top-level
`func Test…` for Go and the case count each runner reports for the other two,
so the figures are reproducible rather than estimated. The Go row counts the
ten files of `server/contract/apitest` even though they are not `_test.go`
files: the harness is L2 code that happens to be a package.

The L4 row is the first one in this repository's history to go down: it was 30
files / 101 cases / 4,247 lines when this page was written, and PSW-78 moved the
assertions that never needed a browser down to L3 — which is why the L3 row grew
by 75 cases in the same change. It went down twice more when the two
`security-headers` cases that only issued `fetch` calls were deleted as
duplicates of `server/contract/crosscut/security_headers_test.go`, which pins
the same headers, directives and nonce at L2 over more response shapes. The
L1 + L2 row drifted upward on its own; that is ordinary growth, not this page's
doing.
Every number above is a measurement, so a later reader can tell growth from
drift.

## The five levels

| | Level | May touch | Lives in | Runner |
|---|---|---|---|---|
| **L1** | Go unit | Pure Go, `t.TempDir()`, in-process fakes | Beside the code: `shelf/`, `internal/`, `server/`, `desktop/`, `reader/` | `go test ./...` |
| **L2** | Go API contract | The real router over `httptest`, a real shelf in a temp dir, the `local_token` gate | `server/contract/<area>/*_test.go`, harness in `server/contract/apitest` | `go test ./...` |
| **L3** | Frontend unit and component | jsdom, components mounted in process, a mocked API client | `frontend/src/**/*.test.ts` next to the module, plus `frontend/scripts/*.test.mjs` for the gate scripts | `npm --prefix frontend test` |
| **L4** | End-to-end | Real Chromium, a real `plainshelf-srv`, a real shelf on disk, the embedded bundle | `e2e/tests/*.spec.ts`, helpers in `e2e/tests/support/` | `just test-e2e`, or `just test-e2e-smoke` for the pull request's subset |
| **L5** | Manual on device | Anything CI cannot reach: a phone, a desktop window, an SMB share | No code. Steps in `docs/development/`, evidence in the pull request | A person |

### L1 — Go unit

Everything that is a function of its inputs, plus everything that is a function
of a directory you can create with `t.TempDir()`. No listener, no network, no
browser. This is where shelf layout, ID stability, parsing, hashing and task
scheduling belong, and it is the level almost every Go change should stop at.

### L2 — Go API contract

The HTTP surface: status codes, request and response shapes, JSON field names,
method gates, and the `local_token` boundary. It runs the real router, so it
catches a route that was registered wrongly, which L1 cannot. These tests are
the specification, so per `CLAUDE.md` rule 4 you read the matching one before
changing a route.

They live in one package per area of the API, so the directory says where to
look and the compiler says what is shared:

| Package | Covers |
|---|---|
| `server/contract/apitest` | the harness itself: `Env`, the assertions, and in `url.go` every route `server/routes.go` registers |
| `server/contract/books` | books, batches, copies, transfers, import, covers, content stats |
| `server/contract/sources` | sources, assets, content streaming, fingerprints, similarity |
| `server/contract/folders` | folders, folder transfers, trash |
| `server/contract/shelves` | shelves, scans, the book cache, `/api/mode` and `/api/version` |
| `server/contract/platform` | task chains, logs, settings |
| `server/contract/crosscut` | what every route owes whatever it does: security, headers, method gates, read-only, request IDs, error envelopes, shelf resolution, schema versions |

A helper is either exported from `apitest` or private to the one package that
uses it — there is no third option, which is what the split bought. When a new
helper is needed by two areas, it belongs in `apitest`, not in whichever
package happened to need it first.

L2 is not a slower L1. If a case does not depend on the request or the response,
it belongs one level down.

### L3 — Frontend unit and component

Composables, stores, utilities, and components mounted in jsdom against a mocked
API client. No real server and no real browser: a test that needs either is not
an L3 test, it is an L4 test that has not admitted it yet.

`npm --prefix frontend run build` is `vue-tsc` plus Vite, not this suite. Type
checking clean says nothing about whether these tests ran.

### L4 — End-to-end

The only level with a browser, a server and a shelf at the same time — so it
should hold only behavior that needs all three at once. That is a short list:
navigation across pages, the embedded bundle actually being served, the desktop
and mobile shell previews, and browser-only APIs such as IndexedDB whose real
behavior jsdom does not reproduce.

It is also the only level whose failures are routinely ambiguous. Its known
traps — preview routing, offline simulation, teardown `ENOTEMPTY`, the pinned
browser revision — are listed under "Mobile and end-to-end tests" in
`.claude/rules/50-lessons.md`. Read them before charging a red spec to your
diff.

#### L4 runs in two tiers

A pull request does not run the whole suite. It runs the cases tagged
`@smoke`, and the rest run once a night on `dev`:

| Tier | What runs | Browsers | Where | What it gates |
|---|---|---|---|---|
| Smoke | `@smoke`, 13 cases | chromium | `ci.yml`, every pull request | the merge |
| Full | all 37 cases | chromium, webkit | `nightly.yml`, 03:00 UTC on `dev` | the release |

The split exists because the wait before a merge should not grow with the
suite. It costs one thing, and this is it: **a green pull request no longer
means the E2E suite passed.** A change outside the smoke set is checked by
`just test-e2e` before pushing, and by the night after merging; when the night
is red, the workflow opens (or comments on) one issue on this repository.

#### The browser matrix is the nightly's, not the gate's

Chromium is the engine Windows (WebView2) and Android (Chromium WebView) ship,
so it is the one the merge gate runs. It is not the engine the macOS cask
ships: Wails renders there in **WKWebView**, and until PSW-111 no E2E case had
ever run against a WebKit build — `desktop-shell.spec.ts` included. The nightly
therefore runs a second round on Playwright's `webkit`.

`E2E_BROWSERS` picks the round, as a comma-separated list; it defaults to
`chromium`, so a local `just test-e2e` and the gate are unaffected and need no
extra browser download. `nightly.yml` sets `chromium,webkit` and installs both.

Two limits, easily conflated:

- Playwright's `webkit` is neither Safari nor WKWebView — it is the same WebKit
  core in a different embedder. A green webkit round means no chromium-only
  assumption was found, **not** that the macOS desktop app is covered. Only a
  real macOS runner would say that, and that is out of scope here.
- `firefox` is available in `E2E_BROWSERS` but is not run anywhere. Gecko is not
  an engine this project ships against, so a third round would spend nightly
  time on a target with no users. Run it by hand if a rendering question makes
  it worth an answer.

The smoke tier is a budget of its own — ≤ 20 cases, 13 today — and its job is
to notice that the product is broken, not to cover behavior. It holds one case
per critical journey:

| Journey | Cases |
|---|---|
| Import and render | `import-book` (txt, markdown, HTML sanitisation), `import-epub` (TOC) |
| Reading and progress | `read-history` (record and clear) |
| Browsing | `library-search` (filter and restore) |
| Editing a source | `source-editor` (edit and see it in the reader, create and switch) |
| Security | `security-headers` (the app boots with no CSP violation) |
| Mobile read-only | `mobile-read-only` (routing, reachability, hidden writes, rejected write) |

Tag with Playwright's own option — `test('…', { tag: '@smoke' }, async …)` —
so `--grep @smoke` finds it. Adding a case to the smoke tier is not free: it is
on the critical path of every merge, so it needs the same net-zero argument R4
asks for, against the 20-case limit rather than the 40-case one.

The suite runs `fullyParallel`, at two workers on CI and half the local cores
otherwise. That is safe because a server is per spec file rather than per
suite: `startServer()` in `e2e/tests/support/server.ts` mints its own temp
shelf, its own store and a port from this worker's own band on every call.

### L5 — Manual on device

Some behavior has no runner. Recording it as L5 is the answer to it, not an
admission that a test is missing:

- Android WebView behavior, which Playwright's browser-context APIs do not fully
  drive; native-only checks need raw CDP.
- Device and emulator networking — host loopback is `10.0.2.2` on the
  emulator, and a physical device needs a LAN-reachable server.
- Wails desktop windowing, which does not run in a headless container at all.
- SMB shelf behavior, where latency is the thing under test.
- Cover loading through Capacitor's native HTTP bridge on a real device.

L5 work is verified in the pull request's **Manual verification** section:
environment, steps, result, screenshot. A claim without those four is not an L5
verification, it is a guess.

#### Layout checks with no runner

One more category landed here with PSW-78, and it is worth naming because it is
the least comfortable part of that change. A handful of E2E cases asserted
things only a layout engine can answer — that a control clears the fold, that a
row does not overflow its container, that a form taller than the screen still
scrolls to its Save button. jsdom resolves no layout and no `var()`, so those
cannot go to L3; and with the total at 39 of a 40-case budget there was no E2E
slot to keep them in either (the budget has since freed three). They are
checked by eye when the surface around them changes:

| Surface | What to look at | Was |
|---|---|---|
| Book detail, 1280×720 and 390×844 | Title, reading progress and the primary action clear the fold, in both locales | `book-detail.spec.ts` |
| Dashboard with one recent book | The "recent reading" cover stays capped rather than stretching the row | `read-history.spec.ts` |
| Library toolbar at 1600/1280/1024/800 | The last control ("Update book list") stays inside the header | `library-rescan.spec.ts` |
| Mobile shelf editor at 360×480 | The page itself scrolls, so "Save and continue" is reachable | `mobile-shelves.spec.ts` |

Two of these were regressions once, so this is a real cost, not a tidy-up. If
one recurs, the answer is an E2E case *and* a deletion to pay for it (R4) —
not a quiet raise of the 40.

## Decision tree

```text
Can the behavior fail without a browser and without an HTTP server?
├─ yes → Is it Go?
│         ├─ yes → L1
│         └─ no  → L3
└─ no  → Can it fail without a browser?
          │      (status code, JSON shape, route gate, local_token)
          ├─ yes → L2
          └─ no  → Can a component mounted in jsdom reproduce it?
                    ├─ yes → L3
                    └─ no  → Can CI's Chromium plus server reach it?
                              ├─ no  → L5
                              └─ yes → Is it worth an E2E slot? (R4, budgets)
                                        ├─ yes → L4, and delete one
                                        └─ no  → no automated test (R2)
```

## Rules

**R1 — Lowest level first.** Write the test at the lowest level that can fail
for the reason you care about. "It is more realistic higher up" is always true
and is not a reason; realism you cannot afford to run is not realism.

**R2 — "No automated test" is a legal answer.** Say so in the pull request and
say why: the behavior is cosmetic, or it is L5, or the cost of pinning it
exceeds what it protects. An honest refusal reviewed by a person beats a test
nobody trusts.

**R3 — A bug fix adds one regression test.** Exactly one, at the lowest level
that reproduces the bug. A bug is evidence about one behavior, not a licence to
cover its neighborhood.

**R4 — A new E2E case is net-zero.** Adding one means deleting or merging
another in the same pull request. The E2E suite is a fixed-size budget, not a
growing one; see the budgets below.

**R5 — Deleting a test costs one sentence.** Name what it covered and where
that coverage now lives (another level, the type system, or nowhere and why
that is acceptable). No ceremony, no approval round. Deletion has to be
cheaper than accumulation or accumulation always wins.

**R6 — A flaky test is fixed within two weeks or deleted.** Not skipped, not
retried, not quarantined indefinitely — a test nobody believes is worse than
no test, because it teaches people to ignore red. Tracking is manual today.

**R7 — Automate a manual step after it appears in three consecutive pull
requests.** Before that, repeating it by hand is cheaper than owning an
automation for it. A step done once is not a process.

## Budgets

| Budget | Limit | Today |
|---|---|---|
| PR gate wall clock (`ci.yml`) | ≤ 10 minutes | 5:28 median, 7:52 worst (2026-09-03) |
| E2E cases in total | ≤ 40 | 37 |
| E2E cases in the PR gate (`@smoke`) | ≤ 20 | 13 |
| E2E cases in one spec | ≤ 5 | 4 (`folder-tree`, `mobile-read-only`) |

**When a budget is exceeded, the answer is to delete tests, not to raise the
budget.** A budget that moves whenever it binds is a description, not a limit.

All four rows are green as of PSW-78, which brought the suite from 101 cases in
30 files to 39 in 23 by moving down everything a stubbed server could not have
broken; deleting the two L2-duplicate `security-headers` cases took it to 37.
Green is the normal state, not slack to spend: three cases of headroom is one
change's worth, so R4 still holds — a new E2E case means deleting or merging
another in the same pull request.

The rows are green for different reasons, and only one of them is a reason to
relax. The smoke row is a limit that was chosen and is enforced on every pull
request. The wall-clock row is cheap because the expensive level was moved off
the gate first (PSW-77) and then shrunk (PSW-78); "about two times' headroom",
which this page claimed before either change, was optimistic, and the table
below is where the number comes from now.

## The time budget per job

Measured 2026-09-03 over the three most recent `dev` runs of `ci.yml`:
[#1382](https://github.com/voilelab/plainshelf/actions/runs/33656639070),
[#1383](https://github.com/voilelab/plainshelf/actions/runs/33656678606) and
[#1385](https://github.com/voilelab/plainshelf/actions/runs/33699956726). All
three were green. "Wall clock" is the job's own start to its own finish, so the
three columns do not add up to the run: every job but `frontend` runs in
parallel behind it.

| Job | #1382 | #1383 | #1385 | Median | `timeout-minutes` |
|---|---:|---:|---:|---:|---:|
| Frontend build | 1:30 | 1:19 | 1:23 | **1:23** | 5 |
| Go lint | 0:30 | 0:26 | 0:49 | **0:30** | 5 |
| Go tests | 0:28 | 0:36 | 0:35 | **0:35** | 5 |
| Frontend E2E | 3:34 | 6:27 | 3:22 | **3:34** | 10 † |
| Android build | 1:29 | 1:24 | 1:05 | **1:24** | 5 |
| Go vulnerability scan | 0:43 | 0:42 | 0:45 | **0:43** | 5 |
| npm audit | 0:07 | 0:07 | 0:10 | **0:07** | 5 |
| **Whole run** | 5:09 | 7:52 | 5:28 | **5:28** | — |

† The E2E row measures the job as it was: 101 cases, one worker. PSW-77 made
it 15 cases at two workers, which ran in 20s on a 4-core container against
2:47–2:55 for the whole suite on CI, so both the measurement and the cap above
it are stale in the same direction. The cap is provisionally 10 — enough for a
3:05 browser-cache miss plus setup — and has to be re-measured over three `dev`
runs and tightened, along with this row. `nightly.yml` now carries the other 24
cases rather than 86; it has no measured baseline either and starts at 25.

One caveat before reading the two kinds of row together: `timeout-minutes`
counts a job's *execution*, while the whole-run row also counts its wait for a
runner. The pull request that added these caps ran for 10:00 — the budget's
whole limit — with its `frontend` job executing for 1:23, five minutes of the
rest being queue. A run near the budget therefore says nothing about how close
a job is to its cap.

`frontend` is the `needs` of every other job, so its time is on the critical
path twice over — once as itself, once as the delay before anything else starts.
The cumulative time from the run starting to the E2E job starting was 1:35 /
1:25 / 2:06; to the E2E *tests* starting, 2:20 / 4:54 / 2:43.

The gap between those two rows is setup, not testing, and it is worth keeping
separate: installing the Playwright browser took 23s, **3:05** and 21s across
the same three runs. The 3:05 is a browser-cache miss, and it is the whole
difference between #1383's 7:52 and the other two. The E2E test step itself was
steady at 2:47 / 2:55 / 2:42 — so an improvement there (PSW-77) would be
invisible in the job total if the install were folded into it.

### How the caps were chosen

`timeout-minutes` is **twice the slowest of the three runs, rounded up to a
whole minute, and never under 5**.

This is not the 1.5×-median rule PSW-76 proposed, and the measurement is why:
1.5 × the E2E median is 5:21, which #1383 would have failed on a browser-cache
miss alone. A cap that turns a cold cache into a red build teaches people to
re-run CI, which is the opposite of what a cap is for. The floor of 5 minutes
does the same job for the short jobs, where `npm ci` on a cold cache is a larger
share of the total than anything the tests do.

Every value is still 25× to 70× tighter than GitHub's 360-minute default, and
tight enough to fail on a real regression: the frontend job has to grow 3.3×
before it hits 5 minutes, E2E 2.0× before it hits 13.

**A job that hits its cap is a regression to explain, not a number to raise.**
Re-measure, update this table with the date, and change the cap only when the
new time is one the project has decided to accept.

### Where the time goes

Both test jobs print their ten slowest cases at the end of the log and into the
GitHub job summary — no artifact to download:

- `frontend/scripts/slowest-tests-reporter.mjs`, wired up in
  `frontend/vite.config.ts` (the Vitest config lives there; there is no
  `vitest.config.ts`).
- `e2e/slowest-tests-reporter.ts`, wired up in `e2e/playwright.config.ts`
  alongside the `list` reporter that already times each case.

The E2E job retries a failed case **once** on CI, not twice. A retry buys one
thing — telling a genuinely flaky test from a broken one — and one is enough
for that; the second attempt only lets a test that fails half the time pass
anyway, which is the outcome R6 exists to prevent.

`go test` has no equivalent here on purpose: getting per-test durations out of
it means `-json`, and that trades readable failure output for a report on a job
whose tests take 10 seconds. It gets `-timeout 2m` per package binary instead,
which is what makes a hung test panic with its goroutine dump while the runner
is still alive — a job killed by `timeout-minutes` leaves no stack at all.

## Where this fits

- `CLAUDE.md` working rule 3 routes here for the level decision.
- `.claude/rules/20-judgment.md` holds the minimum checks per area — which
  commands to run, as opposed to which level to write at.
- `.claude/rules/50-lessons.md` holds the verified traps per level.
- [Mutation testing](mutation-testing.md) asks the next question down: given a
  test at the right level, does it actually check anything?
