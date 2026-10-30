import { defineConfig, devices } from '@playwright/test';

const BROWSER_DEVICES = {
  chromium: devices['Desktop Chrome'],
  firefox: devices['Desktop Firefox'],
  webkit: devices['Desktop Safari']
};

type BrowserName = keyof typeof BROWSER_DEVICES;

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

if (requestedBrowsers.length === 0) {
  throw new Error(`E2E_BROWSERS: no browser selected; expected one of ${BROWSER_NAMES.join(', ')}`);
}

export default defineConfig({
  testDir: './tests',
  globalSetup: './tests/support/globalSetup.ts',
  fullyParallel: true,
  workers: process.env.CI ? 2 : '50%',
  retries: process.env.CI ? 1 : 0,
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
