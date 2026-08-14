import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import {
  importHelloBook,
  importBookFromPath,
  createCoverDataTransfer,
  helloMarkdownFixturePath,
  safeHtmlMarkdownFixturePath
} from './support/books';

test('should import a txt book from the UI and render it in the reader', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    const bookTitle = page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true });
    await bookTitle.click();

    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await expect(page.getByRole('button', { name: 'Start reading' })).toBeVisible();
    await page.getByRole('button', { name: 'Start reading' }).click();

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

    await expect(page.getByText('1 books')).toBeVisible();
    const bookTitle = page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true });
    await expect(bookTitle).toBeVisible();
    await bookTitle.click();

    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await expect(page.getByRole('button', { name: 'Start reading' })).toBeVisible();
    await page.getByRole('button', { name: 'Start reading' }).click();

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

test('should render allow-listed HTML without executing or loading active content', async ({ page }) => {
  const server = await startServer();
  const externalRequests: string[] = [];
  page.on('request', (request) => {
    if (request.url().startsWith('https://example.com/')) externalRequests.push(request.url());
  });

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importBookFromPath(page, safeHtmlMarkdownFixturePath);
    await page
      .locator('.book-list-row')
      .getByRole('heading', { name: 'safe-html', exact: true })
      .click();
    await page.getByRole('button', { name: 'Start reading' }).click();

    const details = page.locator('.reader-safe-html details');
    const summary = details.getByText('🔮功能區', { exact: true });
    const precedingTextBox = await page.getByText('回覆長度：中等').boundingBox();
    const detailsBox = await details.boundingBox();
    expect(precedingTextBox).not.toBeNull();
    expect(detailsBox).not.toBeNull();
    if (!precedingTextBox || !detailsBox) throw new Error('Reader content must have a layout box');
    expect(detailsBox.y - (precedingTextBox.y + precedingTextBox.height)).toBeLessThan(100);
    await expect(details).not.toHaveAttribute('open', '');
    await summary.click();
    await expect(details).toHaveAttribute('open', '');
    await expect(page.getByText('📝總結：開啟')).toBeVisible();
    await expect(details.getByText('本地插圖', { exact: true })).toBeVisible();
    await expect(details.getByText('圖片後的內容仍在功能區內。', { exact: true })).toBeVisible();

    await expect(page.getByText('劇情內容仍然可見。')).toBeVisible();
    await expect(page.getByText('紫色心聲')).toHaveCSS('color', 'rgb(128, 0, 128)');
    await expect(page.getByText('紫色心聲')).toHaveCSS('position', 'static');
    await expect(page.getByText('藍色對話')).toHaveCSS('color', 'rgb(0, 0, 255)');
    await expect(page.locator('.reader-safe-html plot')).toHaveCount(0);
    await expect(page.locator('.reader-safe-html img')).toHaveCount(0);
    expect(await page.evaluate(() => Reflect.get(window, 'readerHtmlExecuted'))).toBeUndefined();
    expect(externalRequests).toEqual([]);
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
      await expect(page.getByText('Drop the image to update the cover')).toBeVisible();
      await coverTarget.dispatchEvent('dragover', { dataTransfer });
      await coverTarget.dispatchEvent('drop', { dataTransfer });
    } finally {
      await dataTransfer.dispose();
    }

    const confirmDialog = page.getByRole('dialog', { name: 'Update book cover?' });
    await expect(confirmDialog).toBeVisible();
    await expect(confirmDialog.getByText('Use this image as the new book cover?')).toBeVisible();
    await confirmDialog.getByRole('button', { name: 'Update cover' }).click();

    await expect(confirmDialog).not.toBeVisible();
    await page.getByRole('button', { name: 'Cover options' }).click();
    await expect(page.getByRole('button', { name: 'Remove' })).toBeEnabled();
    await expect(page.locator('img.detail-cover')).toHaveAttribute(
      'src',
      /\/api\/shelves\/default_shelf\/books\/[^/]+\/cover(?:\?t=\d+)?$/
    );
  } finally {
    await server.dispose();
  }
});
