import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { helloFixturePath, importBookAs } from './support/books';
import {
  addFolder,
  emptyDataTransfer,
  folderRow,
  foldersQueryRegex,
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

  const deleteDialog = page.getByRole('alertdialog', { name: 'Delete folder' });
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
