import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook, importBookFromPath, anotherFixturePath } from './support/books';
import {
  connectMobile,
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

test('persists mobile connection settings across app restarts', async ({ page }) => {
  const server = await startServer();

  try {
    await connectMobile(page, server.baseUrl);
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

    // Simulate an app restart: a fresh top-level navigation, same as a cold
    // launch of the native shell. The router guard (router.ts beforeEach)
    // would bounce back to /connect if the saved serverUrl/shelfId did not
    // survive the "restart".
    await reopenMobileAt(page, server.baseUrl, '/books');
    await expect(page).toHaveURL(/\/books(\?|$)/);
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('does not resurrect the saved shelf when validating an unreachable server on /connect', async ({ page }) => {
  const server = await startServer();

  try {
    await connectMobile(page, server.baseUrl);

    // Revisit the connect page (Settings → "Edit connection" in the real app)
    // and point it at a server that cannot be reached. The failed shelf fetch
    // must NOT fall back to the previously saved shelf id (that fallback is
    // reserved for the routed content layouts), otherwise "Save and continue"
    // would persist a stale shelf for a server that was never validated.
    await reopenMobileAt(page, server.baseUrl, '/connect');
    const urlInput = page.locator('input[type="url"]');
    await expect(urlInput).toHaveValue(server.baseUrl);

    await urlInput.fill('http://127.0.0.1:9');
    await page.getByRole('button', { name: 'Load library' }).click();

    await expect(page.locator('.mobile-connect-error')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Save and continue' })).toBeDisabled();
  } finally {
    await server.dispose();
  }
});

test('downloads books for offline reading and isolates removal between books', async ({ page }) => {
  const server = await startServer();

  try {
    // Import two books in the ordinary (non-mobile) desktop flow first: the
    // mobile provider wraps the same server via ServerBookshelfProvider, but
    // the mobile client is read-only and cannot POST (see
    // mobile-read-only.spec.ts), so importing is done here.
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    await importBookFromPath(page, anotherFixturePath);
    await expect(page.getByText('2 books')).toBeVisible();

    await connectMobile(page, server.baseUrl);

    const helloId = await getBookIdByTitle(page, 'hello');
    const anotherId = await getBookIdByTitle(page, 'another');
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
    await reopenMobileAt(page, server.baseUrl, '/books');
    const helloRow = page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true });
    await expect(helloRow).toBeVisible();
    await helloRow.click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await expect(page.getByRole('button', { name: 'Start reading' })).toBeVisible();
    // Enter the reader via a top-level navigation instead of clicking "Read":
    // the in-app push drops the ?mobile-shell-preview=1 param, so the freshly
    // mounting ReaderLayout would see isMobileRuntime() === false. This is an
    // artifact of the desktop preview only — native Capacitor reports the
    // platform regardless of URL, so the click-through path works there.
    await reopenMobileAt(page, server.baseUrl, `/reader/${helloId}`);
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
  } finally {
    await server.dispose();
  }
});

test('persists reading progress (bookmark) across app restarts', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    await connectMobile(page, server.baseUrl);
    const helloId = await getBookIdByTitle(page, 'hello');
    await downloadBookViaHook(page, helloId);

    await reopenMobileAt(page, server.baseUrl, `/reader/${helloId}`);
    await expect(page.getByText('Hello from PlainShelf E2E.')).toBeVisible();

    await showMobileReaderControls(page);
    await page.getByRole('button', { name: 'Save bookmark' }).click();
    await expect(page.getByRole('button', { name: 'Save bookmark' })).toBeEnabled();

    // hello.txt is short enough that scrolling may not move char_offset off
    // 0, so assert a progress record exists rather than a strict > 0 offset.
    const savedProgress = await getReadProgressViaHook(page, helloId);
    expect(savedProgress).toBeTruthy();
    expect(typeof savedProgress.char_offset).toBe('number');

    await goOffline(page);
    await reopenMobileAt(page, server.baseUrl, `/reader/${helloId}`);
    await expect(page.getByText('Hello from PlainShelf E2E.')).toBeVisible();

    // Authoritative check: read back the persisted value directly, rather
    // than relying on visually inferring scroll position restoration.
    const reloadedProgress = await getReadProgressViaHook(page, helloId);
    expect(reloadedProgress.char_offset).toBe(savedProgress.char_offset);
  } finally {
    await server.dispose();
  }
});

test('accesses downloaded books when the device has network but cannot reach the server', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    await connectMobile(page, server.baseUrl);
    const helloId = await getBookIdByTitle(page, 'hello');
    await downloadBookViaHook(page, helloId);

    // Simulate LTE-with-no-route-to-the-home-server: navigator.onLine stays
    // true (unlike goOffline), so listBooks must fall back to the offline
    // cache rather than surfacing the transport error to the UI.
    await goServerUnreachable(page);
    await reopenMobileAt(page, server.baseUrl, '/books');
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true })
    ).toBeVisible();

    await reopenMobileAt(page, server.baseUrl, `/reader/${helloId}`);
    await expect(page.getByText('Hello from PlainShelf E2E.')).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('shows a clear error when opening a non-downloaded book while offline', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    await connectMobile(page, server.baseUrl);
    const helloId = await getBookIdByTitle(page, 'hello');
    // Deliberately not downloaded.

    await goOffline(page);

    await reopenMobileAt(page, server.baseUrl, '/books');
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
    // Offline listBooks only returns downloaded books, so the not-downloaded
    // book must not appear.
    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true })
    ).not.toBeVisible();

    await reopenMobileAt(page, server.baseUrl, `/reader/${helloId}`);
    await expect(page.getByRole('alert')).toContainText('not downloaded');
  } finally {
    await server.dispose();
  }
});
