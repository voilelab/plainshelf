import { defineConfig, devices } from '@playwright/test';

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
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome']
      }
    }
  ]
});
