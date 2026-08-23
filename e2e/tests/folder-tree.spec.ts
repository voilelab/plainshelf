import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { anotherFixturePath, helloFixturePath, importBookFromPath } from './support/books';
import {
  addFolder,
  emptyDataTransfer,
  expandFoldersSection,
  folderRow,
  foldersQueryRegex,
  foldersSectionToggle,
  openFolderContextMenu,
  selectAllBooks,
  selectFolder,
  switchToCardView
} from './support/folders';

test('creates a nested folder level by level and filters books by exact folder', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

    await addFolder(page, 'novels/scifi');

    await expect(folderRow(page, 'novels')).toBeVisible();
    await expect(folderRow(page, 'scifi')).toBeVisible();

    await selectFolder(page, 'scifi');
    await expect(page).toHaveURL(foldersQueryRegex('novels/scifi'));

    await importBookFromPath(page, helloFixturePath);
    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true })
    ).toBeVisible();

    await selectAllBooks(page);
    await expect(page).not.toHaveURL(/[?&]folders=/);
    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true })
    ).toBeVisible();

    // "novels" itself has no directly-attached books (only its "scifi" child does).
    await selectFolder(page, 'novels');
    await expect(page.getByText('No books in novels.')).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('returns to the folder the book lived in after moving it to trash', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await addFolder(page, 'novels/scifi');
    await expect(page).toHaveURL(foldersQueryRegex('novels/scifi'));

    await importBookFromPath(page, helloFixturePath);
    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);

    await page.getByRole('button', { name: 'More' }).click();
    await page.getByRole('menuitem', { name: 'Move to Trash' }).click();
    const deleteDialog = page.getByRole('dialog', { name: 'Confirm delete' });
    await expect(deleteDialog).toBeVisible();
    await deleteDialog.getByRole('button', { name: 'Delete', exact: true }).click();

    // Back to the book's own folder, not the unfiltered library. The library is
    // empty afterwards, so the folder is proven by the URL and the page heading
    // rather than by the "No books in <folder>." empty state.
    await expect(page).toHaveURL(foldersQueryRegex('novels/scifi'));
    await expect(page.getByRole('heading', { name: 'novels/scifi', exact: true })).toBeVisible();
    await expect(page.getByText('0 books', { exact: true })).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('the root folder node filters to books sitting directly at the shelf root', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importBookFromPath(page, helloFixturePath);

    await addFolder(page, 'novels');
    await importBookFromPath(page, anotherFixturePath);

    await selectAllBooks(page);
    await expect(page.getByText('2 books', { exact: true })).toBeVisible();

    // "/" is a narrower filter than "All books": it keeps the root-level book
    // only, so its folders query must survive rather than collapsing to no query.
    await selectFolder(page, '/');
    await expect(page).toHaveURL(foldersQueryRegex('/'));
    await expect(page.getByText('1 books', { exact: true })).toBeVisible();
    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true })
    ).toBeVisible();
    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'another', exact: true })
    ).toHaveCount(0);
  } finally {
    await server.dispose();
  }
});

test('the new folder dialog rejects a name containing a separator and cancels cleanly', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

    await expandFoldersSection(page);
    await page.getByRole('button', { name: 'Add folder', exact: true }).click();

    const dialog = page.getByRole('dialog', { name: 'New folder' });
    const nameInput = dialog.getByLabel('Folder name');
    const createButton = dialog.getByRole('button', { name: 'Create', exact: true });
    await expect(createButton).toBeDisabled();

    await nameInput.fill('novels/scifi');
    await expect(dialog.getByRole('alert')).toHaveText('Folder name cannot be empty or contain /.');
    await expect(createButton).toBeDisabled();

    await nameInput.fill('novels');
    await expect(dialog.getByRole('alert')).toHaveCount(0);
    await expect(createButton).toBeEnabled();

    await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
    await expect(dialog).not.toBeVisible();
    await expect(page).not.toHaveURL(/[?&]folders=/);
    await expect(folderRow(page, 'novels')).toHaveCount(0);
  } finally {
    await server.dispose();
  }
});

test('expands and collapses a folder that has children', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await addFolder(page, 'novels/scifi');

    const novelsRow = folderRow(page, 'novels');
    const scifiRow = folderRow(page, 'scifi');
    await expect(scifiRow).toBeVisible();

    await novelsRow.getByRole('button', { name: 'Collapse folder', exact: true }).click();
    await expect(scifiRow).toHaveCount(0);

    await novelsRow.getByRole('button', { name: 'Expand folder', exact: true }).click();
    await expect(scifiRow).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('renames a folder, updating the tree and the active URL filter', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await addFolder(page, 'temp');
    await selectFolder(page, 'temp');
    await expect(page).toHaveURL(foldersQueryRegex('temp'));

    await openFolderContextMenu(page, 'temp');
    await page.getByRole('menuitem', { name: 'Rename', exact: true }).click();
    const renameDialog = page.getByRole('dialog', { name: 'Rename folder' });
    await expect(renameDialog).toBeVisible();
    await renameDialog.getByLabel('Folder name').fill('renamed');
    await renameDialog.getByRole('button', { name: 'Rename', exact: true }).click();

    await expect(renameDialog).not.toBeVisible();
    await expect(folderRow(page, 'temp')).toHaveCount(0);
    await expect(folderRow(page, 'renamed')).toBeVisible();
    await expect(page).toHaveURL(foldersQueryRegex('renamed'));
  } finally {
    await server.dispose();
  }
});

test('only offers Delete for empty folders, and deleting removes it from the tree', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);

    await addFolder(page, 'withbook');
    await selectFolder(page, 'withbook');
    await importBookFromPath(page, helloFixturePath);

    await openFolderContextMenu(page, 'withbook');
    await expect(page.getByRole('menuitem', { name: 'Rename', exact: true })).toBeVisible();
    await expect(page.getByRole('menuitem', { name: 'Delete', exact: true })).toHaveCount(0);
    await page.keyboard.press('Escape');

    await addFolder(page, 'removable');
    await openFolderContextMenu(page, 'removable');
    await page.getByRole('menuitem', { name: 'Delete', exact: true }).click();

    const deleteDialog = page.getByRole('dialog', { name: 'Delete folder' });
    await expect(deleteDialog).toBeVisible();
    await deleteDialog.getByRole('button', { name: 'Delete', exact: true }).click();

    await expect(deleteDialog).not.toBeVisible();
    await expect(folderRow(page, 'removable')).toHaveCount(0);
  } finally {
    await server.dispose();
  }
});

test('moves a book into a folder via drag and drop', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importBookFromPath(page, helloFixturePath);
    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true })
    ).toBeVisible();

    await addFolder(page, 'reading');
    await selectFolder(page, 'reading');
    await expect(page.getByText('No books in reading.')).toBeVisible();

    await selectAllBooks(page);
    await switchToCardView(page);

    const bookCard = page.locator('.book-card-view').filter({
      has: page.getByRole('heading', { name: 'hello', exact: true })
    });
    await expect(bookCard).toBeVisible();

    const dataTransfer = await emptyDataTransfer(page);
    try {
      await bookCard.dispatchEvent('dragstart', { dataTransfer });
      const readingRow = folderRow(page, 'reading');
      await readingRow.dispatchEvent('dragenter', { dataTransfer });
      await readingRow.dispatchEvent('dragover', { dataTransfer });
      await readingRow.dispatchEvent('drop', { dataTransfer });
    } finally {
      await dataTransfer.dispose();
    }

    await selectFolder(page, 'reading');
    await expect(
      page.locator('.book-card-view').getByRole('heading', { name: 'hello', exact: true })
    ).toBeVisible();
    await expect(page.getByText('1 books')).toBeVisible();

    await selectAllBooks(page);
    await expect(
      page.locator('.book-card-view').getByRole('heading', { name: 'hello', exact: true })
    ).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('moves a top-level folder under another folder via drag and drop', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await addFolder(page, 'A');
    await addFolder(page, 'B');

    const bRowBefore = folderRow(page, 'B');
    await expect(bRowBefore).toBeVisible();
    await expect(bRowBefore).toHaveCSS('padding-left', '8px');

    const dataTransfer = await emptyDataTransfer(page);
    try {
      await bRowBefore.dispatchEvent('dragstart', { dataTransfer });
      const aRow = folderRow(page, 'A');
      await aRow.dispatchEvent('dragenter', { dataTransfer });
      await aRow.dispatchEvent('dragover', { dataTransfer });
      await aRow.dispatchEvent('drop', { dataTransfer });
    } finally {
      await dataTransfer.dispose();
    }

    const bRowAfter = folderRow(page, 'B');
    await expect(bRowAfter).toHaveCSS('padding-left', '22px');

    await bRowAfter.getByRole('button', { name: 'B', exact: true }).click();
    await expect(page).toHaveURL(foldersQueryRegex('A/B'));
  } finally {
    await server.dispose();
  }
});

test('the new folder dialog scrolls its parent select instead of overflowing the page', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

    // Enough top-level folders that the option list exceeds the 320px cap.
    for (let i = 0; i < 10; i += 1) {
      await addFolder(page, `folder-${i}`);
    }

    await expandFoldersSection(page);
    await page.getByRole('button', { name: 'Add folder', exact: true }).click();

    const dialog = page.getByRole('dialog', { name: 'New folder' });
    await dialog.getByRole('combobox').click();

    const listbox = page.getByRole('listbox');
    await expect(listbox).toBeVisible();

    const box = await listbox.boundingBox();
    if (!box) {
      throw new Error('expected the parent select listbox to have a bounding box');
    }

    const viewportSize = page.viewportSize();
    if (!viewportSize) {
      throw new Error('expected a viewport size');
    }

    // The height cap holds, and the menu stays fully on screen.
    expect(box.height).toBeLessThanOrEqual(320);
    expect(box.y).toBeGreaterThanOrEqual(0);
    expect(box.y + box.height).toBeLessThanOrEqual(viewportSize.height);

    // The options really do overflow, so the list is genuinely scrollable.
    const scrollable = await page
      .locator('[data-reka-select-viewport]')
      .evaluate((el) => el.scrollHeight > el.clientHeight);
    expect(scrollable).toBe(true);

    // The last option is still reachable and selectable.
    await page.getByRole('option', { name: 'folder-9', exact: true }).click();
    await expect(dialog.getByRole('combobox')).toContainText('folder-9');
  } finally {
    await server.dispose();
  }
});

test('disables folder creation while the folder list is unavailable', async ({ page }) => {
  const server = await startServer();

  try {
    // Fail only the folder listing, so the rest of the shell still loads.
    await page.route('**/api/shelves/*/folders', (route) =>
      route.request().method() === 'GET' ? route.fulfill({ status: 500, body: 'boom' }) : route.fallback()
    );

    await page.goto(`${server.baseUrl}/books`);

    // Not expandFoldersSection(): that waits for the Folders nav, which FolderTree
    // never renders while the fetch is failing. Expand the section directly.
    const sectionToggle = foldersSectionToggle(page);
    if ((await sectionToggle.getAttribute('aria-expanded')) === 'false') {
      await sectionToggle.click();
    }

    // The sidebar refuses to show the tree, so creation must be refused too --
    // otherwise the dialog would offer parent options from a list the sidebar
    // itself is declining to show (a previous shelf's, after a shelf switch).
    await expect(page.getByRole('button', { name: 'Retry', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Add folder', exact: true })).toBeDisabled();

    await page.unroute('**/api/shelves/*/folders');
    await page.getByRole('button', { name: 'Retry', exact: true }).click();

    const addFolderButton = page.getByRole('button', { name: 'Add folder', exact: true });
    await expect(addFolderButton).toBeEnabled();
    await addFolderButton.click();
    await expect(page.getByRole('dialog', { name: 'New folder' })).toBeVisible();
  } finally {
    await server.dispose();
  }
});
