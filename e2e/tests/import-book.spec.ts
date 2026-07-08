import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook, importBookFromPath, createCoverDataTransfer, expectBookCount, helloMarkdownFixturePath } from './support/books';

test('should import a txt book from the UI and render it in the reader', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    const bookTitle = page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true });
    await bookTitle.click();

    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await expect(page.getByRole('button', { name: 'Read' })).toBeVisible();
    await page.getByRole('button', { name: 'Read' }).click();

    await expect(page).toHaveURL(/\/reader\/[^/]+$/);
    await expect(page.getByRole('heading', { name: 'hello', exact: true })).toBeVisible();
    await expect(page.getByText('Hello from PlainShelf E2E.')).toBeVisible();
    await expect(page.getByText('This text came from a real uploaded TXT file.')).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('should import a markdown book from the UI and render it as formatted markdown', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
    await expect(page.getByText('No books yet.')).toBeVisible();

    await importBookFromPath(page, helloMarkdownFixturePath);

    await expectBookCount(page, 1);
    const bookTitle = page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true });
    await expect(bookTitle).toBeVisible();
    await bookTitle.click();

    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await expect(page.getByRole('button', { name: 'Read' })).toBeVisible();
    await page.getByRole('button', { name: 'Read' }).click();

    await expect(page).toHaveURL(/\/reader\/[^/]+$/);
    await expect(page.getByRole('heading', { name: 'hello', exact: true })).toBeVisible();
    // Markdown source must be rendered as formatted content, so the "#" heading
    // marker becomes an actual heading element rather than literal text.
    await expect(page.getByRole('heading', { name: 'Hello Markdown' })).toBeVisible();
    await expect(page.getByText('# Hello Markdown')).not.toBeVisible();
    await expect(page.getByText('This text came from a real uploaded MD file.')).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('should update a book cover from drag and drop on the detail page', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);

    const coverTarget = page.locator('.cover-drop-target');
    await expect(coverTarget).toBeVisible();

    const dataTransfer = await createCoverDataTransfer(page);
    try {
      await coverTarget.dispatchEvent('dragenter', { dataTransfer });
      await expect(page.getByText('Drop image to update cover')).toBeVisible();
      await coverTarget.dispatchEvent('dragover', { dataTransfer });
      await coverTarget.dispatchEvent('drop', { dataTransfer });
    } finally {
      await dataTransfer.dispose();
    }

    const confirmDialog = page.getByRole('dialog', { name: 'Update book cover?' });
    await expect(confirmDialog).toBeVisible();
    await expect(confirmDialog.getByText('Do you want to update the book cover?')).toBeVisible();
    await confirmDialog.getByRole('button', { name: 'Update cover' }).click();

    await expect(confirmDialog).not.toBeVisible();
    await expect(page.getByRole('button', { name: 'Remove' })).toBeEnabled();
    await expect(page.locator('img.detail-cover')).toHaveAttribute(
      'src',
      /\/api\/shelves\/default_shelf\/books\/[^/]+\/cover(?:\?t=\d+)?$/
    );
  } finally {
    await server.dispose();
  }
});
