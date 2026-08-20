import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { chaptersMarkdownFixturePath, importBookFromPath, importHelloBook } from './support/books';
import { addLayer, layersQueryRegex, selectAllBooks } from './support/layers';

async function openHelloDetail(page: import('@playwright/test').Page, baseUrl: string): Promise<void> {
  await page.goto(`${baseUrl}/books`);
  await importHelloBook(page);
  await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
  await expect(page).toHaveURL(/\/books\/[^/?]+$/);
}

test('keeps reading progress and the primary action in the desktop first viewport', async ({ page }) => {
  const server = await startServer();

  try {
    await page.setViewportSize({ width: 1280, height: 720 });
    await openHelloDetail(page, server.baseUrl);

    await expect(page.getByRole('heading', { name: 'hello', exact: true })).toBeInViewport();
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

    const dialog = page.getByRole('dialog');
    await expect(dialog).toContainText('The book will be moved to Trash. You can restore it later.');
    await dialog.getByRole('button', { name: 'Cancel' }).click();
    await expect(dialog).not.toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('keeps the summary and reading action above the fold on a narrow viewport', async ({ page }) => {
  const server = await startServer();

  try {
    await page.setViewportSize({ width: 390, height: 844 });
    await openHelloDetail(page, server.baseUrl);

    await expect(page.getByRole('heading', { name: 'hello', exact: true })).toBeInViewport();
    await expect(page.getByRole('button', { name: 'Start reading' })).toBeInViewport();
    await expect(page.getByRole('button', { name: 'Export file' })).toBeInViewport();

    await page.getByRole('combobox').last().click();
    await page.getByRole('option', { name: '繁體中文' }).click();
    await expect(page.getByRole('button', { name: '開始閱讀' })).toBeVisible();
    await expect(page.getByText('尚未開始', { exact: true })).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('derives reading progress using the same UTF-16 units as the reader', async ({ page }) => {
  const server = await startServer();
  const markRequests: string[] = [];
  page.on('request', (request) => {
    if (request.url().includes('/marks/')) markRequests.push(request.url());
  });

  try {
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
    await openHelloDetail(page, server.baseUrl);
    const bookID = new URL(page.url()).pathname.split('/').pop()!;
    await page.evaluate(
      ({ id }) => {
        localStorage.setItem(
          'plainshelf.readingProgress',
          JSON.stringify({ version: 1, shelves: { default_shelf: { [id]: 2 } } })
        );
      },
      { id: bookID }
    );
    await page.reload();

    await expect(page.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '50');
    await expect(page.getByRole('button', { name: 'Continue reading · 50%' })).toBeVisible();
    await expect(page.getByText('Continue where you left off', { exact: true })).toBeVisible();
    expect(markRequests).toEqual([]);
  } finally {
    await server.dispose();
  }
});

test('resets a completed bookmark before reading again', async ({ page }) => {
  const server = await startServer();
  const markRequests: string[] = [];
  page.on('request', (request) => {
    if (request.url().includes('/marks/')) markRequests.push(request.url());
  });

  try {
    await openHelloDetail(page, server.baseUrl);
    const bookID = new URL(page.url()).pathname.split('/').pop()!;
    await page.evaluate(
      ({ id }) => {
        localStorage.setItem(
          'plainshelf.readingProgress',
          JSON.stringify({ version: 1, shelves: { default_shelf: { [id]: 74 } } })
        );
      },
      { id: bookID }
    );
    await page.reload();
    await expect(page.getByRole('button', { name: 'Read again' })).toBeVisible();

    await page.getByRole('button', { name: 'Read again' }).click();

    await expect(page).toHaveURL(/\/reader\/[^/?]+$/);
    await expect.poll(() => page.evaluate(
      ({ id }) => {
        const stored = JSON.parse(localStorage.getItem('plainshelf.readingProgress') ?? '{}');
        return stored.shelves?.default_shelf?.[id] ?? 0;
      },
      { id: bookID }
    )).toBe(0);
    await expect(page.getByText('Progress: 0%', { exact: true })).toBeVisible();
    expect(markRequests).toEqual([]);
  } finally {
    await server.dispose();
  }
});

test('moves the book to another folder from the More menu', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    await addLayer(page, 'moved-here');
    await selectAllBooks(page);
    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/?]+$/);

    await page.getByRole('button', { name: 'More' }).click();
    await page.getByRole('menuitem', { name: 'Move to…', exact: true }).click();

    const moveDialog = page.getByRole('dialog', { name: 'Move book' });
    await moveDialog.getByLabel('Destination').selectOption('moved-here');

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
    await expect(breadcrumb.getByRole('link', { name: 'moved-here', exact: true })).toBeVisible();

    await breadcrumb.getByRole('link', { name: 'moved-here', exact: true }).click();
    await expect(page).toHaveURL(layersQueryRegex('moved-here'));
    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true })
    ).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('opens the reader at a chapter picked from the detail page', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importBookFromPath(page, chaptersMarkdownFixturePath);
    await page.locator('.book-list-row').getByRole('heading', { name: 'chapters', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/?]+$/);

    // The list is a deep link into the reader, so it has to show the reader's
    // own sections: the opening ahead of the first H2 included, in that order.
    const chapterCard = page.locator('.detail-card-chapters');
    await expect(chapterCard.locator('.chapter-item-title')).toHaveText([
      'Chapter Sampler',
      'First Light',
      'Second Wind',
      'Third Rail'
    ]);

    await chapterCard.getByRole('button', { name: 'Second Wind' }).click();
    await expect(page).toHaveURL(/\/reader\/[^/?]+\?section=2$/);
    await expect(page.locator('.reader-text')).toContainText('Text of the second chapter.');

    await page.getByRole('button', { name: 'Show chapters' }).click();
    await expect(page.locator('.chapter-modal-item.active')).toContainText('Second Wind');
  } finally {
    await server.dispose();
  }
});

test('shows no chapter card for a plain text book', async ({ page }) => {
  const server = await startServer();

  try {
    await openHelloDetail(page, server.baseUrl);

    await expect(page.getByRole('button', { name: 'Start reading' })).toBeVisible();
    await expect(page.locator('.detail-card-chapters')).toHaveCount(0);
  } finally {
    await server.dispose();
  }
});
