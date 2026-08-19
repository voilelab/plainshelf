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

// Adding this button took the toolbar past the width it had left, and a row
// that cannot wrap does not shrink — it pushes its last control out of the
// header and clips it, which is how the button becomes unreachable.
test('the library toolbar fits its header instead of clipping its last control', async ({
  page
}) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    for (const width of [1600, 1280, 1024, 800]) {
      await page.setViewportSize({ width, height: 800 });
      // The rescan button is the last control and therefore the first casualty.
      await expect(page.getByRole('button', { name: 'Update book list' })).toBeInViewport();

      const overflowPx = await page.evaluate(() => {
        const toolbar = document.querySelector('.bookshelf-toolbar') as HTMLElement;
        const header = toolbar.parentElement as HTMLElement;
        return toolbar.getBoundingClientRect().right - header.getBoundingClientRect().right;
      });
      expect(overflowPx, `toolbar overflows its header at ${width}px`).toBeLessThanOrEqual(1);
    }
  } finally {
    await server.dispose();
  }
});
