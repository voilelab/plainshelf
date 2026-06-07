import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';

test('should record a book in read history after visiting the reader and clear it', async ({
  page
}) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    // Visit the reader — this triggers addReadHistory on the server
    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await page.getByRole('button', { name: 'Read' }).click();
    await expect(page).toHaveURL(/\/reader\/[^/]+$/);
    // Wait for content to load so the read-history POST has been sent
    await expect(page.getByText('Hello from PlainShelf E2E.')).toBeVisible();

    // Navigate to the read history page
    await page.goto(`${server.baseUrl}/read-history`);
    await expect(page.getByRole('heading', { name: 'Recently Read' })).toBeVisible();

    // The book should appear in the list
    await expect(page.getByRole('heading', { name: 'hello', exact: true })).toBeVisible();

    // Clear history
    await page.getByRole('button', { name: 'Clear history' }).click();

    // Empty state should be shown
    await expect(
      page.getByText('No reading history yet. Open a book in the reader to see it here.')
    ).toBeVisible();
  } finally {
    await server.dispose();
  }
});
