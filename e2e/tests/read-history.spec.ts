import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { helloFixturePath, importBookAs } from './support/books';
import { openReaderTab } from './support/reader';

const getServer = useServer();

test('should record a book in read history after visiting the reader and clear it', async ({
  page
}) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'readhist-record-book');

  // Visit the reader — this records the book in the browser's own storage,
  // not on the server. On the web build the reader opens in a new tab, which
  // shares the origin's localStorage with this one. The read history lives in
  // this test's fresh browser context, so it holds only this book.
  await page.locator('.book-list-row').getByRole('heading', { name: 'readhist-record-book', exact: true }).click();
  await expect(page).toHaveURL(/\/books\/[^/]+$/);
  const reader = await openReaderTab(page, () =>
    page.getByRole('button', { name: 'Start reading' }).click()
  );
  // Wait for content to load so the history entry has been written
  await expect(reader.getByText('Hello from PlainShelf E2E.')).toBeVisible();
  await reader.close();

  // Navigate to the read history page
  await page.goto(`${baseUrl}/read-history`);
  await expect(page.getByRole('heading', { name: 'Recently Read' })).toBeVisible();

  // The book should appear in the list
  await expect(page.getByRole('heading', { name: 'readhist-record-book', exact: true })).toBeVisible();

  // …and still be there after a reload, proving it was persisted locally
  // rather than kept in memory for the session.
  await page.reload();
  const historyBook = page.getByRole('heading', { name: 'readhist-record-book', exact: true });
  await expect(historyBook).toBeVisible();

  // Clicking a history item should open the same book detail route as the
  // main library collection.
  await historyBook.click();
  await expect(page).toHaveURL(/\/books\/[^/?]+$/);

  await page.goto(`${baseUrl}/read-history`);

  // Clear history
  await page.getByRole('button', { name: 'Clear history' }).click();

  // Empty state should be shown
  await expect(
    page.getByText('No reading history yet. Open a book in the reader to see it here.')
  ).toBeVisible();
});

test('dashboard "recent reading" cards keep a bounded width with a single book', async ({
  page
}) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'readhist-dashboard-book');

  // Read the single book so it lands in the browser's read history, which the
  // dashboard's "recent reading" section reflects. History is per-context, so
  // this test's fresh context has exactly one recent book.
  await page.locator('.book-list-row').getByRole('heading', { name: 'readhist-dashboard-book', exact: true }).click();
  await expect(page).toHaveURL(/\/books\/[^/]+$/);
  const reader = await openReaderTab(page, () =>
    page.getByRole('button', { name: 'Start reading' }).click()
  );
  await expect(reader.getByText('Hello from PlainShelf E2E.')).toBeVisible();
  await reader.close();

  // Open the home page (redirects to the dashboard) with only one recent book.
  await page.goto(`${baseUrl}/`);
  const cover = page.locator('.recent-reading-cover').first();
  await expect(cover).toBeVisible();

  // With a single book the card must not stretch to fill the row: its width is
  // capped, so the 2:3 cover cannot balloon to thousands of px tall.
  const box = await cover.boundingBox();
  expect(box).not.toBeNull();
  expect(box!.width).toBeLessThanOrEqual(200);
});
