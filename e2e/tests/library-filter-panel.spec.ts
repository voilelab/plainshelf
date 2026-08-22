import { expect, test, type Page } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';

async function openFilterPanel(page: Page): Promise<void> {
  await page.getByRole('button', { name: /^Filter/ }).click();
  await expect(page.getByRole('heading', { name: 'Filters' })).toBeVisible();
}

// The imported hello.txt has neither an author nor a cover, so both "unset"
// conditions keep it visible — which lets the test watch two chips, one removal,
// and the URL stay in agreement without editing metadata.
test('applies two panel conditions as chips, then removes one', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    const bookRow = page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true });
    await expect(bookRow).toBeVisible();

    await openFilterPanel(page);

    // Two conditions from two different control shapes: cover (tri-state) and
    // author (single facet).
    await page.locator('.filter-field', { hasText: 'Cover' }).getByRole('radio', { name: 'Unset' }).check();
    await expect(page).toHaveURL(/cover=none/);

    await page.locator('.filter-field', { hasText: 'Author' }).getByRole('radio', { name: 'Unset' }).check();
    await expect(page).toHaveURL(/author=none/);
    await expect(page).toHaveURL(/cover=none/);

    // Two chips, and the book both conditions still match stays on screen.
    await expect(page.getByText('Cover: Unset')).toBeVisible();
    await expect(page.getByText('Author: Unset')).toBeVisible();
    await expect(bookRow).toBeVisible();

    // Remove one chip: its key leaves the URL, the other stays, and the result
    // is still in sync.
    await page.getByRole('button', { name: 'Remove Author: Unset' }).click();
    await expect(page).not.toHaveURL(/author=none/);
    await expect(page).toHaveURL(/cover=none/);
    await expect(page.getByText('Author: Unset')).toBeHidden();
    await expect(page.getByText('Cover: Unset')).toBeVisible();
    await expect(bookRow).toBeVisible();

    // Clear all removes the last chip and the last query key.
    await page.getByRole('button', { name: 'Clear all' }).click();
    await expect(page).not.toHaveURL(/cover=none/);
    await expect(page.locator('.filter-chip')).toHaveCount(0);
    await expect(bookRow).toBeVisible();
  } finally {
    await server.dispose();
  }
});

// The badge on the Filter button counts active panel conditions, and a condition
// that eliminates every book names itself in the empty state.
test('shows an active-condition badge and blames the emptying condition', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    await openFilterPanel(page);
    // hello has no cover, so requiring one empties the list.
    await page.locator('.filter-field', { hasText: 'Cover' }).getByRole('radio', { name: 'Present' }).check();

    await expect(page).toHaveURL(/cover=has/);
    await expect(page.getByText('No books match Cover: Present.')).toBeVisible();

    // The trigger carries the one-active count.
    await expect(page.getByRole('button', { name: 'Filter, 1 active' })).toBeVisible();
  } finally {
    await server.dispose();
  }
});
