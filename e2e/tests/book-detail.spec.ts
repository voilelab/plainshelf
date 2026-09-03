import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { chaptersMarkdownFixturePath, helloFixturePath, importBookAs, importBookFromPath } from './support/books';
import { openReaderTab } from './support/reader';

const getServer = useServer();

// Imports a plain-text book under a per-test title and opens its detail page.
// The shelf is shared across this file's tests, so each test passes its own
// name to avoid colliding with books left behind by earlier tests.
async function openBookDetail(
  page: import('@playwright/test').Page,
  baseUrl: string,
  title: string
): Promise<void> {
  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, title);
  await page.locator('.book-list-row').getByRole('heading', { name: title, exact: true }).click();
  await expect(page).toHaveURL(/\/books\/[^/?]+$/);
}

test('resets a completed bookmark before reading again', async ({ page }) => {
  const { baseUrl } = getServer();
  const markRequests: string[] = [];
  // The reader opens in its own tab on the web build, so watch the whole
  // context to catch any server mark write from either tab.
  page.context().on('request', (request) => {
    if (request.url().includes('/marks/')) markRequests.push(request.url());
  });

  await openBookDetail(page, baseUrl, 'detail-reset-bookmark');
  const bookID = new URL(page.url()).pathname.split('/').pop()!;
  await page.evaluate(
    ({ id }) => {
      localStorage.setItem(
        'plainshelf.readingProgress',
        JSON.stringify({
          version: 2,
          shelves: { default_shelf: { [id]: { offset: 74, at: Date.now() } } }
        })
      );
    },
    { id: bookID }
  );
  await page.reload();
  await expect(page.getByRole('button', { name: 'Read again' })).toBeVisible();

  // "Read again" resets the client-side progress and then opens the reader in
  // a new tab; the reset happens on this tab before the tab opens, and the
  // progress store is shared across tabs of the same origin.
  const reader = await openReaderTab(
    page,
    () => page.getByRole('button', { name: 'Read again' }).click(),
    /\/reader\/[^/?]+$/
  );

  await expect.poll(() => page.evaluate(
    ({ id }) => {
      const stored = JSON.parse(localStorage.getItem('plainshelf.readingProgress') ?? '{}');
      // A reset now leaves a timestamped tombstone (offset 0) rather than
      // deleting the entry, so read the offset rather than the whole entry.
      return stored.shelves?.default_shelf?.[id]?.offset ?? 0;
    },
    { id: bookID }
  )).toBe(0);
  // "Progress: 0%" is reader UI, now shown in the reader's own tab.
  await expect(reader.getByText('Progress: 0%', { exact: true })).toBeVisible();
  expect(markRequests).toEqual([]);
  await reader.close();
});

test('opens the reader at a chapter picked from the detail page', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  // Only this test imports the chapters fixture, so its "chapters" title stays
  // unique within the shared shelf.
  await importBookFromPath(page, chaptersMarkdownFixturePath);
  await page.locator('.book-list-row').getByRole('heading', { name: 'chapters', exact: true }).click();
  await expect(page).toHaveURL(/\/books\/[^/?]+$/);

  // The list is a deep link into the reader, so it has to show the reader's
  // own sections: the opening ahead of the first H2 included, in that order.
  const chapterCard = page.locator('.detail-card-chapters');
  // The chapter list is the way into the book rather than a fact about it, so
  // it reads after the cards describing the book.
  await expect(page.locator('.detail-sections > section').last()).toHaveClass(
    /detail-card-chapters/
  );
  await expect(chapterCard.locator('.chapter-item-title')).toHaveText([
    'Chapter Sampler',
    'First Light',
    'Second Wind',
    'Third Rail'
  ]);

  const reader = await openReaderTab(
    page,
    () => chapterCard.getByRole('button', { name: 'Second Wind' }).click(),
    /\/reader\/[^/?]+\?section=2$/
  );
  await expect(reader.locator('.reader-text')).toContainText('Text of the second chapter.');

  await reader.getByRole('button', { name: 'Show chapters' }).click();
  await expect(reader.locator('.chapter-modal-item.active')).toContainText('Second Wind');
});
