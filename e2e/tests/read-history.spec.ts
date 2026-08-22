import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';
import { openReaderTab } from './support/reader';

test('should record a book in read history after visiting the reader and clear it', async ({
  page
}) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    // Visit the reader — this records the book in the browser's own storage,
    // not on the server. On the web build the reader opens in a new tab, which
    // shares the origin's localStorage with this one.
    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    const reader = await openReaderTab(page, () =>
      page.getByRole('button', { name: 'Start reading' }).click()
    );
    // Wait for content to load so the history entry has been written
    await expect(reader.getByText('Hello from PlainShelf E2E.')).toBeVisible();
    await reader.close();

    // Navigate to the read history page
    await page.goto(`${server.baseUrl}/read-history`);
    await expect(page.getByRole('heading', { name: 'Recently Read' })).toBeVisible();

    // The book should appear in the list
    await expect(page.getByRole('heading', { name: 'hello', exact: true })).toBeVisible();

    // …and still be there after a reload, proving it was persisted locally
    // rather than kept in memory for the session.
    await page.reload();
    const historyBook = page.getByRole('heading', { name: 'hello', exact: true });
    await expect(historyBook).toBeVisible();

    // Clicking a history item should open the same book detail route as the
    // main library collection.
    await historyBook.click();
    await expect(page).toHaveURL(/\/books\/[^/?]+$/);

    await page.goto(`${server.baseUrl}/read-history`);

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
