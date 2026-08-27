import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { importBookAs, helloFixturePath, anotherFixturePath } from './support/books';
import {
  connectMobile,
  openMobileShelfEditor,
  reopenMobileAt,
  getBookIdByTitle,
  downloadBookViaHook,
  removeDownloadViaHook,
  getDownloadStateViaHook,
  getReadProgressViaHook,
  getCurrentSourceIdViaHook,
  getBookContentViaHook,
  getSourceContentViaHook,
  goOffline,
  goOnline,
  goServerUnreachable,
  showMobileReaderControls
} from './support/mobile';

// These tests exercise the Android app storage layer (frontend/src/providers/
// mobileConfig.ts + filesystemMobileBookCache.ts + mobileBookshelfProvider.ts)
// by running the desktop Chromium build with `?mobile-shell-preview=1`, which
// makes isMobileRuntime() true and swaps in the same MobileBookshelfProvider
// used by the native Capacitor shell — no Android emulator required.
//
// The server (and its shelf) is shared across this file's tests, so each test
// imports its own uniquely-named book(s); the device-side state (saved shelves,
// downloads, read progress) lives in per-test browser storage and resets on its
// own.

const getServer = useServer();

test('persists mobile connection settings across app restarts', async ({ page }) => {
  const { baseUrl } = getServer();

  await connectMobile(page, baseUrl);
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

  // Simulate an app restart: a fresh top-level navigation, same as a cold
  // launch of the native shell. The router guard (router.ts beforeEach)
  // would bounce back to /connect if the saved serverUrl/shelfId did not
  // survive the "restart".
  await reopenMobileAt(page, baseUrl, '/books');
  await expect(page).toHaveURL(/\/books(\?|$)/);
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
});

test('does not resurrect the saved shelf when validating an unreachable server on /connect', async ({ page }) => {
  const { baseUrl } = getServer();

  await connectMobile(page, baseUrl);

  // Reopen the saved shelf for editing (Settings → "Manage shelves" in the
  // real app) and point it at a server that cannot be reached. The failed
  // shelf fetch must NOT fall back to the previously saved shelf id,
  // otherwise "Save and continue" would persist a stale shelf for a server
  // that was never validated.
  await openMobileShelfEditor(page, baseUrl);
  const urlInput = page.locator('input[type="url"]');
  await expect(urlInput).toHaveValue(baseUrl);

  await urlInput.fill('http://127.0.0.1:9');
  await page.getByRole('button', { name: 'Load library' }).click();

  await expect(page.locator('.mobile-connect-error')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Save and continue' })).toBeDisabled();
});

test('downloads books for offline reading and isolates removal between books', async ({ page }) => {
  const { baseUrl } = getServer();

  // Import two books in the ordinary (non-mobile) desktop flow first: the
  // mobile provider wraps the same server via ServerBookshelfProvider, but
  // the mobile client is read-only and cannot POST (see
  // mobile-read-only.spec.ts), so importing is done here. Unique titles keep
  // them distinct from other tests' books on the shared shelf.
  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'storage-offline-hello');
  await importBookAs(page, anotherFixturePath, 'storage-offline-another');

  await connectMobile(page, baseUrl);

  const helloId = await getBookIdByTitle(page, 'storage-offline-hello');
  const anotherId = await getBookIdByTitle(page, 'storage-offline-another');
  expect(helloId).not.toBe(anotherId);

  await downloadBookViaHook(page, helloId);
  await downloadBookViaHook(page, anotherId);

  expect(await getDownloadStateViaHook(page, helloId)).toBe('downloaded');
  expect(await getDownloadStateViaHook(page, anotherId)).toBe('downloaded');

  const helloSourceId = await getCurrentSourceIdViaHook(page, helloId);
  const anotherSourceId = await getCurrentSourceIdViaHook(page, anotherId);

  // Go offline and re-launch the app: reading a downloaded book must work
  // entirely from the offline cache, with no network round trip.
  await goOffline(page);
  await reopenMobileAt(page, baseUrl, '/books');
  const helloRow = page
    .locator('.book-list-row')
    .getByRole('heading', { name: 'storage-offline-hello', exact: true });
  await expect(helloRow).toBeVisible();
  await helloRow.click();
  await expect(page).toHaveURL(/\/books\/[^/]+$/);
  await expect(page.getByRole('button', { name: 'Start reading' })).toBeVisible();
  // Enter the reader via a top-level navigation instead of clicking "Read":
  // the in-app push drops the ?mobile-shell-preview=1 param, so the freshly
  // mounting ReaderLayout would see isMobileRuntime() === false. This is an
  // artifact of the desktop preview only — native Capacitor reports the
  // platform regardless of URL, so the click-through path works there.
  await reopenMobileAt(page, baseUrl, `/reader/${helloId}`);
  await expect(page.getByText('Hello from PlainShelf E2E.')).toBeVisible();

  await goOnline(page);
  await removeDownloadViaHook(page, helloId);

  // Removing hello's offline cache must not clobber another's cached
  // content — go offline again so getBookContent/getSourceContent are
  // forced to read from the local cache only (no remote fallback), and
  // assert purely on provider-visible behavior rather than any particular
  // cache backend's storage layout.
  await goOffline(page);

  expect(await getDownloadStateViaHook(page, helloId)).toBe('not_downloaded');
  expect(await getDownloadStateViaHook(page, anotherId)).toBe('downloaded');

  expect(await getBookContentViaHook(page, helloId)).toBeNull();
  expect(await getSourceContentViaHook(page, helloId, helloSourceId)).toBeNull();

  expect(await getBookContentViaHook(page, anotherId)).toContain('Another book for PlainShelf E2E.');
  expect(await getSourceContentViaHook(page, anotherId, anotherSourceId)).toContain(
    'Another book for PlainShelf E2E.'
  );
});

test('automatically persists reading progress across app restarts', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'storage-progress');

  await connectMobile(page, baseUrl);
  const helloId = await getBookIdByTitle(page, 'storage-progress');
  await downloadBookViaHook(page, helloId);

  await reopenMobileAt(page, baseUrl, `/reader/${helloId}`);
  await expect(page.getByText('Hello from PlainShelf E2E.')).toBeVisible();

  await showMobileReaderControls(page);
  await expect(page.getByRole('button', { name: /bookmark/i })).toHaveCount(0);

  // The fixture is intentionally short, so give its scroll container a
  // deterministic viewport for this storage test and dispatch the same event
  // a real scroll produces. Leaving through Vue Router then awaits autosave's
  // final flush before the reader unmounts.
  await page.locator('.reader-content').evaluate((element) => {
    Object.defineProperty(element, 'scrollHeight', { configurable: true, value: 1_000 });
    Object.defineProperty(element, 'clientHeight', { configurable: true, value: 100 });
    Object.defineProperty(element, 'scrollTop', { configurable: true, value: 450, writable: true });
    element.dispatchEvent(new Event('scroll'));
  });
  await page.getByRole('button', { name: 'Back' }).click();
  await expect(page).toHaveURL(new RegExp(`/books/${helloId}$`));

  const savedProgress = await getReadProgressViaHook(page, helloId);
  expect(savedProgress.char_offset).toBeGreaterThan(0);

  await goOffline(page);
  await reopenMobileAt(page, baseUrl, `/reader/${helloId}`);
  await expect(page.getByText('Hello from PlainShelf E2E.')).toBeVisible();

  // Authoritative check: read back the persisted value directly, rather
  // than relying on visually inferring scroll position restoration.
  const reloadedProgress = await getReadProgressViaHook(page, helloId);
  expect(reloadedProgress.char_offset).toBe(savedProgress.char_offset);
});

test('accesses downloaded books when the device has network but cannot reach the server', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'storage-unreachable');

  await connectMobile(page, baseUrl);
  const helloId = await getBookIdByTitle(page, 'storage-unreachable');
  await downloadBookViaHook(page, helloId);

  // Simulate LTE-with-no-route-to-the-home-server: navigator.onLine stays
  // true (unlike goOffline), so listBooks must fall back to the offline
  // cache rather than surfacing the transport error to the UI.
  await goServerUnreachable(page);
  await reopenMobileAt(page, baseUrl, '/books');
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
  await expect(
    page.locator('.book-list-row').getByRole('heading', { name: 'storage-unreachable', exact: true })
  ).toBeVisible();

  await reopenMobileAt(page, baseUrl, `/reader/${helloId}`);
  await expect(page.getByText('Hello from PlainShelf E2E.')).toBeVisible();
});

test('redirects a non-downloaded book to its detail page instead of opening the reader', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'storage-redirect');

  await connectMobile(page, baseUrl);
  const helloId = await getBookIdByTitle(page, 'storage-redirect');
  // Deliberately not downloaded.

  await goOffline(page);

  await reopenMobileAt(page, baseUrl, '/books');
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
  // Offline listBooks only returns downloaded books, so the not-downloaded
  // book must not appear.
  await expect(
    page.locator('.book-list-row').getByRole('heading', { name: 'storage-redirect', exact: true })
  ).not.toBeVisible();

  // The mobile client requires a download before reading. Opening the reader
  // route for a not-downloaded book redirects to the book's detail page, which
  // prompts the user to download it first — the reader content never loads.
  await reopenMobileAt(page, baseUrl, `/reader/${helloId}`);
  await expect(page).not.toHaveURL(/\/reader\//);
  await expect(page.locator('.download-required-notice')).toBeVisible();
  await expect(page.getByText('Hello from PlainShelf E2E.')).toHaveCount(0);
});
