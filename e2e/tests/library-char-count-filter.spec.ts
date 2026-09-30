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
// reka's NumberField applies while focus stays on the input, so the panel does
// not close (Tab or blur would move focus out and dismiss it). The results
// behind the panel are still visible, so they can be asserted with the panel
// open.
//
// The bounds are located by their spinbutton role rather than by label: each
// one also has a "Decrease/Increase <label>" stepper button, so a plain label
// lookup matches three elements.
async function openFilterPanel(page: Page): Promise<void> {
  await page.getByRole('button', { name: /^Filter/ }).click();
  await expect(page.getByRole('heading', { name: 'Filters' })).toBeVisible();
}

test('updates the content statistics of books with an unknown character count', async ({ page }) => {
  // per-test server: this test rescans and counts the WHOLE library. The toast
  // "Found 1 books", the "1 with an unknown character count" status, and the
  // "Update statistics for 1 books" button all assert whole-library totals, and
  // clearStoredCharCounts rewrites every book's meta.json on disk. Those totals
  // only equal one on a pristine shelf, so this test keeps its own server.
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
