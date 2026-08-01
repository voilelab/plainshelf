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

test('rejects a write from the mobile client but still records read history', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    await connectMobile(page, server.baseUrl);
    const bookId = await getBookIdByTitle(page, 'hello');
    await reopenMobileAt(page, server.baseUrl, '/books');

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

    // Opening the reader records read history, an allowlisted POST, and the
    // bookmark button stays because mobile progress is stored on-device.
    // Awaiting the response both proves the allowlist reached the server and
    // keeps the read-history assertion below off a race with the in-flight POST.
    const readHistoryPost = page.waitForResponse(
      (res) => res.request().method() === 'POST' && res.url().includes('/read_history')
    );
    await reopenMobileAt(page, server.baseUrl, `/reader/${bookId}`);
    await expect(page.getByRole('button', { name: /bookmark/i }).first()).toBeVisible();
    expect((await readHistoryPost).ok()).toBe(true);

    await reopenMobileAt(page, server.baseUrl, '/read-history');
    await expect(page.getByRole('heading', { name: 'hello', exact: true }).first()).toBeVisible();
  } finally {
    await server.dispose();
  }
});
