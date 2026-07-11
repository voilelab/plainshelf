import { expect, type Page } from '@playwright/test';

// Mirrors frontend/src/providers/runtime.ts isMobileRuntime()'s desktop-browser
// escape hatch: appending this to a top-level goto makes the app boot with the
// mobile (Capacitor) provider on ordinary desktop Chromium, so the IndexedDB
// offline cache + Preferences-backed connection config get exercised without
// an Android emulator.
export const MOBILE_PREVIEW_QUERY = 'mobile-shell-preview=1';

// Minimal shape of the hook attached by frontend/src/main.ts
// (window.__plainshelfTestHooks). Declared locally rather than imported from
// the frontend package: the e2e project has no path alias to frontend/src and
// intentionally stays a standalone TS project (see e2e/package.json).
declare global {
  interface Window {
    __plainshelfTestHooks?: {
      provider: {
        listBooks: (
          page?: number,
          pageSize?: number,
          search?: string
        ) => Promise<{ items: Array<{ id: string; title: string }> }>;
        downloadBook?: (bookId: string) => Promise<void>;
        removeDownload?: (bookId: string) => Promise<void>;
        getDownloadState?: (bookId: string) => Promise<string>;
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

export interface MobileStoreDump {
  manifests: string[];
  bookContents: string[];
  sourceContents: string[];
  progress: string[];
  covers: string[];
}

/**
 * Reads the raw key list of all 5 IndexedDB object stores in the
 * `plainshelf-mobile` database (see frontend/src/providers/indexedDbMobileBookCache.ts,
 * schema v2: v1's 4 stores plus `covers`).
 * Uses raw indexedDB APIs rather than the provider hook so tests can assert
 * on cache structure (e.g. the `${bookId}::${sourceId}` key prefix used for
 * isolating removal) independently of provider behavior.
 */
export async function dumpMobileStores(page: Page): Promise<MobileStoreDump> {
  return page.evaluate(() => {
    const storeNames = ['manifests', 'bookContents', 'sourceContents', 'progress', 'covers'] as const;

    return new Promise<MobileStoreDump>((resolve, reject) => {
      // Version must match indexedDbMobileBookCache.ts's DB_VERSION (2); the
      // upgrade handler idempotently creates stores so this also works if
      // called before the app has ever touched the cache. (It does NOT
      // replicate the app's v1→v2 size backfill — these tests always start
      // from a fresh browser context, never from v1 data.)
      const request = indexedDB.open('plainshelf-mobile', 2);

      request.onupgradeneeded = () => {
        const db = request.result;
        for (const name of storeNames) {
          if (!db.objectStoreNames.contains(name)) {
            db.createObjectStore(name);
          }
        }
      };

      request.onerror = () => reject(request.error);
      request.onsuccess = () => {
        const db = request.result;
        const tx = db.transaction(storeNames, 'readonly');
        const result = {} as Record<(typeof storeNames)[number], string[]>;
        let remaining = storeNames.length;

        for (const name of storeNames) {
          const keysRequest = tx.objectStore(name).getAllKeys();
          keysRequest.onsuccess = () => {
            result[name] = keysRequest.result.map((key) => String(key));
            remaining -= 1;
            if (remaining === 0) {
              db.close();
              resolve(result as MobileStoreDump);
            }
          };
          keysRequest.onerror = () => reject(keysRequest.error);
        }
      };
    });
  });
}

/**
 * Shape of the size-accounting fields on a cached book manifest
 * (frontend/src/providers/mobileBookCache.ts CachedBookManifest, v2 schema).
 */
export interface MobileManifestSizeInfo {
  size_bytes?: number;
  size_breakdown?: { content: number; sources: number; cover: number };
  downloaded_at?: string;
}

/**
 * Reads a single manifest record straight from the `manifests` object store,
 * so tests can assert on persisted size accounting (size_bytes/size_breakdown)
 * rather than trusting what the UI displays. Returns null when the book has
 * no manifest (not downloaded).
 */
export async function readManifestFromIdb(
  page: Page,
  bookId: string
): Promise<MobileManifestSizeInfo | null> {
  return page.evaluate((id) => {
    return new Promise<MobileManifestSizeInfo | null>((resolve, reject) => {
      const request = indexedDB.open('plainshelf-mobile', 2);
      request.onerror = () => reject(request.error);
      request.onupgradeneeded = () => {
        // Fresh DB: the app has not downloaded anything yet, so create the
        // stores (mirroring dumpMobileStores) and report "no manifest".
        const db = request.result;
        for (const name of ['manifests', 'bookContents', 'sourceContents', 'progress', 'covers']) {
          if (!db.objectStoreNames.contains(name)) {
            db.createObjectStore(name);
          }
        }
      };
      request.onsuccess = () => {
        const db = request.result;
        const getRequest = db.transaction('manifests', 'readonly').objectStore('manifests').get(id);
        getRequest.onerror = () => reject(getRequest.error);
        getRequest.onsuccess = () => {
          const manifest = getRequest.result as
            | { size_bytes?: number; size_breakdown?: { content: number; sources: number; cover: number }; downloaded_at?: string }
            | undefined;
          db.close();
          resolve(
            manifest
              ? {
                  size_bytes: manifest.size_bytes,
                  size_breakdown: manifest.size_breakdown,
                  downloaded_at: manifest.downloaded_at
                }
              : null
          );
        };
      };
    });
  }, bookId);
}
