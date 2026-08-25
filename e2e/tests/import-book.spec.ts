import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import {
  importBookAs,
  createCoverDataTransfer,
  helloFixturePath,
  helloMarkdownFixturePath,
  safeHtmlMarkdownFixturePath
} from './support/books';
import { openReaderTab } from './support/reader';

const getServer = useServer();

test('should import a txt book from the UI and render it in the reader', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'import-txt-render');
  const bookTitle = page.locator('.book-list-row').getByRole('heading', { name: 'import-txt-render', exact: true });
  await bookTitle.click();

  await expect(page).toHaveURL(/\/books\/[^/]+$/);
  await expect(page.getByRole('button', { name: 'Start reading' })).toBeVisible();
  const reader = await openReaderTab(page, () =>
    page.getByRole('button', { name: 'Start reading' }).click()
  );

  await expect(reader.getByRole('heading', { name: 'import-txt-render', exact: true })).toBeVisible();
  await expect(reader.getByText('Hello from PlainShelf E2E.')).toBeVisible();
  await expect(reader.getByText('This text came from a real uploaded TXT file.')).toBeVisible();
});

test('should import a markdown book from the UI and render it as formatted markdown', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloMarkdownFixturePath, 'import-md-render');

  const bookTitle = page.locator('.book-list-row').getByRole('heading', { name: 'import-md-render', exact: true });
  await expect(bookTitle).toBeVisible();
  await bookTitle.click();

  await expect(page).toHaveURL(/\/books\/[^/]+$/);
  await expect(page.getByRole('button', { name: 'Start reading' })).toBeVisible();
  const reader = await openReaderTab(page, () =>
    page.getByRole('button', { name: 'Start reading' }).click()
  );

  await expect(reader.getByRole('heading', { name: 'import-md-render', exact: true })).toBeVisible();
  // Markdown source must be rendered as formatted content, so the "#" heading
  // marker becomes an actual heading element rather than literal text.
  await expect(reader.getByRole('heading', { name: 'Hello Markdown' })).toBeVisible();
  await expect(reader.getByText('# Hello Markdown')).not.toBeVisible();
  await expect(reader.getByText('This text came from a real uploaded MD file.')).toBeVisible();
});

test('should render allow-listed HTML without executing or loading active content', async ({ page }) => {
  const { baseUrl } = getServer();
  const externalRequests: string[] = [];
  // The reader renders in a new tab on the web build, so watch the whole browser
  // context rather than only this page for any active-content fetch.
  page.context().on('request', (request) => {
    if (request.url().startsWith('https://example.com/')) externalRequests.push(request.url());
  });

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, safeHtmlMarkdownFixturePath, 'import-safe-html');
  await page
    .locator('.book-list-row')
    .getByRole('heading', { name: 'import-safe-html', exact: true })
    .click();
  const reader = await openReaderTab(page, () =>
    page.getByRole('button', { name: 'Start reading' }).click()
  );

  const details = reader.locator('.reader-safe-html details');
  const summary = details.getByText('🔮功能區', { exact: true });
  const precedingTextBox = await reader.getByText('回覆長度：中等').boundingBox();
  const detailsBox = await details.boundingBox();
  expect(precedingTextBox).not.toBeNull();
  expect(detailsBox).not.toBeNull();
  if (!precedingTextBox || !detailsBox) throw new Error('Reader content must have a layout box');
  expect(detailsBox.y - (precedingTextBox.y + precedingTextBox.height)).toBeLessThan(100);
  await expect(details).not.toHaveAttribute('open', '');
  await summary.click();
  await expect(details).toHaveAttribute('open', '');
  await expect(reader.getByText('📝總結：開啟')).toBeVisible();
  await expect(details.getByText('本地插圖', { exact: true })).toBeVisible();
  await expect(details.getByText('圖片後的內容仍在功能區內。', { exact: true })).toBeVisible();

  await expect(reader.getByText('劇情內容仍然可見。')).toBeVisible();
  await expect(reader.getByText('紫色心聲')).toHaveCSS('color', 'rgb(128, 0, 128)');
  await expect(reader.getByText('紫色心聲')).toHaveCSS('position', 'static');
  await expect(reader.getByText('藍色對話')).toHaveCSS('color', 'rgb(0, 0, 255)');
  await expect(reader.locator('.reader-safe-html plot')).toHaveCount(0);
  await expect(reader.locator('.reader-safe-html img')).toHaveCount(0);
  expect(await reader.evaluate(() => Reflect.get(window, 'readerHtmlExecuted'))).toBeUndefined();
  expect(externalRequests).toEqual([]);
});

test('should update a book cover from drag and drop on the detail page', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'import-cover-book');

  await page.locator('.book-list-row').getByRole('heading', { name: 'import-cover-book', exact: true }).click();
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
});
