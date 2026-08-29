import { expect, test } from '@playwright/test';
import { helloFixturePath, importBookAs } from './support/books';
import { addFolder, selectFolder } from './support/folders';
import { connectMobile, reopenMobileAt } from './support/mobile';
import { useServer } from './support/server';

const getServer = useServer();

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
