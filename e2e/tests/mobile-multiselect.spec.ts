import { expect, test, type Locator, type Page } from '@playwright/test';
import { anotherFixturePath, helloFixturePath, importBookAs } from './support/books';
import {
  connectMobile,
  getBookIdByTitle,
  getDownloadStateViaHook,
  reopenMobileAt
} from './support/mobile';
import { useServer } from './support/server';

const getServer = useServer();

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
  const { baseUrl } = getServer();

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'multiselect-hello');
  await importBookAs(page, anotherFixturePath, 'multiselect-another');
  await connectMobile(page, baseUrl);
  await reopenMobileAt(page, baseUrl, '/books');

  const helloId = await getBookIdByTitle(page, 'multiselect-hello');
  const anotherId = await getBookIdByTitle(page, 'multiselect-another');
  const helloRow = page.locator('.book-list-row', { hasText: 'multiselect-hello' });
  const anotherRow = page.locator('.book-list-row', { hasText: 'multiselect-another' });
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
  await expect(downloadDialog.getByText('multiselect-another', { exact: true })).toBeVisible();
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
});

// Before the badge, the list gave no sign of which books were downloaded: the
// only way to find out was to open one and be redirected back by the reader
// gate. The state has to survive a batch download too, without a reload.
test('mobile library marks download state on every row and refreshes it after a batch download', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'download-state-hello');

  // The server has no download concept, so its rows must not gain an empty marker.
  const webRow = page.locator('.book-list-row', { hasText: 'download-state-hello' });
  await expect(webRow).toBeVisible();
  await expect(webRow.locator('.book-download-badge')).toHaveCount(0);

  await connectMobile(page, baseUrl);
  await reopenMobileAt(page, baseUrl, '/books');

  const row = page.locator('.book-list-row', { hasText: 'download-state-hello' });
  const badge = row.locator('.book-download-badge');
  await expect(badge).toHaveText('Not downloaded');

  await longPress(page, row);
  await page
    .locator('.mobile-selection-actions')
    .getByRole('button', { name: 'Download to device' })
    .click();
  const downloadDialog = page.getByRole('dialog', { name: 'Download to device' });
  await expect(downloadDialog.getByText('1 books downloaded.')).toBeVisible();
  await downloadDialog.getByRole('button', { name: 'Close' }).click();

  await expect(badge).toHaveText('Downloaded');
});

// The download bar used to be styled only inside a max-width:760px media query,
// so on a tablet-sized mobile shell multi-select opened a toolbar whose every
// action had been removed — Move and Trash are write surfaces the mobile client
// never offers, and Download was hidden by width. There was no way out but to
// cancel.
test('mobile download action stays reachable on a tablet-width screen', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.setViewportSize({ width: 1024, height: 768 });
  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'multiselect-tablet-hello');
  await connectMobile(page, baseUrl);
  await reopenMobileAt(page, baseUrl, '/books');

  const helloId = await getBookIdByTitle(page, 'multiselect-tablet-hello');
  const helloRow = page.locator('.book-list-row', { hasText: 'multiselect-tablet-hello' });

  await longPress(page, helloRow);
  await expect(
    page.getByRole('toolbar', { name: 'Selected books actions' }).getByText('1 selected')
  ).toBeVisible();

  const downloadBar = page.getByRole('toolbar', { name: 'Selected books download bar' });
  await expect(downloadBar).toBeVisible();

  const downloadButton = downloadBar.getByRole('button', { name: 'Download to device' });
  await expect(downloadButton).toBeVisible();
  await downloadButton.click();

  const downloadDialog = page.getByRole('dialog', { name: 'Download to device' });
  await expect(downloadDialog.getByText('1 books downloaded.')).toBeVisible();
  await downloadDialog.getByRole('button', { name: 'Close' }).click();
  expect(await getDownloadStateViaHook(page, helloId)).toBe('downloaded');
});
