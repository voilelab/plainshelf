import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';
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
    await page.route('**/books/*/content', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'text/plain; charset=utf-8',
        body: '🍥🍥'
      });
    });
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
