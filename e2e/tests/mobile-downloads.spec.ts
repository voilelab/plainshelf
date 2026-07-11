import { expect, test, type Page } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook, createCoverDataTransfer } from './support/books';
import {
  connectMobile,
  reopenMobileAt,
  getBookIdByTitle,
  downloadBookViaHook,
  dumpMobileStores,
  readManifestFromIdb,
  goOffline
} from './support/mobile';

// These tests cover the "saved books" UI that sits on top of the offline
// download data layer exercised by mobile-storage.spec.ts: the mobile-only
// "Download to device" button on BookDetailPage (useOfflineDownload), the
// /downloads management page (DownloadsPage.vue), and offline cover display
// via provider-rewritten blob: URLs. They run desktop Chromium with
// `?mobile-shell-preview=1` — same approach as mobile-storage.spec.ts.

/**
 * Uploads a cover for the book currently open on the (desktop-mode) detail
 * page via drag-and-drop — same flow as import-book.spec.ts. Covers must be
 * set in desktop mode: mobile-preview mode cannot POST without a token.
 */
async function uploadCoverOnDetailPage(page: Page): Promise<void> {
  const coverTarget = page.locator('.cover-drop-target');
  await expect(coverTarget).toBeVisible();

  const dataTransfer = await createCoverDataTransfer(page);
  try {
    await coverTarget.dispatchEvent('dragenter', { dataTransfer });
    await expect(page.getByText('Drop image to update cover')).toBeVisible();
    await coverTarget.dispatchEvent('dragover', { dataTransfer });
    await coverTarget.dispatchEvent('drop', { dataTransfer });
  } finally {
    await dataTransfer.dispose();
  }

  const confirmDialog = page.getByRole('dialog', { name: 'Update book cover?' });
  await expect(confirmDialog).toBeVisible();
  await confirmDialog.getByRole('button', { name: 'Update cover' }).click();
  await expect(confirmDialog).not.toBeVisible();
  await expect(page.getByRole('button', { name: 'Remove' })).toBeEnabled();
}

test('downloads a book to the device from the detail page button', async ({ page }) => {
  const server = await startServer();

  try {
    // Import and set a cover in the ordinary desktop flow first (mobile mode
    // cannot POST without a token — see mobile-storage.spec.ts).
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await uploadCoverOnDetailPage(page);

    await connectMobile(page, server.baseUrl);
    const helloId = await getBookIdByTitle(page, 'hello');

    // Open the detail page via a top-level navigation: an in-app click would
    // drop the ?mobile-shell-preview=1 param, and BookDetailPage gates the
    // offline-download button on a live isMobileRuntime() check.
    await reopenMobileAt(page, server.baseUrl, `/books/${helloId}`);

    const downloadButton = page.getByRole('button', { name: 'Download to device' });
    await expect(downloadButton).toBeVisible();
    await downloadButton.click();

    // The same button flips to the remove affordance once the download lands.
    await expect(page.getByRole('button', { name: 'Remove download' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Download to device' })).not.toBeVisible();

    // Authoritative check on the cache itself, not just the button label:
    // manifest persisted with non-zero size accounting, and the cover blob
    // cached in the v2 `covers` store.
    const stores = await dumpMobileStores(page);
    expect(stores.manifests).toContain(helloId);
    expect(stores.bookContents).toContain(helloId);
    expect(stores.covers).toContain(helloId);

    const manifest = await readManifestFromIdb(page, helloId);
    expect(manifest).not.toBeNull();
    expect(manifest?.size_bytes).toBeGreaterThan(0);
    expect(manifest?.size_breakdown?.content).toBeGreaterThan(0);
    expect(manifest?.size_breakdown?.cover).toBeGreaterThan(0);
  } finally {
    await server.dispose();
  }
});

test('lists, sizes, and removes downloads on the /downloads page', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    await connectMobile(page, server.baseUrl);
    const helloId = await getBookIdByTitle(page, 'hello');
    await downloadBookViaHook(page, helloId);

    await reopenMobileAt(page, server.baseUrl, '/downloads');
    await expect(page.getByRole('heading', { name: 'Downloaded Books' })).toBeVisible();

    // Overview panel: one book downloaded, non-zero total size.
    const overview = page.locator('.downloads-overview');
    await expect(overview.getByText('1 downloaded')).toBeVisible();
    const totalSizeValue = overview
      .locator('.downloads-overview-row', { hasText: 'Total size' })
      .locator('.downloads-overview-value');
    await expect(totalSizeValue).not.toHaveText('0 B');

    // List row: the book with a non-zero per-book size and a remove button.
    const row = page.locator('.downloads-list .book-list-row', { hasText: 'hello' });
    await expect(row.getByRole('heading', { name: 'hello', exact: true })).toBeVisible();
    await expect(row.locator('.downloads-size')).not.toHaveText('0 B');

    // Removal goes through the DeleteModal confirmation.
    await row.getByRole('button', { name: 'Remove download' }).click();
    const removeDialog = page.getByRole('dialog', { name: 'Remove download' });
    await expect(removeDialog).toBeVisible();
    await expect(removeDialog.getByText('Delete hello?')).toBeVisible();
    await removeDialog.getByRole('button', { name: 'Remove download' }).click();
    await expect(removeDialog).not.toBeVisible();

    // Row disappears and the overview zeroes out.
    await expect(page.getByText('No books downloaded yet.')).toBeVisible();
    await expect(page.locator('.downloads-list .book-list-row')).toHaveCount(0);
    await expect(overview.getByText('0 downloaded')).toBeVisible();
    await expect(totalSizeValue).toHaveText('0 B');

    // The cache is actually empty, not just hidden from the UI.
    const stores = await dumpMobileStores(page);
    expect(stores.manifests).not.toContain(helloId);
    expect(stores.bookContents).not.toContain(helloId);
    expect(stores.covers).not.toContain(helloId);
  } finally {
    await server.dispose();
  }
});

test('shows the cached cover from a blob: URL in the library while offline', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await uploadCoverOnDetailPage(page);

    await connectMobile(page, server.baseUrl);
    const helloId = await getBookIdByTitle(page, 'hello');
    await downloadBookViaHook(page, helloId);

    // Cover blob must be in the cache before going offline.
    const stores = await dumpMobileStores(page);
    expect(stores.covers).toContain(helloId);

    await goOffline(page);
    await reopenMobileAt(page, server.baseUrl, '/books');

    // The provider rewrites cover_url to an object URL for the cached blob
    // while offline, so the library's <img> must resolve locally instead of
    // pointing at the unreachable server.
    const row = page.locator('.book-list-row', { hasText: 'hello' });
    await expect(row.getByRole('heading', { name: 'hello', exact: true })).toBeVisible();
    await expect(row.locator('.book-list-cover')).toHaveAttribute('src', /^blob:/);
  } finally {
    await server.dispose();
  }
});
