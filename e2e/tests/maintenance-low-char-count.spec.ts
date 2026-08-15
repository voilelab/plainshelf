import { expect, test } from '@playwright/test';
import { promises as fs } from 'node:fs';
import path from 'node:path';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';

/**
 * Drops char_count from every source meta.json under the shelf, reproducing a
 * book whose statistics were never computed. The API then omits char_count,
 * which is what the page reports as an unknown count.
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

// hello.txt is 74 characters, so it sits far below the default 1000 threshold
// and far above a threshold of 1.
test('low character count page filters books by the threshold input', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    await page
      .locator('#sidebar-section-maintenance')
      .getByRole('link', { name: 'Low Character Count' })
      .click();

    await expect(page).toHaveURL(/\/books\/maintenance\/low-char-count/);
    await expect(page.getByRole('heading', { name: 'Low Character Count' })).toBeVisible();

    const bookRow = page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true });
    await expect(bookRow).toBeVisible();

    const thresholdInput = page.getByLabel('Max characters');
    await expect(thresholdInput).toHaveValue('1000');

    await thresholdInput.fill('1');
    await thresholdInput.blur();

    await expect(page).toHaveURL(/maxChars=1(&|$)/);
    await expect(page.getByText('No books under this character count.')).toBeVisible();

    await thresholdInput.fill('5000');
    await thresholdInput.blur();

    await expect(bookRow).toBeVisible();

    await bookRow.click();
    await expect(page).toHaveURL(/\/books\/(?!maintenance\/)[^/?]+$/);
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

    await page.goto(`${server.baseUrl}/books/maintenance/low-char-count`);
    await expect(page.getByText('1 with an unknown character count')).toBeVisible();

    const refreshButton = page.getByRole('button', { name: /^Update statistics for/ });
    await expect(refreshButton).toHaveText('Update statistics for 1 books');
    await refreshButton.click();

    await expect(page.getByRole('status')).toHaveText('Updated 1 books');
    await expect(page.getByText('1 with an unknown character count')).toBeHidden();
    await expect(refreshButton).toBeDisabled();

    // hello.txt is 74 characters, so the recomputed count puts the book back
    // above a threshold of 1 instead of leaving it indistinguishable from 0.
    await page.getByLabel('Max characters').fill('1');
    await page.getByLabel('Max characters').blur();
    await expect(page.getByText('No books under this character count.')).toBeVisible();
  } finally {
    await server.dispose();
  }
});
