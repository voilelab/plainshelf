import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';

test('should empty the trash through a background task and report its progress', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await page.getByRole('button', { name: 'More' }).click();
    await page.getByRole('menuitem', { name: 'Move to Trash' }).click();
    const deleteDialog = page.getByRole('alertdialog', { name: 'Confirm delete' });
    await expect(deleteDialog).toBeVisible();
    await deleteDialog.getByRole('button', { name: 'Delete', exact: true }).click();

    await page.goto(`${server.baseUrl}/trash`);
    await expect(page.getByRole('cell', { name: 'hello', exact: true })).toBeVisible();

    const emptyButton = page.getByRole('button', { name: 'Empty trash', exact: true }).first();
    await expect(emptyButton).toBeEnabled();
    await emptyButton.click();

    const dialog = page.getByRole('alertdialog', { name: 'Empty trash' });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText('Permanently delete all 1 books in the trash?')).toBeVisible();

    await dialog.getByRole('button', { name: 'Empty trash', exact: true }).click();

    const progressBar = dialog.getByRole('progressbar');
    await expect(progressBar).toBeVisible();
    await expect(progressBar).toHaveAttribute('aria-valuenow', '100');
    await expect(dialog.getByText('The trash is now empty.')).toBeVisible();

    await dialog.getByRole('button', { name: 'Close', exact: true }).click();

    await expect(page.getByText('Trash is empty.')).toBeVisible();

    await expect(emptyButton).toBeEnabled();
    await emptyButton.click();
    await expect(dialog.getByText(/Permanently delete everything in the trash\?/)).toBeVisible();
  } finally {
    await server.dispose();
  }
});
