import { expect, test } from '@playwright/test';
import { promises as fs } from 'node:fs';
import path from 'node:path';
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
  selectNoFolder,
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

  await addFolder(page, 'ftorder-a');
  await addFolder(page, 'ftorder-b');
  await openFolderContextMenu(page, 'ftorder-b');
  await page.getByRole('menuitem', { name: 'Move up', exact: true }).click();
  const orderLabels = () => page.locator('.folder-node > .sidebar-nav-item-label').allTextContents();
  await expect.poll(async () => {
    const labels = (await orderLabels()).map(label => label.trim());
    return labels.indexOf('ftorder-b') < labels.indexOf('ftorder-a');
  }).toBe(true);
  await page.reload();
  await expect(folderRow(page, 'ftorder-b')).toBeVisible();
  await expect.poll(async () => {
    const labels = (await orderLabels()).map(label => label.trim());
    return labels.indexOf('ftorder-b') < labels.indexOf('ftorder-a');
  }).toBe(true);


  await selectFolder(page, 'ftnested-scifi');
  await expect(page).toHaveURL(foldersQueryRegex('ftnested/ftnested-scifi'));

  await importBookAs(page, helloFixturePath, 'ft-nested-book');

  await selectAllBooks(page);
  await expect(page).not.toHaveURL(/[?&]folders=/);
  await expect(
    page.locator('.book-list-row').getByRole('heading', { name: 'ft-nested-book', exact: true })
  ).toBeVisible();

  await selectFolder(page, 'ftnested');
  await expect(page.getByText('No books in ftnested.')).toBeVisible();

  await selectAllBooks(page);
  await importBookAs(page, helloFixturePath, 'ft-loose-book');
  await selectNoFolder(page);
  await expect(page).toHaveURL(/[?&]folders=(?:\/|%2F)(?:&|$)/);
  await expect(page.getByRole('heading', { name: 'No folder', exact: true })).toBeVisible();
  const titles = page.locator('.book-list-row').getByRole('heading');
  await expect(titles.filter({ hasText: 'ft-loose-book' })).toBeVisible();
  await expect(titles.filter({ hasText: 'ft-nested-book' })).toHaveCount(0);
});

test('renames a folder, keeping its icon and updating the tree and the active URL filter', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await addFolder(page, 'ftrename');
  await selectFolder(page, 'ftrename');
  await expect(page).toHaveURL(foldersQueryRegex('ftrename'));
  await expect(folderRow(page, 'ftrename').locator('[data-folder-icon="folder"]')).toBeVisible();

  await openFolderContextMenu(page, 'ftrename');
  await page.getByRole('menuitem', { name: 'Change icon', exact: true }).click();
  const iconDialog = page.getByRole('dialog', { name: 'Icon for "ftrename"' });
  await iconDialog.getByRole('radio', { name: 'rocket', exact: true }).click();
  await iconDialog.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(iconDialog).not.toBeVisible();
  await expect(folderRow(page, 'ftrename').locator('[data-folder-icon="rocket"]')).toBeVisible();

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

  await page.reload();
  await expect(folderRow(page, 'ftrenamed').locator('[data-folder-icon="rocket"]')).toBeVisible();

  await openFolderContextMenu(page, 'ftrenamed');
  await page.getByRole('menuitem', { name: 'Change icon', exact: true }).click();
  const clearDialog = page.getByRole('dialog', { name: 'Icon for "ftrenamed"' });
  await clearDialog.getByRole('radio', { name: 'Default', exact: true }).click();
  await clearDialog.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(clearDialog).not.toBeVisible();
  await expect(folderRow(page, 'ftrenamed').locator('[data-folder-icon="folder"]')).toBeVisible();
});

test('only offers Delete for empty folders, and deleting removes it from the tree', async ({ page }) => {
  const { baseUrl, shelfDir } = getServer();

  await page.goto(`${baseUrl}/books`);

  await addFolder(page, 'ftwithbook');
  await selectFolder(page, 'ftwithbook');
  await importBookAs(page, helloFixturePath, 'ft-withbook-book');

  await openFolderContextMenu(page, 'ftwithbook');
  await expect(page.getByRole('menuitem', { name: 'Rename', exact: true })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: 'Delete', exact: true })).toHaveCount(0);
  await page.keyboard.press('Escape');

  await addFolder(page, 'ftremovable');
  const strayFile = path.join(shelfDir, 'books', 'ftremovable', '.gitkeep');
  await fs.writeFile(strayFile, '');
  await openFolderContextMenu(page, 'ftremovable');
  await page.getByRole('menuitem', { name: 'Delete', exact: true }).click();

  const deleteDialog = page.getByRole('alertdialog', { name: 'Delete folder' });
  await expect(deleteDialog).toBeVisible();
  await deleteDialog.getByRole('button', { name: 'Delete', exact: true }).click();
  await expect(deleteDialog.getByRole('alert')).toContainText('• .gitkeep');

  await fs.rm(strayFile);
  await deleteDialog.getByRole('button', { name: 'Delete', exact: true }).click();

  await expect(deleteDialog).not.toBeVisible();
  await expect(folderRow(page, 'ftremovable')).toHaveCount(0);
});

test('drags books and folders onto folders, and undoes a folder move', async ({ page }) => {
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
  await expect(page.getByText('1 books')).toBeVisible();

  await selectAllBooks(page);
  await expect(
    page.locator('.book-card-view').getByRole('heading', { name: 'ft-dnd-book', exact: true })
  ).toBeVisible();

  await addFolder(page, 'ftdrag-a');
  await addFolder(page, 'ftdrag-b');
  const orderLabels = async () =>
    (await page.locator('.folder-node > .sidebar-nav-item-label').allTextContents()).map((label) => label.trim());

  await folderRow(page, 'ftdrag-b').dragTo(folderRow(page, 'ftdrag-a'), { targetPosition: { x: 60, y: 2 } });
  await expect.poll(async () => {
    const labels = await orderLabels();
    return labels.indexOf('ftdrag-b') < labels.indexOf('ftdrag-a');
  }).toBe(true);

  await folderRow(page, 'ftdrag-b').dragTo(folderRow(page, 'ftdrag-a'), { targetPosition: { x: 60, y: 16 } });
  const notice = page.locator('.reka-toast').filter({ hasText: 'Moved “ftdrag-b” into “ftdrag-a”.' });
  await expect(notice).toBeVisible();
  await expect(folderRow(page, 'ftdrag-b')).toHaveAttribute('aria-level', '2');

  await notice.getByRole('button', { name: 'Undo', exact: true }).click();
  await expect(folderRow(page, 'ftdrag-b')).toHaveAttribute('aria-level', '1');
  await expect(notice).toHaveCount(0);
});
