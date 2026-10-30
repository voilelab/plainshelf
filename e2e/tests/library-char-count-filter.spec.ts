import { expect, test, type Page } from '@playwright/test';
import { promises as fs } from 'node:fs';
import path from 'node:path';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';

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

async function openFilterPanel(page: Page): Promise<void> {
  await page.getByRole('button', { name: /^Filter/ }).click();
  await expect(page.getByRole('heading', { name: 'Filters' })).toBeVisible();
}

test('updates the content statistics of books with an unknown character count', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    expect(await clearStoredCharCounts(server.shelfDir)).toBeGreaterThan(0);

    await page.getByRole('button', { name: 'Update book list' }).click();
    await expect(page.locator('.reka-toast-viewport .reka-toast')).toContainText(/Found 1 books/);

    await page.goto(`${server.baseUrl}/books?maxChars=1`);

    const bookRow = page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true });
    await expect(bookRow).toBeVisible();

    await openFilterPanel(page);
    await expect(page.getByText('1 with an unknown character count')).toBeVisible();

    const refreshButton = page.getByRole('button', { name: /^Update statistics for/ });
    await expect(refreshButton).toHaveText('Update statistics for 1 books');
    await refreshButton.click();

    await expect(bookRow).toBeHidden();
    await expect(page.getByText('No books in this character range.')).toBeVisible();
  } finally {
    await server.dispose();
  }
});
