import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import {
  chaptersMarkdownFixturePath,
  helloFixturePath,
  importBookAs,
  importBookFromPath
} from './support/books';
import { addFolder, foldersQueryRegex, selectAllBooks } from './support/folders';
import { useLocale } from './support/locale';
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

test('keeps reading progress and the primary action in the desktop first viewport', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.setViewportSize({ width: 1280, height: 720 });
  await openBookDetail(page, baseUrl, 'detail-desktop-fold');

  await expect(page.getByRole('heading', { name: 'detail-desktop-fold', exact: true })).toBeInViewport();
  await expect(page.getByRole('button', { name: 'Start reading' })).toBeInViewport();
  await expect(page.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0');
  await expect(page.getByText('Not started', { exact: true })).toBeVisible();

  const uploadButton = page.getByRole('button', { name: 'Upload', exact: true });
  await expect(uploadButton).not.toBeVisible();
  await page.getByRole('button', { name: 'Cover options' }).click();
  await expect(uploadButton).toBeVisible();

  await page.getByRole('button', { name: 'More' }).click();
  await expect(page.getByRole('menuitem', { name: 'Edit book details' })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: 'Manage sources' })).toBeVisible();
  await page.getByRole('menuitem', { name: 'Move to Trash' }).click();

  const dialog = page.getByRole('alertdialog');
  await expect(dialog).toContainText('The book will be moved to Trash. You can restore it later.');
  await dialog.getByRole('button', { name: 'Cancel' }).click();
  await expect(dialog).not.toBeVisible();
});

test('keeps the summary and reading action above the fold on a narrow viewport', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.setViewportSize({ width: 390, height: 844 });
  await openBookDetail(page, baseUrl, 'detail-narrow-fold');

  await expect(page.getByRole('heading', { name: 'detail-narrow-fold', exact: true })).toBeInViewport();
  await expect(page.getByRole('button', { name: 'Start reading' })).toBeInViewport();
  await expect(page.getByRole('button', { name: 'Export file' })).toBeInViewport();

  // The language switcher no longer lives in the narrow top bar — it moved to
  // Settings — so drive the locale the way a returning zh-Hant user boots
  // (seed storage, then reload the same detail URL) rather than a control this
  // viewport no longer shows. The summary and reading action must clear the
  // fold in that locale too.
  await useLocale(page, 'zh-Hant');
  await page.reload();

  await expect(page.getByRole('button', { name: '開始閱讀' })).toBeInViewport();
  await expect(page.getByText('尚未開始', { exact: true })).toBeVisible();
});

test('derives reading progress using the same UTF-16 units as the reader', async ({ page }) => {
  const { baseUrl } = getServer();
  const markRequests: string[] = [];
  page.on('request', (request) => {
    if (request.url().includes('/marks/')) markRequests.push(request.url());
  });

  // The detail page reads the current source, the same text the reader opens,
  // and falls back to the book-scoped route only when that source is gone.
  const serveEmoji = async (route: import('@playwright/test').Route) => {
    await route.fulfill({
      status: 200,
      contentType: 'text/plain; charset=utf-8',
      body: '🍥🍥'
    });
  };
  await page.route('**/books/*/content', serveEmoji);
  await page.route('**/books/*/sources/*/content', serveEmoji);
  await openBookDetail(page, baseUrl, 'detail-utf16');
  const bookID = new URL(page.url()).pathname.split('/').pop()!;
  await page.evaluate(
    ({ id }) => {
      localStorage.setItem(
        'plainshelf.readingProgress',
        JSON.stringify({
          version: 2,
          shelves: { default_shelf: { [id]: { offset: 2, at: Date.now() } } }
        })
      );
    },
    { id: bookID }
  );
  await page.reload();

  await expect(page.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '50');
  await expect(page.getByRole('button', { name: 'Continue reading · 50%' })).toBeVisible();
  await expect(page.getByText('Continue where you left off', { exact: true })).toBeVisible();
  expect(markRequests).toEqual([]);
});

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

test('moves the book to another folder from the More menu', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'detail-move-book');
  await addFolder(page, 'detail-move-folder');
  await selectAllBooks(page);
  await page.locator('.book-list-row').getByRole('heading', { name: 'detail-move-book', exact: true }).click();
  await expect(page).toHaveURL(/\/books\/[^/?]+$/);

  await page.getByRole('button', { name: 'More' }).click();
  await page.getByRole('menuitem', { name: 'Move to…', exact: true }).click();

  const moveDialog = page.getByRole('dialog', { name: 'Move book' });
  await moveDialog.getByLabel('Destination').selectOption('detail-move-folder');

  // A failed move has to report inside the dialog: the overlay hides the
  // page's own error banner. The dialog stays open with the destination
  // still picked, so retrying is one more click.
  await page.route('**/books/*', async (route) => {
    if (route.request().method() !== 'PATCH') {
      await route.continue();
      return;
    }
    await route.fulfill({ status: 500, contentType: 'text/plain', body: 'move failed' });
  });
  await moveDialog.getByRole('button', { name: 'Move 1 book', exact: true }).click();
  await expect(moveDialog.getByRole('alert')).toHaveText('move failed');
  await page.unroute('**/books/*');

  await moveDialog.getByRole('button', { name: 'Move 1 book', exact: true }).click();
  await expect(moveDialog).not.toBeVisible();

  const breadcrumb = page.getByRole('navigation', { name: 'Book folder path' });
  await expect(breadcrumb.getByRole('link', { name: 'detail-move-folder', exact: true })).toBeVisible();

  await breadcrumb.getByRole('link', { name: 'detail-move-folder', exact: true }).click();
  await expect(page).toHaveURL(foldersQueryRegex('detail-move-folder'));
  await expect(
    page.locator('.book-list-row').getByRole('heading', { name: 'detail-move-book', exact: true })
  ).toBeVisible();
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

test('shows no chapter card for a plain text book', async ({ page }) => {
  const { baseUrl } = getServer();

  await openBookDetail(page, baseUrl, 'detail-plain-text');

  await expect(page.getByRole('button', { name: 'Start reading' })).toBeVisible();
  await expect(page.locator('.detail-card-chapters')).toHaveCount(0);
});
