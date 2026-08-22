import { expect, test, type Page } from '@playwright/test';
import { promises as fs } from 'node:fs';
import path from 'node:path';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';

/**
 * Drops char_count from every source meta.json under the shelf, reproducing a
 * book whose statistics were never computed. The API then omits char_count,
 * which is what the library toolbar reports as an unknown count.
 */
async function clearStoredCharCounts(shelfDir: string): Promise<number> {
  let cleared = 0;

  const walk = async (dir: string): Promise<void> => {
    for (const entry of await fs.readdir(dir, { withFileTypes: true })) {
      const entryPath = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        await walk(entryPath);
        continue;
      }
      if (entry.name !== 'meta.json') {
        continue;
      }

      const meta = JSON.parse(await fs.readFile(entryPath, 'utf8')) as Record<string, unknown>;
      if (!('char_count' in meta)) {
        continue;
      }
      delete meta.char_count;
      await fs.writeFile(entryPath, JSON.stringify(meta), 'utf8');
      cleared += 1;
    }
  };

  await walk(shelfDir);
  return cleared;
}

// The character-range control lives inside the filter panel now. The panel is
// opened once and kept open across edits: a bound is committed with Enter, which
// fires the input's change while focus stays on the input, so the panel does not
// close (Tab or blur would move focus out and dismiss it). The results behind
// the panel are still visible, so they can be asserted with the panel open.
async function openFilterPanel(page: Page): Promise<void> {
  await page.getByRole('button', { name: /^Filter/ }).click();
  await expect(page.getByRole('heading', { name: 'Filters' })).toBeVisible();
}

// hello.txt is 74 characters, so it sits inside a 0-5000 range, above a
// maximum of 1, and below a minimum of 100.
test('library filters books by a character-count range', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    const bookRow = page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true });
    await expect(bookRow).toBeVisible();

    await openFilterPanel(page);
    const minInput = page.getByLabel('Minimum characters');
    const maxInput = page.getByLabel('Maximum characters');

    // Both bounds start empty, which is the unlimited state.
    await expect(minInput).toHaveValue('');
    await expect(maxInput).toHaveValue('');

    await maxInput.fill('1');
    await maxInput.press('Enter');

    await expect(page).toHaveURL(/maxChars=1(&|$)/);
    await expect(page.getByText('No books in this character range.')).toBeVisible();

    await maxInput.fill('5000');
    await maxInput.press('Enter');

    await expect(page).toHaveURL(/maxChars=5000(&|$)/);
    await expect(bookRow).toBeVisible();

    // A lower bound above the book's length excludes it from the same range.
    await minInput.fill('100');
    await minInput.press('Enter');

    await expect(page).toHaveURL(/minChars=100(&|$)/);
    await expect(page.getByText('No books in this character range.')).toBeVisible();

    await minInput.fill('10');
    await minInput.press('Enter');
    await expect(bookRow).toBeVisible();

    // Clearing removes the clear button (and may dismiss the panel as focus
    // leaves it), so the cleared state is asserted from the URL and the result.
    await page.getByRole('button', { name: 'Clear character range' }).click();
    await expect(page).not.toHaveURL(/[?&](min|max)Chars=/);
    await expect(bookRow).toBeVisible();
  } finally {
    await server.dispose();
  }
});

// An active range must not take the blame for a set the search had already
// emptied: the message has to name the cause the user can act on.
test('an active range does not mask the search empty state', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    await openFilterPanel(page);
    const maxInput = page.getByLabel('Maximum characters');
    await maxInput.fill('5000');
    await maxInput.press('Enter');
    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true })
    ).toBeVisible();

    await page.locator('input[type="search"]').fill('nothingmatchesthis');
    await page.getByRole('button', { name: 'Search', exact: true }).click();

    await expect(page.getByText('No books found for "nothingmatchesthis"')).toBeVisible();
    await expect(page.getByText('No books in this character range.')).toBeHidden();
  } finally {
    await server.dispose();
  }
});

test('reversed bounds are applied as a single ordered range', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    await openFilterPanel(page);
    const minInput = page.getByLabel('Minimum characters');
    const maxInput = page.getByLabel('Maximum characters');

    await minInput.fill('5000');
    await minInput.press('Enter');
    await maxInput.fill('10');
    await maxInput.press('Enter');

    // Committed reversed, stored in order.
    await expect(minInput).toHaveValue('10');
    await expect(maxInput).toHaveValue('5000');
    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true })
    ).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('updates the content statistics of books with an unknown character count', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    expect(await clearStoredCharCounts(server.shelfDir)).toBeGreaterThan(0);

    // Character counts are answered from the server's book cache, so an edit
    // made straight on disk reaches it through a walk of the shelf - the same
    // button a user presses after changing a shelf from outside PlainShelf.
    await page.getByRole('button', { name: 'Update book list' }).click();
    await expect(page.locator('.reka-toast-viewport .reka-toast')).toContainText(/Found 1 books/);

    // A book with no stored count reads as zero characters, so it still falls
    // inside a range whose maximum is 1.
    await page.goto(`${server.baseUrl}/books?maxChars=1`);

    const bookRow = page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true });
    await expect(bookRow).toBeVisible();

    await openFilterPanel(page);
    await expect(page.getByText('1 with an unknown character count')).toBeVisible();

    const refreshButton = page.getByRole('button', { name: /^Update statistics for/ });
    await expect(refreshButton).toHaveText('Update statistics for 1 books');
    await refreshButton.click();

    // The sweep recomputes counts; hello becomes 74 characters and drops out of
    // the 0-1 range instead of staying indistinguishable from an empty book.
    // Asserting the result (not the in-panel status) keeps this independent of
    // whether the panel is still open once the sweep finishes.
    await expect(bookRow).toBeHidden();
    await expect(page.getByText('No books in this character range.')).toBeVisible();
  } finally {
    await server.dispose();
  }
});
