import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { helloFixturePath, importBookAs } from './support/books';
import { openReaderTab } from './support/reader';

const getServer = useServer();

test('should record a book in read history after visiting the reader and clear it', { tag: '@smoke' }, async ({
  page
}) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'readhist-record-book');

  await page.locator('.book-list-row').getByRole('heading', { name: 'readhist-record-book', exact: true }).click();
  await expect(page).toHaveURL(/\/books\/[^/]+$/);
  const reader = await openReaderTab(page, () =>
    page.getByRole('button', { name: 'Start reading' }).click()
  );
  await expect(reader.getByText('Hello from PlainShelf E2E.')).toBeVisible();
  await reader.close();

  await page.goto(`${baseUrl}/read-history`);
  await expect(page.getByRole('heading', { name: 'Recently Read' })).toBeVisible();

  await expect(page.getByRole('heading', { name: 'readhist-record-book', exact: true })).toBeVisible();

  await page.reload();
  const historyBook = page.getByRole('heading', { name: 'readhist-record-book', exact: true });
  await expect(historyBook).toBeVisible();

  await historyBook.click();
  await expect(page).toHaveURL(/\/books\/[^/?]+$/);

  await page.goto(`${baseUrl}/read-history`);

  await page.getByRole('button', { name: 'Clear history' }).click();

  await expect(
    page.getByText('No reading history yet. Open a book in the reader to see it here.')
  ).toBeVisible();
});
