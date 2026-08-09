import { expect, type Page } from '@playwright/test';

// Mirrors frontend/src/providers/runtime.ts isMobileRuntime()'s desktop-browser
// escape hatch: appending this to a top-level goto makes the app boot with the
// mobile (Capacitor) provider on ordinary desktop Chromium, so the offline
// book cache + Preferences-backed connection config get exercised without an
// Android emulator.
export const MOBILE_PREVIEW_QUERY = 'mobile-shell-preview=1';

// Minimal shape of the hook attached by frontend/src/main.ts
// (window.__plainshelfTestHooks). Declared locally rather than imported from
// the frontend package: the e2e project has no path alias to frontend/src and
// intentionally stays a standalone TS project (see e2e/package.json).
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
 * Drives the mobile connect flow (`/connect`) end to end: fills in the server
 * URL (and optional token), loads the shelf list, picks a shelf from the
 * reka-ui Select, and saves — landing on `/books`.
 *
 * The token does not grant write access — the client is read-only regardless —
 * but server/security.go demands one for the allowlisted reading-telemetry
 * POSTs, so a native install still needs it.
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
}

/**
 * Standard "reopen the app" action: a top-level navigation (not an in-app
 * router push) carrying the mobile-shell-preview param, since the param is
 * dropped from the URL by in-app navigations (MobileConnectPage.onSave does a
 * bare `router.push('/books')`) and isMobileRuntime() reads location.search
 * live on every check.
 */
export async function reopenMobileAt(page: Page, baseUrl: string, route: string): Promise<void> {
  await page.goto(`${baseUrl}${withMobilePreview(route)}`);
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

/**
 * Simulates the device having network connectivity but being unable to
 * reach the self-hosted PlainShelf server — e.g. the phone is on LTE away
 * from the home LAN. Unlike {@link goOffline}, `navigator.onLine` is left
 * `true`: only requests to the server (`/api/**`) are aborted, exercising
 * MobileBookshelfProvider's network-first-with-cache-fallback path rather
 * than its isOnline()===false offline path.
 */
export async function goServerUnreachable(page: Page): Promise<void> {
  await page.route('**/api/**', (route) => route.abort('connectionrefused'));
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
