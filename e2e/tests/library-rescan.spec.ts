import { promises as fs } from 'node:fs';
import path from 'node:path';
import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';

async function writeBookIntoShelf(
  shelfDir: string,
  dirName: string,
  bookID: string,
  title: string
): Promise<void> {
  const bookDir = path.join(shelfDir, 'books', `${dirName}.bookpkg`);
  await fs.mkdir(bookDir, { recursive: true });
  await fs.writeFile(
    path.join(bookDir, 'book.json'),
    JSON.stringify({ schema_version: 1, id: bookID, title })
  );
}

test('Update book list shows a book copied into the shelf from outside', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    await expect(page.getByText('1 books', { exact: true })).toBeVisible();

    await writeBookIntoShelf(server.shelfDir, 'dropped-in', 'drop1nkb', 'Dropped In');

    await page.reload();
    await expect(page.getByText('1 books', { exact: true })).toBeVisible();

    const rescan = page.getByRole('button', { name: 'Update book list' });
    await expect(rescan).toBeEnabled();
    await rescan.click();

    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'Dropped In', exact: true })
    ).toBeVisible();
    await expect(page.getByText('2 books', { exact: true })).toBeVisible();
    await expect(rescan).toBeEnabled();

    // Reka UI 2.10.5 duplicates toast text in a visually hidden alert; the visible body is teleported into ToastViewport.
    const toast = page.locator('.reka-toast-viewport .reka-toast');
    await expect(toast).toHaveText(/Found 2 books in 1 folders/);
    await expect(page.locator('.toolbar-bar.shelf-refresh-bar')).not.toContainText('Found');

    await toast.getByRole('button', { name: 'Dismiss notification' }).click();
    await expect(toast).toHaveCount(0);
  } finally {
    await server.dispose();
  }
});
