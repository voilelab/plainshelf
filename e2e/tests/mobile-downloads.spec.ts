import { expect, test, type Page } from '@playwright/test';
import { useServer } from './support/server';
import { importBookAs, helloFixturePath, createCoverDataTransfer } from './support/books';
import {
  connectMobile,
  reopenMobileAt,
  getBookIdByTitle,
  downloadBookViaHook,
  getDownloadStateViaHook,
  getDownloadedEntryViaHook,
  goOffline
} from './support/mobile';


const getServer = useServer();

async function uploadCoverOnDetailPage(page: Page): Promise<void> {
  const coverTarget = page.locator('.cover-drop-target');
  await expect(coverTarget).toBeVisible();

  const dataTransfer = await createCoverDataTransfer(page);
  try {
    await coverTarget.dispatchEvent('dragenter', { dataTransfer });
    await expect(page.getByText('Drop the image to update the cover')).toBeVisible();
    await coverTarget.dispatchEvent('dragover', { dataTransfer });
    await coverTarget.dispatchEvent('drop', { dataTransfer });
  } finally {
    await dataTransfer.dispose();
  }

  const confirmDialog = page.getByRole('dialog', { name: 'Update book cover?' });
  await expect(confirmDialog).toBeVisible();
  await confirmDialog.getByRole('button', { name: 'Update cover' }).click();
  await expect(confirmDialog).not.toBeVisible();
  await page.getByRole('button', { name: 'Cover options' }).click();
  await expect(page.getByRole('button', { name: 'Remove' })).toBeEnabled();
}

test('lists, sizes, and removes downloads on the /downloads page', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'downloads-page-book');

  await connectMobile(page, baseUrl);
  const helloId = await getBookIdByTitle(page, 'downloads-page-book');
  await downloadBookViaHook(page, helloId);

  await reopenMobileAt(page, baseUrl, '/downloads');
  await expect(page.getByRole('heading', { name: 'Downloaded Books' })).toBeVisible();

  const overview = page.locator('.downloads-overview');
  await expect(overview.getByText('1 downloaded')).toBeVisible();
  const totalSizeValue = overview
    .locator('.downloads-overview-row', { hasText: 'Total size' })
    .locator('.downloads-overview-value');
  await expect(totalSizeValue).not.toHaveText('0 B');

  const row = page.locator('.downloads-list .book-list-row', { hasText: 'downloads-page-book' });
  await expect(row.getByRole('heading', { name: 'downloads-page-book', exact: true })).toBeVisible();
  await expect(row.locator('.downloads-size')).not.toHaveText('0 B');

  await row.getByRole('button', { name: 'Remove download' }).click();
  const removeDialog = page.getByRole('alertdialog', { name: 'Remove download' });
  await expect(removeDialog).toBeVisible();
  await expect(removeDialog.getByText('Delete downloads-page-book?')).toBeVisible();
  await removeDialog.getByRole('button', { name: 'Remove download' }).click();
  await expect(removeDialog).not.toBeVisible();

  await expect(page.getByText('No books downloaded yet.')).toBeVisible();
  await expect(page.locator('.downloads-list .book-list-row')).toHaveCount(0);
  await expect(overview.getByText('0 downloaded')).toBeVisible();
  await expect(totalSizeValue).toHaveText('0 B');

  expect(await getDownloadStateViaHook(page, helloId)).toBe('not_downloaded');
  expect(await getDownloadedEntryViaHook(page, helloId)).toBeNull();
});

test('shows the cached cover from a blob: URL in the library while offline', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'downloads-cover-book');
  await page
    .locator('.book-list-row')
    .getByRole('heading', { name: 'downloads-cover-book', exact: true })
    .click();
  await expect(page).toHaveURL(/\/books\/[^/]+$/);
  await uploadCoverOnDetailPage(page);

  await connectMobile(page, baseUrl);
  const helloId = await getBookIdByTitle(page, 'downloads-cover-book');
  await downloadBookViaHook(page, helloId);
  expect(await getDownloadStateViaHook(page, helloId)).toBe('downloaded');

  await goOffline(page);
  await reopenMobileAt(page, baseUrl, '/books');

  const row = page.locator('.book-list-row', { hasText: 'downloads-cover-book' });
  await expect(row.getByRole('heading', { name: 'downloads-cover-book', exact: true })).toBeVisible();
  await expect(row.locator('.book-list-cover')).toHaveAttribute('src', /^blob:/);
});
