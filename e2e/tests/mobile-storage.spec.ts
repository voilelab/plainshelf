import { expect, test, type Page } from '@playwright/test';
import { useServer } from './support/server';
import { importBookAs, helloFixturePath, anotherFixturePath } from './support/books';
import {
  connectMobile,
  reopenMobileAt,
  getBookIdByTitle,
  downloadBookViaHook,
  removeDownloadViaHook,
  getDownloadStateViaHook,
  getReadProgressViaHook,
  getCurrentSourceIdViaHook,
  getBookContentViaHook,
  getSourceContentViaHook,
  goOffline,
  goOnline,
  showMobileReaderControls
} from './support/mobile';


const getServer = useServer();

async function scrollReaderToMiddle(page: Page): Promise<void> {
  await page.locator('.reader-content').evaluate((element) => {
    Object.defineProperty(element, 'scrollHeight', { configurable: true, value: 1_000 });
    Object.defineProperty(element, 'clientHeight', { configurable: true, value: 100 });
    Object.defineProperty(element, 'scrollTop', { configurable: true, value: 450, writable: true });
    element.dispatchEvent(new Event('scroll'));
  });
}

test('downloads books for offline reading and isolates removal between books', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'storage-offline-hello');
  await importBookAs(page, anotherFixturePath, 'storage-offline-another');

  await connectMobile(page, baseUrl);

  const helloId = await getBookIdByTitle(page, 'storage-offline-hello');
  const anotherId = await getBookIdByTitle(page, 'storage-offline-another');
  expect(helloId).not.toBe(anotherId);

  await downloadBookViaHook(page, helloId);
  await downloadBookViaHook(page, anotherId);

  expect(await getDownloadStateViaHook(page, helloId)).toBe('downloaded');
  expect(await getDownloadStateViaHook(page, anotherId)).toBe('downloaded');

  const helloSourceId = await getCurrentSourceIdViaHook(page, helloId);
  const anotherSourceId = await getCurrentSourceIdViaHook(page, anotherId);

  await goOffline(page);
  await reopenMobileAt(page, baseUrl, '/books');
  const helloRow = page
    .locator('.book-list-row')
    .getByRole('heading', { name: 'storage-offline-hello', exact: true });
  await expect(helloRow).toBeVisible();
  await helloRow.click();
  await expect(page).toHaveURL(/\/books\/[^/]+$/);
  await expect(page.getByRole('button', { name: 'Start reading' })).toBeVisible();
  await reopenMobileAt(page, baseUrl, `/reader/${helloId}`);
  await expect(page.getByText('Hello from PlainShelf E2E.')).toBeVisible();

  await goOnline(page);
  await removeDownloadViaHook(page, helloId);

  await goOffline(page);

  expect(await getDownloadStateViaHook(page, helloId)).toBe('not_downloaded');
  expect(await getDownloadStateViaHook(page, anotherId)).toBe('downloaded');

  expect(await getBookContentViaHook(page, helloId)).toBeNull();
  expect(await getSourceContentViaHook(page, helloId, helloSourceId)).toBeNull();

  expect(await getBookContentViaHook(page, anotherId)).toContain('Another book for PlainShelf E2E.');
  expect(await getSourceContentViaHook(page, anotherId, anotherSourceId)).toContain(
    'Another book for PlainShelf E2E.'
  );
});

test('automatically persists reading progress across app restarts', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'storage-progress');

  await connectMobile(page, baseUrl);
  const helloId = await getBookIdByTitle(page, 'storage-progress');
  await downloadBookViaHook(page, helloId);

  await reopenMobileAt(page, baseUrl, `/reader/${helloId}`);
  await expect(page.getByText('Hello from PlainShelf E2E.')).toBeVisible();

  await showMobileReaderControls(page);
  await expect(page.getByRole('button', { name: /bookmark/i })).toHaveCount(0);

  await scrollReaderToMiddle(page);
  await page.getByRole('button', { name: 'Back' }).click();
  await expect(page).toHaveURL(new RegExp(`/books/${helloId}$`));

  const savedProgress = await getReadProgressViaHook(page, helloId);
  expect(savedProgress.char_offset).toBeGreaterThan(0);

  await goOffline(page);
  await reopenMobileAt(page, baseUrl, `/reader/${helloId}`);
  await expect(page.getByText('Hello from PlainShelf E2E.')).toBeVisible();

  const reloadedProgress = await getReadProgressViaHook(page, helloId);
  expect(reloadedProgress.char_offset).toBe(savedProgress.char_offset);
});
