import { expect, type Page } from '@playwright/test';

// isMobileRuntime()'s desktop-browser escape hatch: on a top-level goto this
// boots the mobile provider in ordinary Chromium, so the offline book cache and
// the Preferences-backed connection config are exercised without an emulator.
export const MOBILE_PREVIEW_QUERY = 'mobile-shell-preview=1';

// The hook frontend/src/main.ts attaches. Declared locally rather than imported:
// the e2e project intentionally stays a standalone TS project.
declare global {
  interface Window {
    __plainshelfTestHooks?: {
      // Refuses and throws when the active provider cannot write; the mobile
      // shell's provider never can.
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

/**
 * Drives the mobile shelf setup (`/connect`) end to end, landing on `/books`.
 * A device with no shelves has nothing to list, so `/connect` renders the add
 * form directly — which is what this walks. Use openMobileShelfEditor() once a
 * shelf exists.
 *
 * The token grants no write access: it is needed only when the server sets
 * `protect_read`, which makes reads require one too.
 */
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

  // reka-ui SelectTrigger renders as a button[role=combobox]; the listbox
  // content is portaled to <body>, so the option is queried globally rather
  // than scoped under the trigger (same pattern as e2e/tests/metadata-edit.spec.ts).
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

/**
 * Saving a shelf restarts the app, and a URL assertion is satisfied the moment
 * that navigation commits — before main.ts has attached the hooks every mobile
 * spec drives the provider through. Waiting on the hook is the signal that the
 * app, and its saved shelf, are actually up.
 */
export async function waitForMobileApp(page: Page): Promise<void> {
  await page.waitForFunction(() => Boolean(window.__plainshelfTestHooks));
}

/**
 * Adds a second shelf to a device that already has one, by walking the list
 * page's "Add a shelf" button rather than the empty-list shortcut.
 */
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

/** Opens the edit form for a saved shelf from the device's shelf list. */
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

/**
 * A top-level navigation, not an in-app router push, carrying the
 * mobile-shell-preview param: in-app navigations drop it from the URL and
 * isMobileRuntime() reads location.search on its first check.
 *
 * Waits for the boot because `goto` resolves on the document load, ahead of
 * main.ts restoring the device's shelves — a caller reaching straight for the
 * provider hook would race the bootstrap it just triggered.
 */
export async function reopenMobileAt(page: Page, baseUrl: string, route: string): Promise<void> {
  await page.goto(`${baseUrl}${withMobilePreview(route)}`);
  await waitForMobileApp(page);
}

/** Reveals the immersive reader chrome through its central tap gesture. */
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

/**
 * Simulates the device losing connectivity to the PlainShelf server.
 *
 * Deliberately NOT `context.setOffline(true)`: that also blocks the
 * top-level document/JS/CSS request itself, which fails a `reopenMobileAt`
 * full-reload navigation with net::ERR_INTERNET_DISCONNECTED (confirmed
 * empirically — Go's http.FileServerFS sends no Cache-Control/Last-Modified
 * for go:embed'd files, so Chromium cannot serve the shell from cache while
 * offline). That failure mode doesn't exist on the real Android shell, which
 * loads its JS/CSS/HTML from the installed app package, not over the
 * network — only requests to the remote server need connectivity. So this
 * only blocks `/api/**` (matching frontend/src/api/client.ts's buildApiUrl)
 * and forces `navigator.onLine` to false, which is what
 * MobileBookshelfProvider.isOnline() actually reads.
 */
export async function goOffline(page: Page): Promise<void> {
  await page.route('**/api/**', (route) => route.abort('internetdisconnected'));
  await page.addInitScript((isOnline) => {
    Object.defineProperty(window.navigator, 'onLine', { get: () => isOnline, configurable: true });
  }, false);
  await setNavigatorOnline(page, false);
}

/** Restores connectivity after {@link goOffline}. */
export async function goOnline(page: Page): Promise<void> {
  await page.unroute('**/api/**');
  await page.addInitScript((isOnline) => {
    Object.defineProperty(window.navigator, 'onLine', { get: () => isOnline, configurable: true });
  }, true);
  await setNavigatorOnline(page, true);
}

/** Looks up a book's id by title via the test hook (order-independent). */
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

/** Downloads a book for offline reading via the test hook (no UI entry point). */
export async function downloadBookViaHook(page: Page, bookId: string): Promise<void> {
  await page.evaluate(async (id) => {
    const provider = window.__plainshelfTestHooks?.provider;
    await provider?.downloadBook?.(id);
  }, bookId);
}

/** Removes a downloaded book via the test hook (no UI entry point). */
export async function removeDownloadViaHook(page: Page, bookId: string): Promise<void> {
  await page.evaluate(async (id) => {
    const provider = window.__plainshelfTestHooks?.provider;
    await provider?.removeDownload?.(id);
  }, bookId);
}

/** Reads a book's download state via the test hook (no UI entry point). */
export async function getDownloadStateViaHook(page: Page, bookId: string): Promise<string> {
  return page.evaluate(async (id) => {
    const provider = window.__plainshelfTestHooks?.provider;
    return (await provider?.getDownloadState?.(id)) ?? 'not_downloaded';
  }, bookId);
}

/** Reads back the persisted reading progress via the test hook (authoritative check, not UI scroll position). */
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

/**
 * Reads a book's `current_source` id via the test hook. Needed to exercise
 * getSourceContent for a book without depending on any particular cache's
 * internal key scheme.
 */
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

/**
 * Reads a book's content via the test hook (goes through the same
 * cache-then-remote path the reader uses), returning `null` instead of
 * throwing when unavailable (e.g. offline and not downloaded). This lets
 * tests assert both the "readable" and "evicted" cases uniformly, without
 * coupling to which offline cache backend is wired up (frontend/src/
 * providers/index.ts) or its internal storage structure.
 */
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

/** Same as {@link getBookContentViaHook}, for a single source's content. */
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

/**
 * Size accounting for one downloaded book, read via the provider's public
 * `listDownloadedBookEntries()` (the same API DownloadsPage renders). This
 * stays backend-agnostic: it asserts on the persisted `sizeBytes` the
 * provider reports, not on any particular cache's internal storage (the
 * production mobile cache is filesystem-backed, not `plainshelf-mobile` IDB).
 */
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
