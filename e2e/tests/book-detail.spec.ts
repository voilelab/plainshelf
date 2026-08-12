import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';

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
