import { defineConfig, devices } from '@playwright/test';

// PSW-111: which rendering engines this run covers. Chromium alone was every
// run there was, which left WKWebView — what Wails ships inside the macOS cask
// — never exercised by a single E2E case, `desktop-shell.spec.ts` included.
//
// Playwright's `webkit` is not Safari and not WKWebView; it is the same
// WebKit core with a different embedder, and it is the closest of the three
// that runs on a Linux runner. It catches chromium-only assumptions. It does
// not make "green" mean "macOS desktop works".
//
// Firefox is deliberately absent: Gecko is not an engine this project ships
// against anywhere (Wails is WKWebView on macOS, WebView2 on Windows,
// WebKitGTK on Linux; Android is a Chromium WebView), so a firefox round
// would cost a third of the nightly to test a target with no users. It stays
// one `E2E_BROWSERS=firefox` away for anyone who wants to look.
const BROWSER_DEVICES = {
  chromium: devices['Desktop Chrome'],
  firefox: devices['Desktop Firefox'],
  webkit: devices['Desktop Safari']
};

// Default to chromium so the pull request gate and a local `npm test` are
// unchanged and need no extra browser download; `nightly.yml` opts into the
// wider matrix. An unknown name throws rather than silently running nothing.
type BrowserName = keyof typeof BROWSER_DEVICES;

// `Object.keys`, not `name in BROWSER_DEVICES`: `in` walks the prototype, so
// `E2E_BROWSERS=constructor` would pass the check and then spread a project
// with no browser in it — which Playwright runs as chromium, silently.
const BROWSER_NAMES = Object.keys(BROWSER_DEVICES) as BrowserName[];

const requestedBrowsers = (process.env.E2E_BROWSERS ?? 'chromium')
  .split(',')
  .map((name) => name.trim())
  .filter(Boolean)
  .map((name) => {
    if (!BROWSER_NAMES.includes(name as BrowserName)) {
      throw new Error(
        `E2E_BROWSERS: unknown browser "${name}"; expected one of ${BROWSER_NAMES.join(', ')}`
      );
    }
    return name as BrowserName;
  });

// `?? 'chromium'` covers an unset variable, not an empty or comma-only one:
// that would leave zero projects, and a run of zero tests exits 0.
if (requestedBrowsers.length === 0) {
  throw new Error(`E2E_BROWSERS: no browser selected; expected one of ${BROWSER_NAMES.join(', ')}`);
}

export default defineConfig({
  testDir: './tests',
  globalSetup: './tests/support/globalSetup.ts',
  // PSW-77: safe because a server is per spec file, not per suite —
  // `startServer()` mints its own temp shelf/store and its own port on every
  // call, so two workers never share state. The unit of parallelism is the
  // worker, and `useServer()`'s beforeAll runs once in each of them.
  fullyParallel: true,
  // GitHub's ubuntu-latest is 4 vCPU and every worker costs a Chromium *and* a
  // plainshelf-srv; 2 measured 2:03 against 1:47 for 4 on the same 4-core box,
  // which is not worth the contention on a shared runner. Locally, half the
  // cores is Playwright's own default.
  workers: process.env.CI ? 2 : '50%',
  // PSW-77: one retry, not two. A retry is time the job spends to tell you
  // nothing; the second one exists only to bury a test that fails half the
  // time, and R6 in docs/development/testing-levels.md says such a test gets
  // fixed or deleted instead.
  retries: process.env.CI ? 1 : 0,
  // PSW-76: `list` already times each case; the second reporter sorts them so
  // the expensive end is visible without reading the whole log.
  reporter: [['list'], ['./slowest-tests-reporter.ts']],
  use: {
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure'
  },
  projects: requestedBrowsers.map((name) => ({
    name,
    use: { ...BROWSER_DEVICES[name] }
  }))
});
