import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';
import { connectMobile, reopenMobileAt, getBookIdByTitle } from './support/mobile';

// The Android app is a read-only reading client. These tests run the desktop
// Chromium build with `?mobile-shell-preview=1`, which makes isMobileRuntime()
// true — the same switch the native Capacitor shell flips — so the routing
// guard, the hidden write affordances, and the API-client rejection are all
// exercised without an emulator.
//
// Books are imported in the ordinary desktop flow first: importing is a write,
// which the mobile client no longer performs.

test('redirects edit-only routes to the library', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    await connectMobile(page, server.baseUrl);
    const bookId = await getBookIdByTitle(page, 'hello');

    for (const route of [
      `/books/${bookId}/edit`,
      `/books/${bookId}/sources`,
      '/trash',
      '/duplicates',
      '/books/maintenance/missing-author',
      '/books/maintenance/missing-cover',
      '/books/maintenance/missing-language',
      '/books/maintenance/low-char-count'
    ]) {
      await reopenMobileAt(page, server.baseUrl, route);
      await expect(page, `${route} should redirect to the library`).toHaveURL(/\/books(\?|$)/);
    }
  } finally {
    await server.dispose();
  }
});

test('keeps reading routes reachable', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    await connectMobile(page, server.baseUrl);

    for (const [route, pattern] of [
      ['/dashboard', /\/dashboard/],
      ['/books', /\/books/],
      ['/read-history', /\/read-history/],
      ['/downloads', /\/downloads/],
      ['/settings', /\/settings/]
    ] as const) {
      await reopenMobileAt(page, server.baseUrl, route);
      await expect(page, `${route} should stay reachable`).toHaveURL(pattern);
    }
  } finally {
    await server.dispose();
  }
});

test('hides write affordances in the library and on book detail', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    await connectMobile(page, server.baseUrl);
    const bookId = await getBookIdByTitle(page, 'hello');

    await reopenMobileAt(page, server.baseUrl, '/books');
    // Trash, the maintenance section, and import are edit-only.
    await expect(page.getByRole('link', { name: 'Trash' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /^Import/ })).toHaveCount(0);
    // Downloads is a local-device action and must survive.
    await expect(page.getByRole('link', { name: 'Downloads' })).toHaveCount(1);
    // The banner is for a read-only *server*, not for this platform.
    await expect(page.locator('.read-only-banner')).toHaveCount(0);

    await reopenMobileAt(page, server.baseUrl, `/books/${bookId}`);
    await expect(page.getByRole('button', { name: 'Edit metadata' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Edit Sources' })).toHaveCount(0);
  } finally {
    await server.dispose();
  }
});

test('stays read-only after an in-app navigation drops the preview query', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    // connectMobile ends with MobileConnectPage's own `router.push('/books')`,
    // an in-app navigation that strips ?mobile-shell-preview=1. Assert here
    // deliberately WITHOUT reopenMobileAt: before the runtime was latched,
    // isMobileRuntime() went false at this point and every guard disengaged.
    await connectMobile(page, server.baseUrl);
    await expect(page).toHaveURL(/\/books(\?|$)/);

    await expect(page.getByRole('button', { name: /^Import/ })).toHaveCount(0);
    await expect(page.getByRole('link', { name: 'Trash' })).toHaveCount(0);
    // Rendered but inert, matching the read-only-server behavior.
    await expect(page.getByRole('button', { name: 'Add layer' })).toBeDisabled();
  } finally {
    await server.dispose();
  }
});

test('offers clear-history but ignores the import query on mobile', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    await connectMobile(page, server.baseUrl);

    // Clearing history only touches this device's own storage — no request
    // reaches the server — so the read-only shell still offers it.
    await reopenMobileAt(page, server.baseUrl, '/read-history');
    await expect(page.getByRole('heading', { name: 'Recently Read' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Clear history' })).toBeVisible();

    // The retention limit is device-local too, so its settings tab is reachable
    // on mobile even though the server-only tabs are not.
    await reopenMobileAt(page, server.baseUrl, '/settings');
    await page.getByRole('tab', { name: 'Reading history' }).click();
    await expect(page.getByText('Reading history limit')).toBeVisible();
    await expect(page.getByRole('tab', { name: 'Cover' })).toHaveCount(0);

    // /import redirects to /books?import=1, and the modal opens purely off that
    // query — a redirect the router guard cannot intercept by route name.
    await reopenMobileAt(page, server.baseUrl, '/books?import=1');
    await expect(page).toHaveURL(/\/books/);
    await expect(page.getByRole('dialog', { name: 'Import Book' })).toHaveCount(0);

    await reopenMobileAt(page, server.baseUrl, '/import');
    await expect(page.getByRole('dialog', { name: 'Import Book' })).toHaveCount(0);
  } finally {
    await server.dispose();
  }
});

test('rejects a write from the mobile client but still records read history', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    // Take the token the way a user would (the server prints it at startup) and
    // enter it in the connect form, so the allowlisted POST below is
    // authenticated from Capacitor Preferences — the only source a real APK
    // has. Relying on the server-injected window.__PLAINSHELF_SECURITY__ would
    // pass even with the mobile token wiring removed entirely.
    const serverToken = await page.evaluate(
      () =>
        (window as unknown as { __PLAINSHELF_SECURITY__?: { token?: string } })
          .__PLAINSHELF_SECURITY__?.token ?? ''
    );
    expect(serverToken).not.toBe('');

    await connectMobile(page, server.baseUrl, { token: serverToken });
    const bookId = await getBookIdByTitle(page, 'hello');
    await reopenMobileAt(page, server.baseUrl, '/books');

    // main.ts installs the hook only after awaiting initMobileConfig(), so it
    // is not present the instant `load` fires. Without this wait the evaluate
    // below can read an undefined hook and pass vacuously.
    await page.waitForFunction(() => Boolean(window.__plainshelfTestHooks));

    // A metadata PATCH must be refused on the device, before any request goes
    // out — every provider write funnels through api/client.ts.
    const writeResult = await page.evaluate(async (id) => {
      const hooks = window.__plainshelfTestHooks;
      if (!hooks) {
        return 'no-hook';
      }
      try {
        await (
          hooks.provider as unknown as {
            updateBook: (bookId: string, payload: unknown) => Promise<unknown>;
          }
        ).updateBook(id, { title: 'Should Not Persist' });
        return 'resolved';
      } catch (err) {
        return err instanceof Error ? err.message : String(err);
      }
    }, bookId);
    expect(writeResult).toContain('read-only');

    // Opening the reader records read history on the device (no request at
    // all — it is stored in app-private storage), and the bookmark button
    // stays because mobile progress is stored on-device too.
    const readHistoryRequests: string[] = [];
    page.on('request', (request) => {
      if (request.url().includes('/read_history')) {
        readHistoryRequests.push(`${request.method()} ${request.url()}`);
      }
    });
    await reopenMobileAt(page, server.baseUrl, `/reader/${bookId}`);
    await expect(page.getByRole('button', { name: /bookmark/i }).first()).toBeVisible();
    // The history entry is written while the reader is still loading, and the
    // device store is asynchronous (app-private files), so wait for the load to
    // finish before navigating away — otherwise the assertion below races the
    // in-flight write.
    await expect(page.getByText('Loading content...')).toHaveCount(0);
    expect(readHistoryRequests).toEqual([]);

    // Read history survives the reload reopenMobileAt performs, proving it was
    // persisted rather than held in memory.
    await reopenMobileAt(page, server.baseUrl, '/read-history');
    await expect(page.getByRole('heading', { name: 'hello', exact: true }).first()).toBeVisible();
  } finally {
    await server.dispose();
  }
});
