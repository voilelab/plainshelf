import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { anotherFixturePath, helloFixturePath, importBookAs } from './support/books';
import {
  addFolder,
  emptyDataTransfer,
  folderRow,
  foldersQueryRegex,
  foldersNav,
  foldersSectionToggle,
  openCreateFolderDialog,
  openFolderContextMenu,
  selectAllBooks,
  selectFolder,
  switchToCardView
} from './support/folders';

const getServer = useServer();

test('creates a nested folder level by level and filters books by exact folder', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

  await addFolder(page, 'ftnested/ftnested-scifi');

  await expect(folderRow(page, 'ftnested')).toBeVisible();
  await expect(folderRow(page, 'ftnested-scifi')).toBeVisible();

  await selectFolder(page, 'ftnested-scifi');
  await expect(page).toHaveURL(foldersQueryRegex('ftnested/ftnested-scifi'));

  await importBookAs(page, helloFixturePath, 'ft-nested-book');

  await selectAllBooks(page);
  await expect(page).not.toHaveURL(/[?&]folders=/);
  await expect(
    page.locator('.book-list-row').getByRole('heading', { name: 'ft-nested-book', exact: true })
  ).toBeVisible();

  // "ftnested" itself has no directly-attached books (only its "ftnested-scifi" child does).
  await selectFolder(page, 'ftnested');
  await expect(page.getByText('No books in ftnested.')).toBeVisible();
});

test('returns to the folder the book lived in after moving it to trash', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await addFolder(page, 'fttrash/fttrash-scifi');
  await expect(page).toHaveURL(foldersQueryRegex('fttrash/fttrash-scifi'));

  await importBookAs(page, helloFixturePath, 'ft-trash-book');
  await page.locator('.book-list-row').getByRole('heading', { name: 'ft-trash-book', exact: true }).click();
  await expect(page).toHaveURL(/\/books\/[^/]+$/);

  await page.getByRole('button', { name: 'More' }).click();
  await page.getByRole('menuitem', { name: 'Move to Trash' }).click();
  const deleteDialog = page.getByRole('dialog', { name: 'Confirm delete' });
  await expect(deleteDialog).toBeVisible();
  await deleteDialog.getByRole('button', { name: 'Delete', exact: true }).click();

  // Back to the book's own folder, not the unfiltered library. This folder holds
  // only this test's book, so after trashing it the folder-scoped count is 0; the
  // folder is also proven by the URL and the page heading.
  await expect(page).toHaveURL(foldersQueryRegex('fttrash/fttrash-scifi'));
  await expect(page.getByRole('heading', { name: 'fttrash/fttrash-scifi', exact: true })).toBeVisible();
  await expect(page.getByText('0 books', { exact: true })).toBeVisible();
});

test('the root folder node filters to books sitting directly at the shelf root', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'ft-root-hello');

  await addFolder(page, 'ftroot');
  await importBookAs(page, anotherFixturePath, 'ft-root-another');

  await selectAllBooks(page);
  // The shelf is shared, so a whole-library count is not deterministic; assert
  // both of this test's books are present in the unfiltered view instead.
  await expect(
    page.locator('.book-list-row').getByRole('heading', { name: 'ft-root-hello', exact: true })
  ).toBeVisible();
  await expect(
    page.locator('.book-list-row').getByRole('heading', { name: 'ft-root-another', exact: true })
  ).toBeVisible();

  // "/" is a narrower filter than "All books": it keeps the root-level book
  // only, so its folders query must survive rather than collapsing to no query.
  await selectFolder(page, '/');
  await expect(page).toHaveURL(foldersQueryRegex('/'));
  // ft-root-hello sits at the shelf root and stays; ft-root-another lives in
  // ftroot and is excluded. Other tests may leave root-level books too, so this
  // asserts on these two rows rather than a root-level count.
  await expect(
    page.locator('.book-list-row').getByRole('heading', { name: 'ft-root-hello', exact: true })
  ).toBeVisible();
  await expect(
    page.locator('.book-list-row').getByRole('heading', { name: 'ft-root-another', exact: true })
  ).toHaveCount(0);
});

test('the new folder dialog rejects a name containing a separator and cancels cleanly', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

  const dialog = await openCreateFolderDialog(page, '/');
  const nameInput = dialog.getByLabel('Folder name');
  const createButton = dialog.getByRole('button', { name: 'Create', exact: true });
  await expect(createButton).toBeDisabled();

  await nameInput.fill('ftreject/scifi');
  await expect(dialog.getByRole('alert')).toHaveText('Folder name cannot be empty or contain /.');
  await expect(createButton).toBeDisabled();

  await nameInput.fill('ftreject');
  await expect(dialog.getByRole('alert')).toHaveCount(0);
  await expect(createButton).toBeEnabled();

  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(dialog).not.toBeVisible();
  await expect(page).not.toHaveURL(/[?&]folders=/);
  await expect(folderRow(page, 'ftreject')).toHaveCount(0);
});

test('expands and collapses a folder that has children', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await addFolder(page, 'ftexpand/ftexpand-child');

  const novelsRow = folderRow(page, 'ftexpand');
  const scifiRow = folderRow(page, 'ftexpand-child');
  await expect(scifiRow).toBeVisible();

  await novelsRow.getByRole('button', { name: 'Collapse folder', exact: true }).click();
  await expect(scifiRow).toHaveCount(0);

  await novelsRow.getByRole('button', { name: 'Expand folder', exact: true }).click();
  await expect(scifiRow).toBeVisible();
});

test('renames a folder, updating the tree and the active URL filter', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await addFolder(page, 'ftrename');
  await selectFolder(page, 'ftrename');
  await expect(page).toHaveURL(foldersQueryRegex('ftrename'));

  await openFolderContextMenu(page, 'ftrename');
  await page.getByRole('menuitem', { name: 'Rename', exact: true }).click();
  const renameDialog = page.getByRole('dialog', { name: 'Rename folder' });
  await expect(renameDialog).toBeVisible();
  await renameDialog.getByLabel('Folder name').fill('ftrenamed');
  await renameDialog.getByRole('button', { name: 'Rename', exact: true }).click();

  await expect(renameDialog).not.toBeVisible();
  await expect(folderRow(page, 'ftrename')).toHaveCount(0);
  await expect(folderRow(page, 'ftrenamed')).toBeVisible();
  await expect(page).toHaveURL(foldersQueryRegex('ftrenamed'));
});

test('only offers Delete for empty folders, and deleting removes it from the tree', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);

  await addFolder(page, 'ftwithbook');
  await selectFolder(page, 'ftwithbook');
  await importBookAs(page, helloFixturePath, 'ft-withbook-book');

  await openFolderContextMenu(page, 'ftwithbook');
  await expect(page.getByRole('menuitem', { name: 'Rename', exact: true })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: 'Delete', exact: true })).toHaveCount(0);
  await page.keyboard.press('Escape');

  await addFolder(page, 'ftremovable');
  await openFolderContextMenu(page, 'ftremovable');
  await page.getByRole('menuitem', { name: 'Delete', exact: true }).click();

  const deleteDialog = page.getByRole('dialog', { name: 'Delete folder' });
  await expect(deleteDialog).toBeVisible();
  await deleteDialog.getByRole('button', { name: 'Delete', exact: true }).click();

  await expect(deleteDialog).not.toBeVisible();
  await expect(folderRow(page, 'ftremovable')).toHaveCount(0);
});

test('moves a book into a folder via drag and drop', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'ft-dnd-book');

  await addFolder(page, 'ftreading');
  await selectFolder(page, 'ftreading');
  await expect(page.getByText('No books in ftreading.')).toBeVisible();

  await selectAllBooks(page);
  await switchToCardView(page);

  const bookCard = page.locator('.book-card-view').filter({
    has: page.getByRole('heading', { name: 'ft-dnd-book', exact: true })
  });
  await expect(bookCard).toBeVisible();

  const dataTransfer = await emptyDataTransfer(page);
  try {
    await bookCard.dispatchEvent('dragstart', { dataTransfer });
    const readingRow = folderRow(page, 'ftreading');
    await readingRow.dispatchEvent('dragenter', { dataTransfer });
    await readingRow.dispatchEvent('dragover', { dataTransfer });
    await readingRow.dispatchEvent('drop', { dataTransfer });
  } finally {
    await dataTransfer.dispose();
  }

  await selectFolder(page, 'ftreading');
  await expect(
    page.locator('.book-card-view').getByRole('heading', { name: 'ft-dnd-book', exact: true })
  ).toBeVisible();
  // Folder-scoped count: only this test's book was moved into ftreading.
  await expect(page.getByText('1 books')).toBeVisible();

  await selectAllBooks(page);
  await expect(
    page.locator('.book-card-view').getByRole('heading', { name: 'ft-dnd-book', exact: true })
  ).toBeVisible();
});

test('moves a top-level folder under another folder via drag and drop', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await addFolder(page, 'ftmoveA');
  await addFolder(page, 'ftmoveB');

  const bRowBefore = folderRow(page, 'ftmoveB');
  await expect(bRowBefore).toBeVisible();
  await expect(bRowBefore).toHaveCSS('padding-left', '8px');

  const dataTransfer = await emptyDataTransfer(page);
  try {
    await bRowBefore.dispatchEvent('dragstart', { dataTransfer });
    const aRow = folderRow(page, 'ftmoveA');
    await aRow.dispatchEvent('dragenter', { dataTransfer });
    await aRow.dispatchEvent('dragover', { dataTransfer });
    await aRow.dispatchEvent('drop', { dataTransfer });
  } finally {
    await dataTransfer.dispose();
  }

  const bRowAfter = folderRow(page, 'ftmoveB');
  await expect(bRowAfter).toHaveCSS('padding-left', '22px');

  await bRowAfter.getByRole('button', { name: 'ftmoveB', exact: true }).click();
  await expect(page).toHaveURL(foldersQueryRegex('ftmoveA/ftmoveB'));
});

test('the new folder dialog asks only for a name', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

  const dialog = await openCreateFolderDialog(page, '/');
  await expect(dialog.getByLabel('Folder name')).toBeVisible();
  await expect(dialog.getByRole('combobox')).toHaveCount(0);
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(dialog).not.toBeVisible();
});

test('disables folder creation while the folder list is unavailable', async ({ page }) => {
  const { baseUrl } = getServer();

  // Fail only the folder listing, so the rest of the shell still loads.
  await page.route('**/api/shelves/*/folders', (route) =>
    route.request().method() === 'GET' ? route.fulfill({ status: 500, body: 'boom' }) : route.fallback()
  );

  await page.goto(`${baseUrl}/books`);

  // Not expandFoldersSection(): that waits for the Folders nav, which FolderTree
  // never renders while the fetch is failing. Expand the section directly.
  const sectionToggle = foldersSectionToggle(page);
  if ((await sectionToggle.getAttribute('aria-expanded')) === 'false') {
    await sectionToggle.click();
  }

  // The sidebar refuses to show the tree, so there is no parent context menu
  // from which creation can be started.
  await expect(page.getByRole('button', { name: 'Retry', exact: true })).toBeVisible();
  await expect(foldersNav(page)).toHaveCount(0);
  await expect(page.getByRole('menuitem', { name: 'Add folder', exact: true })).toHaveCount(0);

  await page.unroute('**/api/shelves/*/folders');
  await page.getByRole('button', { name: 'Retry', exact: true }).click();

  const dialog = await openCreateFolderDialog(page, '/');
  await expect(dialog).toBeVisible();
});
