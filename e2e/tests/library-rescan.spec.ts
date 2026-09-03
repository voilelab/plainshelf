import { promises as fs } from 'node:fs';
import path from 'node:path';
import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';

/**
 * Puts a book package straight into the shelf, the way copying a folder in from
 * a file manager does. Nothing tells the server about it, so it stays invisible
 * until the tree is walked again.
 */
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

// per-test server: this file's assertions are whole-shelf counts — "1 books",
// "2 books", and the rescan toast "Found 2 books in 1 folders" — which only
// hold on a shelf that starts empty. Sharing a shelf across the file would make
// those totals depend on test order, so each test keeps its own fresh server.
test('Update book list shows a book copied into the shelf from outside', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    await expect(page.getByText('1 books', { exact: true })).toBeVisible();

    await writeBookIntoShelf(server.shelfDir, 'dropped-in', 'drop1nkb', 'Dropped In');

    // Still absent: the server discovers an external change only on its next
    // scan_interval walk, which is a minute away by default.
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

    // What the walk found is announced and goes. It must not take a permanent
    // line in the toolbar: the count is five digits on a real shelf, and a
    // label that wide pushes the controls beside it around.
    //
    // Located by class rather than by text: Reka renders the same words a
    // second time in its off-screen live region, so a text locator matches two
    // elements. The toast body itself is teleported into the viewport.
    const toast = page.locator('.reka-toast-viewport .reka-toast');
    await expect(toast).toHaveText(/Found 2 books in 1 folders/);
    await expect(page.locator('.toolbar-bar.shelf-refresh-bar')).not.toContainText('Found');

    await toast.getByRole('button', { name: 'Dismiss notification' }).click();
    await expect(toast).toHaveCount(0);
  } finally {
    await server.dispose();
  }
});
