import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { importBookAs, helloFixturePath } from './support/books';
import {
  connectMobile,
  reopenMobileAt,
  getBookIdByTitle,
  downloadBookViaHook,
  showMobileReaderControls
} from './support/mobile';


const getServer = useServer();

test('redirects unavailable mobile routes to the library', { tag: '@smoke' }, async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'readonly-redirect');
  await connectMobile(page, baseUrl);
  const bookId = await getBookIdByTitle(page, 'readonly-redirect');

  for (const route of [
    `/books/${bookId}/sources`,
    '/admin/logs',
    '/trash',
    '/duplicates'
  ]) {
    await reopenMobileAt(page, baseUrl, route);
    await expect(page, `${route} should redirect to the library`).toHaveURL(/\/books(\?|$)/);
  }
});

test('keeps reading routes reachable', { tag: '@smoke' }, async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'readonly-routes');
  await connectMobile(page, baseUrl);

  for (const [route, pattern] of [
    ['/home', /\/home/],
    ['/dashboard', /\/home/],
    ['/books', /\/books/],
    ['/read-history', /\/read-history/],
    ['/downloads', /\/downloads/],
    ['/settings', /\/settings/],
    ['/books/maintenance/missing-author', /\/books\?[^#]*author=none/],
    ['/books/maintenance/missing-cover', /\/books\?[^#]*cover=none/],
    ['/books/maintenance/missing-language', /\/books\?[^#]*language=none/]
  ] as const) {
    await reopenMobileAt(page, baseUrl, route);
    await expect(page, `${route} should stay reachable`).toHaveURL(pattern);
  }
});

test('hides write affordances in the library and on book detail', { tag: '@smoke' }, async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'readonly-affordances');
  await connectMobile(page, baseUrl);
  const bookId = await getBookIdByTitle(page, 'readonly-affordances');

  await reopenMobileAt(page, baseUrl, '/books');
  await expect(page.getByRole('link', { name: 'Trash' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: /^Import/ })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Add folder' })).toHaveCount(0);
  await expect(page.getByRole('link', { name: 'Logs' })).toHaveCount(0);
  await expect(page.getByRole('navigation', { name: 'Primary' })).toHaveCount(1);
  await expect(page.getByRole('link', { name: 'Downloads' })).toHaveCount(1);
  await expect(page.getByRole('link', { name: 'Library' })).toHaveCount(1);
  await expect(page.locator('.read-only-banner')).toHaveCount(0);

  await reopenMobileAt(page, baseUrl, `/books/${bookId}`);
  await expect(page.getByRole('button', { name: 'More' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Cover options' })).toHaveCount(0);
});

test('rejects a write from the mobile client but still records reading on the device', { tag: '@smoke' }, async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'readonly-write');

  const bootstrapTokenForMobileConnection = await page.evaluate(
    () =>
      (window as unknown as { __PLAINSHELF_SECURITY__?: { token?: string } })
        .__PLAINSHELF_SECURITY__?.token ?? ''
  );
  expect(bootstrapTokenForMobileConnection).not.toBe('');

  await connectMobile(page, baseUrl, { token: bootstrapTokenForMobileConnection });
  const bookId = await getBookIdByTitle(page, 'readonly-write');
  await reopenMobileAt(page, baseUrl, '/books');

  await page.waitForFunction(() => Boolean(window.__plainshelfTestHooks));

  const updateBookType = await page.evaluate(
    () =>
      typeof (window.__plainshelfTestHooks?.provider as unknown as { updateBook?: unknown })
        ?.updateBook
  );
  expect(updateBookType).toBe('undefined');

  const writeResult = await page.evaluate(() => {
    const hooks = window.__plainshelfTestHooks;
    if (!hooks) {
      return 'no-hook';
    }
    try {
      hooks.bookshelfWriter();
      return 'resolved';
    } catch (err) {
      return err instanceof Error ? err.message : String(err);
    }
  });
  expect(writeResult).toContain('read-only');

  await downloadBookViaHook(page, bookId);

  const deviceStateRequests: string[] = [];
  page.on('request', (request) => {
    const url = request.url();
    if (url.includes('/read_history') || url.includes('/reading_activity')) {
      deviceStateRequests.push(`${request.method()} ${url}`);
    }
  });
  await reopenMobileAt(page, baseUrl, `/reader/${bookId}`);
  await showMobileReaderControls(page);
  await expect(page.getByRole('button', { name: /bookmark/i })).toHaveCount(0);
  await expect(page.getByText('Loading content...')).toHaveCount(0);
  expect(deviceStateRequests).toEqual([]);

  await reopenMobileAt(page, baseUrl, '/read-history');
  await expect(page.getByRole('heading', { name: 'readonly-write', exact: true }).first()).toBeVisible();
});
