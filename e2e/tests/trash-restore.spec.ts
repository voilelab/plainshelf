import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';

test('should move a book to trash and restore it back to the library', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    // Open detail page
    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);

    // Click "Move to Trash" and confirm
    await page.getByRole('button', { name: 'Move to Trash' }).click();
    const deleteDialog = page.getByRole('dialog', { name: 'Confirm delete' });
    await expect(deleteDialog).toBeVisible();
    await deleteDialog.getByRole('button', { name: 'Delete', exact: true }).click();

    // Should redirect to /books (LibraryPage may append its default query params)
    await expect(page).toHaveURL(/\/books(\?|$)/);
    await expect(page.getByText('No books yet.')).toBeVisible();

    // Navigate to trash
    await page.goto(`${server.baseUrl}/trash`);
    await expect(page.getByRole('heading', { name: 'Trash' })).toBeVisible();
    await expect(page.getByRole('cell', { name: 'hello', exact: true })).toBeVisible();

    // Restore the book
    await page.getByRole('button', { name: 'Restore' }).click();
    await expect(page.getByText('Trash is empty.')).toBeVisible();

    // Navigate back to library and verify the book is restored
    await page.goto(`${server.baseUrl}/books`);
    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true })
    ).toBeVisible();
  } finally {
    await server.dispose();
  }
});
