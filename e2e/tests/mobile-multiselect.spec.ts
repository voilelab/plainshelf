import { expect, test, type Locator, type Page } from '@playwright/test';
import { helloFixturePath, importBookAs } from './support/books';
import { addFolder, selectFolder } from './support/folders';
import { connectMobile, reopenMobileAt } from './support/mobile';
import { useServer } from './support/server';

const getServer = useServer();

async function longPressWithCompatibilityClick(page: Page, row: Locator): Promise<void> {
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
  await row.click();
}

test('mobile library marks download state on every row and refreshes it after a batch download', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'download-state-hello');

  const webRow = page.locator('.book-list-row', { hasText: 'download-state-hello' });
  await expect(webRow).toBeVisible();
  await expect(webRow.locator('.book-download-badge')).toHaveCount(0);

  await connectMobile(page, baseUrl);
  await reopenMobileAt(page, baseUrl, '/books');

  const row = page.locator('.book-list-row', { hasText: 'download-state-hello' });
  const badge = row.locator('.book-download-badge');
  await expect(badge).toHaveText('Not downloaded');

  await longPressWithCompatibilityClick(page, row);
  await page
    .locator('.mobile-selection-actions')
    .getByRole('button', { name: 'Download to device' })
    .click();
  const downloadDialog = page.getByRole('dialog', { name: 'Download to device' });
  await expect(downloadDialog.getByText('1 books downloaded.')).toBeVisible();
  await downloadDialog.getByRole('button', { name: 'Close' }).click();

  await expect(badge).toHaveText('Downloaded');
});

test('a deep folder path does not push the download badge past the row edge', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await addFolder(page, 'badgelayout/a-rather-long-nested-folder-name');
  await selectFolder(page, 'a-rather-long-nested-folder-name');
  await importBookAs(page, helloFixturePath, 'badge-layout-book');

  await page.setViewportSize({ width: 390, height: 844 });
  await connectMobile(page, baseUrl);
  await reopenMobileAt(page, baseUrl, '/books');

  const row = page.locator('.book-list-row', { hasText: 'badge-layout-book' });
  const badge = row.locator('.book-download-badge');
  await expect(badge).toHaveText('Not downloaded');

  const layout = await row.evaluate((element) => {
    const folder = element.querySelector('.book-list-folder') as HTMLElement;
    const badgeElement = element.querySelector('.book-download-badge') as HTMLElement;
    return {
      overflow: element.scrollWidth - element.clientWidth,
      folderOverhang: folder.getBoundingClientRect().right - element.getBoundingClientRect().right,
      badgeClipped: badgeElement.scrollWidth - badgeElement.clientWidth
    };
  });

  expect(layout.overflow).toBeLessThanOrEqual(0);
  expect(layout.folderOverhang).toBeLessThanOrEqual(0);
  expect(layout.badgeClipped).toBeLessThanOrEqual(0);
});
