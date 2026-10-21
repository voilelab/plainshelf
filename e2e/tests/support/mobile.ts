import { expect, type Page } from '@playwright/test';

export const MOBILE_PREVIEW_QUERY = 'mobile-shell-preview=1';

declare global {
  interface Window {
    __plainshelfTestHooks?: {
      bookshelfWriter: () => unknown;
      provider: {
        listBooks: (
          page?: number,
          pageSize?: number,
          search?: string
        ) => Promise<{ items: Array<{ id: string; title: string }> }>;
        getBook: (bookId: string) => Promise<{ current_source?: string }>;
        getBookContent: (bookId: string) => Promise<{ content: string }>;
        getSourceContent: (bookId: string, sourceId: string) => Promise<string>;
        downloadBook?: (bookId: string) => Promise<void>;
        removeDownload?: (bookId: string) => Promise<void>;
        getDownloadState?: (bookId: string) => Promise<string>;
        listDownloadedBookEntries?: () => Promise<
          Array<{ book: { id: string }; sizeBytes: number; downloadedAt: string }>
        >;
        getReadProgress: (bookId: string) => Promise<{ char_offset: number; percent?: number }>;
      };
    };
  }
}

function withMobilePreview(route: string): string {
  const separator = route.includes('?') ? '&' : '?';
  return `${route}${separator}${MOBILE_PREVIEW_QUERY}`;
}

export async function connectMobile(
  page: Page,
  baseUrl: string,
  opts: { shelfName?: string; token?: string } = {}
): Promise<void> {
  const shelfName = opts.shelfName ?? 'Default Shelf';

  await page.goto(`${baseUrl}${withMobilePreview('/connect')}`);
  await expect(page.getByRole('heading', { name: 'Connect to PlainShelf' })).toBeVisible();

  await page.locator('input[type="url"]').fill(baseUrl);
  if (opts.token) {
    await page.locator('input[type="password"]').fill(opts.token);
  }

  await page.getByRole('button', { name: 'Load library' }).click();

  // Reka UI 2.10.5 SelectPortal teleports to body by default; SelectTrigger renders a combobox button.
  const shelfTrigger = page.locator('.mobile-connect-shelf-select');
  await expect(shelfTrigger).toBeEnabled();
  await shelfTrigger.click();
  await page.getByRole('option', { name: shelfName }).click();
  await expect(shelfTrigger).toHaveText(shelfName);

  const saveButton = page.getByRole('button', { name: 'Save and continue' });
  await expect(saveButton).toBeEnabled();
  await saveButton.click();
  await expect(page).toHaveURL(/\/books(\?|$)/);
  await waitForMobileApp(page);
}

export async function waitForMobileApp(page: Page): Promise<void> {
  await page.waitForFunction(() => Boolean(window.__plainshelfTestHooks));
}

export async function addMobileShelf(
  page: Page,
  baseUrl: string,
  opts: { name: string; shelfName: string }
): Promise<void> {
  await reopenMobileAt(page, baseUrl, '/connect');
  await page.getByRole('button', { name: 'Add a shelf' }).click();

  await page.getByLabel('Name').fill(opts.name);
  await page.locator('input[type="url"]').fill(baseUrl);
  await page.getByRole('button', { name: 'Load library' }).click();

  const shelfTrigger = page.locator('.mobile-connect-shelf-select');
  await expect(shelfTrigger).toBeEnabled();
  await shelfTrigger.click();
  await page.getByRole('option', { name: opts.shelfName }).click();

  await page.getByRole('button', { name: 'Save and continue' }).click();
  await expect(page).toHaveURL(/\/books(\?|$)/);
  await waitForMobileApp(page);
}

export async function openMobileShelfEditor(
  page: Page,
  baseUrl: string,
  shelfName?: string
): Promise<void> {
  await reopenMobileAt(page, baseUrl, '/connect');
  const row = shelfName
    ? page.locator('.mobile-shelves-item').filter({ hasText: shelfName })
    : page.locator('.mobile-shelves-item').first();
  await row.getByRole('button', { name: 'Edit' }).click();
  await expect(page.getByRole('heading', { name: 'Edit shelf' })).toBeVisible();
}

export async function reopenMobileAt(page: Page, baseUrl: string, route: string): Promise<void> {
  await page.goto(`${baseUrl}${withMobilePreview(route)}`);
  await waitForMobileApp(page);
}

export async function showMobileReaderControls(page: Page): Promise<void> {
  const reader = page.locator('[data-reader-variant="mobile"]');
  await expect(reader).toBeVisible();
  const toolbar = reader.locator('.mobile-reader-toolbar');
  if (await toolbar.isVisible()) return;

  const box = await reader.boundingBox();
  if (!box) throw new Error('Mobile reader has no visible bounding box');
  const point = { x: box.x + box.width / 2, y: box.y + box.height / 2 };
  await reader.dispatchEvent('pointerdown', {
    pointerType: 'mouse', pointerId: 1, isPrimary: true, button: 0, clientX: point.x, clientY: point.y
  });
  await reader.dispatchEvent('pointerup', {
    pointerType: 'mouse', pointerId: 1, isPrimary: true, button: 0, clientX: point.x, clientY: point.y
  });
  await expect(toolbar).toBeVisible();
}

async function setNavigatorOnline(page: Page, online: boolean): Promise<void> {
  await page.evaluate((isOnline) => {
    Object.defineProperty(window.navigator, 'onLine', { get: () => isOnline, configurable: true });
  }, online);
}

// Playwright 1.62.1 context offline mode also blocks top-level document requests; Capacitor Android 8.5.2 loads its shell from installed app assets.
export async function goOffline(page: Page): Promise<void> {
  await page.route('**/api/**', (route) => route.abort('internetdisconnected'));
  await page.addInitScript((isOnline) => {
    Object.defineProperty(window.navigator, 'onLine', { get: () => isOnline, configurable: true });
  }, false);
  await setNavigatorOnline(page, false);
}

export async function goOnline(page: Page): Promise<void> {
  await page.unroute('**/api/**');
  await page.addInitScript((isOnline) => {
    Object.defineProperty(window.navigator, 'onLine', { get: () => isOnline, configurable: true });
  }, true);
  await setNavigatorOnline(page, true);
}

export async function getBookIdByTitle(page: Page, title: string): Promise<string> {
  const id = await page.evaluate(async (bookTitle) => {
    const provider = window.__plainshelfTestHooks?.provider;
    if (!provider) {
      throw new Error('__plainshelfTestHooks is not attached; is the page in mobile-shell-preview mode?');
    }
    const { items } = await provider.listBooks(1, 50);
    return items.find((book) => book.title === bookTitle)?.id ?? null;
  }, title);

  if (!id) {
    throw new Error(`No book titled "${title}" found via the test hook.`);
  }
  return id;
}

export async function downloadBookViaHook(page: Page, bookId: string): Promise<void> {
  await page.evaluate(async (id) => {
    const provider = window.__plainshelfTestHooks?.provider;
    await provider?.downloadBook?.(id);
  }, bookId);
}

export async function removeDownloadViaHook(page: Page, bookId: string): Promise<void> {
  await page.evaluate(async (id) => {
    const provider = window.__plainshelfTestHooks?.provider;
    await provider?.removeDownload?.(id);
  }, bookId);
}

export async function getDownloadStateViaHook(page: Page, bookId: string): Promise<string> {
  return page.evaluate(async (id) => {
    const provider = window.__plainshelfTestHooks?.provider;
    return (await provider?.getDownloadState?.(id)) ?? 'not_downloaded';
  }, bookId);
}

export async function getReadProgressViaHook(
  page: Page,
  bookId: string
): Promise<{ char_offset: number; percent?: number }> {
  return page.evaluate(async (id) => {
    const provider = window.__plainshelfTestHooks?.provider;
    if (!provider) {
      throw new Error('__plainshelfTestHooks is not attached; is the page in mobile-shell-preview mode?');
    }
    return provider.getReadProgress(id);
  }, bookId);
}

export async function getCurrentSourceIdViaHook(page: Page, bookId: string): Promise<string> {
  return page.evaluate(async (id) => {
    const provider = window.__plainshelfTestHooks?.provider;
    if (!provider) {
      throw new Error('__plainshelfTestHooks is not attached; is the page in mobile-shell-preview mode?');
    }
    const book = await provider.getBook(id);
    if (!book.current_source) {
      throw new Error(`Book ${id} has no current_source`);
    }
    return book.current_source;
  }, bookId);
}

export async function getBookContentViaHook(page: Page, bookId: string): Promise<string | null> {
  return page.evaluate(async (id) => {
    const provider = window.__plainshelfTestHooks?.provider;
    if (!provider) {
      throw new Error('__plainshelfTestHooks is not attached; is the page in mobile-shell-preview mode?');
    }
    try {
      const { content } = await provider.getBookContent(id);
      return content;
    } catch {
      return null;
    }
  }, bookId);
}

export async function getSourceContentViaHook(page: Page, bookId: string, sourceId: string): Promise<string | null> {
  return page.evaluate(
    async ({ id, srcId }) => {
      const provider = window.__plainshelfTestHooks?.provider;
      if (!provider) {
        throw new Error('__plainshelfTestHooks is not attached; is the page in mobile-shell-preview mode?');
      }
      try {
        return await provider.getSourceContent(id, srcId);
      } catch {
        return null;
      }
    },
    { id: bookId, srcId: sourceId }
  );
}

export async function getDownloadedEntryViaHook(
  page: Page,
  bookId: string
): Promise<{ sizeBytes: number; downloadedAt: string } | null> {
  return page.evaluate(async (id) => {
    const provider = window.__plainshelfTestHooks?.provider;
    if (!provider?.listDownloadedBookEntries) {
      throw new Error('provider.listDownloadedBookEntries is not available; is the page in mobile-shell-preview mode?');
    }
    const entries = await provider.listDownloadedBookEntries();
    const entry = entries.find((item) => item.book.id === id);
    return entry ? { sizeBytes: entry.sizeBytes, downloadedAt: entry.downloadedAt } : null;
  }, bookId);
}
