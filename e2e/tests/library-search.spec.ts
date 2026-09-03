import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { importBookAs, helloFixturePath, anotherFixturePath } from './support/books';

const getServer = useServer();

test('should filter books by search query and restore the full list after clearing', { tag: '@smoke' }, async ({
  page
}) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);

  // Import two distinct books. The shelf is shared across this file's tests, so
  // each carries a name unique to this test and the search targets that name.
  await importBookAs(page, helloFixturePath, 'search-filter-alpha');
  await importBookAs(page, anotherFixturePath, 'search-filter-beta');
  await expect(
    page.locator('.book-list-row').getByRole('heading', { name: 'search-filter-beta', exact: true })
  ).toBeVisible();

  // Search for the first book's unique title — only that book should match
  await page.locator('input[type="search"]').fill('search-filter-alpha');
  await page.getByRole('button', { name: 'Search', exact: true }).click();

  await expect(
    page.locator('.book-list-row').getByRole('heading', { name: 'search-filter-alpha', exact: true })
  ).toBeVisible();
  await expect(
    page.locator('.book-list-row').getByRole('heading', { name: 'search-filter-beta', exact: true })
  ).not.toBeVisible();

  // Clear search — both books should reappear
  await page.getByLabel('Clear search').click();

  await expect(
    page.locator('.book-list-row').getByRole('heading', { name: 'search-filter-alpha', exact: true })
  ).toBeVisible();
  await expect(
    page.locator('.book-list-row').getByRole('heading', { name: 'search-filter-beta', exact: true })
  ).toBeVisible();
});
