import { expect, test, type Locator, type Page } from '@playwright/test';
import { helloFixturePath, importBookAs } from './support/books';
import { addFolder, selectFolder } from './support/folders';
import { connectMobile, reopenMobileAt } from './support/mobile';
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

// At the narrow breakpoint the download badge and the folder label share one
// line. The badge is the fixed part — a state the user cannot read is no better
// than no badge — so the folder path is what has to give way; when it could not,
// a deeply nested book pushed its whole row past the card's right edge.
//
// jsdom has no layout, so only a real browser can see this.
test('a deep folder path does not push the download badge past the row edge', async ({ page }) => {
  const { baseUrl } = getServer();

  // The folder tree lives in the sidebar, which a narrow viewport folds into a
  // drawer, so the book is filed at desktop width and only then read on a phone.
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
      // The badge label is never the thing that gets clipped.
      badgeClipped: badgeElement.scrollWidth - badgeElement.clientWidth
    };
  });

  expect(layout.overflow).toBeLessThanOrEqual(0);
  expect(layout.folderOverhang).toBeLessThanOrEqual(0);
  expect(layout.badgeClipped).toBeLessThanOrEqual(0);
});
