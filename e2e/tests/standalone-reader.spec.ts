import { promises as fs } from 'node:fs';
import os from 'node:os';
import path from 'node:path';

import { expect, test, type Page } from '@playwright/test';

import { importHelloBook } from './support/books';
import { snapshotTree, startReader } from './support/reader';
import { startServer } from './support/server';

/**
 * Seeds a shelf with the full server and returns a copy of it for the reader to
 * open.
 *
 * A copy rather than the server's own folder for two reasons: the server's
 * dispose removes its temp root, and a server left running alongside the reader
 * would keep writing to the shelf on its own timers — which is what the
 * untouched-folder assertion below is about, so it must not be another process
 * doing it.
 */
async function seedShelfCopy(page: Page): Promise<string> {
  const target = path.join(await fs.mkdtemp(path.join(os.tmpdir(), 'plainshelf-reader-')), 'shelf');

  const server = await startServer();
  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);
    await fs.cp(server.shelfDir, target, { recursive: true });
  } finally {
    // The temp root goes with the server, and removing it can lose a race with
    // the writes the server was still finishing — a known teardown-only failure
    // (see support/server.ts). The copy is already taken by then, so it is not
    // this spec's problem.
    await server.dispose().catch(() => undefined);
  }

  return target;
}

// The standalone reader binary is the whole product for someone who only wants
// to read a folder: no config file, no data directory, and no way for it to
// change what it was pointed at. This is the one spec that drives it as a user
// would — the frontend it serves has to discover for itself that its backend is
// a reader, and gate its own pages on the answer.
test('the standalone reader serves a shelf and leaves it alone', async ({ page }) => {
  const shelfDir = await seedShelfCopy(page);

  try {
    const before = await snapshotTree(shelfDir);
    const reader = await startReader(shelfDir);

    try {
      await page.goto(`${reader.baseUrl}/books`);

      // The library comes up against the folder the binary was given.
      await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
      const seededBook = page
        .locator('.book-list-row')
        .getByRole('heading', { name: 'hello', exact: true });
      await expect(seededBook).toBeVisible();

      // No import affordance: the reading server mounts no import route, and
      // the shell it installs reports itself as unwritable, so the whole
      // editing UI is absent rather than present and broken.
      await expect(page.getByRole('button', { name: /^Import/ })).toHaveCount(0);

      // A page the reading server does not serve is refused by the shell's
      // route policy rather than left to fail against a 404.
      await page.goto(`${reader.baseUrl}/trash`);
      await expect(page).toHaveURL(/\/books(\?|$)/);
      await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

      // Same for the query that opens the import modal on a route that stays
      // readable — /import redirects to /books?import=1, so the route name is
      // not enough to refuse it.
      await page.goto(`${reader.baseUrl}/books?import=1`);
      await expect(page.getByRole('dialog', { name: 'Import Book' })).toHaveCount(0);

      // Reading a book still works, which is the whole point.
      await page.goto(`${reader.baseUrl}/books`);
      await seededBook.click();
      await expect(page.getByRole('heading', { name: 'hello', exact: true })).toBeVisible();
    } finally {
      await reader.dispose();
    }

    // The acceptance case: a whole round of reading left the folder byte for
    // byte and mtime for mtime as it was found.
    expect(await snapshotTree(shelfDir)).toEqual(before);
  } finally {
    await fs.rm(path.dirname(shelfDir), { recursive: true, force: true });
  }
});
