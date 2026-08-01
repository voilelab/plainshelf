import { expect, test, type Locator, type Page } from '@playwright/test';
import { anotherFixturePath, helloFixturePath, importBookFromPath } from './support/books';
import {
  connectMobile,
  getBookIdByTitle,
  getDownloadStateViaHook,
  reopenMobileAt
} from './support/mobile';
import { startServer } from './support/server';

async function longPress(page: Page, row: Locator): Promise<void> {
  await row.dispatchEvent('pointerdown', {
    pointerType: 'touch',
    pointerId: 1,
    clientX: 24,
    clientY: 24,
    isPrimary: true
  });
  await page.waitForTimeout(500);
  await row.dispatchEvent('pointerup', {
    pointerType: 'touch',
    pointerId: 1,
    clientX: 24,
    clientY: 24,
    isPrimary: true
  });
  // A real touch sequence emits a compatibility click after pointerup; the
  // interaction composable consumes it so the long-pressed item is not toggled twice.
  await row.click();
}

test('mobile long press selects, retries failed downloads, and skips downloaded books', async ({ page }) => {
  const server = await startServer();

  try {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`${server.baseUrl}/books`);
    await importBookFromPath(page, helloFixturePath);
    await importBookFromPath(page, anotherFixturePath);
    await connectMobile(page, server.baseUrl);
    await reopenMobileAt(page, server.baseUrl, '/books');

    const helloId = await getBookIdByTitle(page, 'hello');
    const anotherId = await getBookIdByTitle(page, 'another');
    const helloRow = page.locator('.book-list-row', { hasText: 'hello' });
    const anotherRow = page.locator('.book-list-row', { hasText: 'another' });
    const selectionToolbar = page.getByRole('toolbar', { name: 'Selected books actions' });

    // Moving beyond the touch tolerance cancels the pending long press.
    await helloRow.dispatchEvent('pointerdown', {
      pointerType: 'touch', pointerId: 1, clientX: 20, clientY: 20, isPrimary: true
    });
    await helloRow.dispatchEvent('pointermove', {
      pointerType: 'touch', pointerId: 1, clientX: 20, clientY: 45, isPrimary: true
    });
    await page.waitForTimeout(500);
    await helloRow.dispatchEvent('pointerup', {
      pointerType: 'touch', pointerId: 1, clientX: 20, clientY: 45, isPrimary: true
    });
    await expect(selectionToolbar).not.toBeVisible();

    await longPress(page, helloRow);
    await expect(selectionToolbar.getByText('1 selected')).toBeVisible();
    await anotherRow.click();
    await expect(selectionToolbar.getByText('2 selected')).toBeVisible();

    const mobileActionBar = page.locator('.mobile-selection-actions');
    await expect(mobileActionBar).toHaveCSS('position', 'fixed');

    // Fail one item locally; successful items clear while the failed item remains selected.
    await page.route(`**/books/${anotherId}/content`, (route) => route.abort('connectionrefused'));
    await mobileActionBar.getByRole('button', { name: 'Download to device' }).click();
    const downloadDialog = page.getByRole('dialog', { name: 'Download to device' });
    await expect(downloadDialog.getByText('1 downloaded; 1 failed.')).toBeVisible();
    await expect(downloadDialog.getByText('another', { exact: true })).toBeVisible();
    expect(await getDownloadStateViaHook(page, helloId)).toBe('downloaded');
    expect(await getDownloadStateViaHook(page, anotherId)).not.toBe('downloaded');

    await page.unroute(`**/books/${anotherId}/content`);
    await downloadDialog.getByRole('button', { name: 'Close' }).click();
    await expect(selectionToolbar.getByText('1 selected')).toBeVisible();
    await mobileActionBar.getByRole('button', { name: 'Download to device' }).click();
    await expect(downloadDialog.getByText('1 books downloaded.')).toBeVisible();
    await downloadDialog.getByRole('button', { name: 'Close' }).click();
    await expect(selectionToolbar).not.toBeVisible();
    expect(await getDownloadStateViaHook(page, anotherId)).toBe('downloaded');

    // Selecting already-current downloads counts as success without invoking downloadBook again.
    await page.evaluate(() => {
      const provider = window.__plainshelfTestHooks?.provider;
      if (!provider?.downloadBook) throw new Error('mobile download provider is unavailable');
      const original = provider.downloadBook.bind(provider);
      (window as unknown as { __batchDownloadCalls: string[] }).__batchDownloadCalls = [];
      provider.downloadBook = async (id: string) => {
        (window as unknown as { __batchDownloadCalls: string[] }).__batchDownloadCalls.push(id);
        await original(id);
      };
    });
    await longPress(page, helloRow);
    await anotherRow.click();
    await mobileActionBar.getByRole('button', { name: 'Download to device' }).click();
    await expect(downloadDialog.getByText('2 books downloaded.')).toBeVisible();
    expect(await page.evaluate(() => (window as unknown as { __batchDownloadCalls: string[] }).__batchDownloadCalls)).toEqual([]);
    await downloadDialog.getByRole('button', { name: 'Close' }).click();

    // The Android back listener consumes selection before routing backward.
    await longPress(page, helloRow);
    const urlBeforeBack = page.url();
    const notified = await page.evaluate(async () => {
      const app = (window as unknown as { Capacitor?: { Plugins?: { App?: { notifyListeners?: (name: string, data: unknown) => Promise<void> } } } }).Capacitor?.Plugins?.App;
      if (!app?.notifyListeners) return false;
      await app.notifyListeners('backButton', { canGoBack: true });
      return true;
    });
    expect(notified).toBe(true);
    await expect(selectionToolbar).not.toBeVisible();
    expect(page.url()).toBe(urlBeforeBack);
  } finally {
    await server.dispose();
  }
});
