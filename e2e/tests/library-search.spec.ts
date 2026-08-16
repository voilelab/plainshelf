import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook, importBookFromPath, anotherFixturePath } from './support/books';

test('should filter books by search query and restore the full list after clearing', async ({
  page
}) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);

    // Import two distinct books
    await importHelloBook(page);
    await importBookFromPath(page, anotherFixturePath);
    await expect(page.getByText('2 books')).toBeVisible();
    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'another', exact: true })
    ).toBeVisible();

    // Search for "hello" — only the first book should match
    await page.locator('input[type="search"]').fill('hello');
    await page.getByRole('button', { name: 'Search', exact: true }).click();

    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true })
    ).toBeVisible();
    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'another', exact: true })
    ).not.toBeVisible();

    // Clear search — both books should reappear
    await page.getByLabel('Clear search').click();

    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true })
    ).toBeVisible();
    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'another', exact: true })
    ).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('a search with no matches shows the empty state without a pagination row', async ({
  page
}) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    await expect(page.locator('.pagination')).toBeVisible();

    await page.locator('input[type="search"]').fill('nothing-matches-this');
    await page.getByRole('button', { name: 'Search', exact: true }).click();

    await expect(page.getByText('No books found for "nothing-matches-this".')).toBeVisible();
    // The pager used to render regardless of the result count, so the empty
    // state was followed by a "Page 1 / 0" control row.
    await expect(page.locator('.pagination')).toHaveCount(0);

    await page.getByLabel('Clear search').click();
    await expect(page.locator('.pagination')).toBeVisible();
  } finally {
    await server.dispose();
  }
});
