import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { anotherFixturePath, helloFixturePath, importBookAs } from './support/books';
import { addFolder, emptyDataTransfer, folderRow, selectAllBooks, selectFolder } from './support/folders';

const getServer = useServer();

test('selects the current page and moves the selected books through a task chain', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'ms-move-hello');
  await importBookAs(page, anotherFixturePath, 'ms-move-another');
  await addFolder(page, 'ms-move-target');
  await selectAllBooks(page);

  await page.getByLabel('Select ms-move-hello', { exact: true }).check({ force: true });
  await page.getByLabel('Select ms-move-another', { exact: true }).check({ force: true });
  const selectionToolbar = page.getByRole('toolbar', { name: 'Selected books actions' });
  await expect(selectionToolbar.getByText('2 selected')).toBeVisible();

  await selectionToolbar.getByRole('button', { name: 'Move', exact: true }).click();
  const moveDialog = page.getByRole('dialog', { name: 'Move selected books' });
  await moveDialog.getByLabel('Destination').selectOption('ms-move-target');
  await moveDialog.getByRole('button', { name: 'Move 2 books', exact: true }).click();

  const progressDialog = page.getByRole('dialog', { name: 'Move selected books' });
  await expect(progressDialog.getByText('2 books completed.')).toBeVisible();
  await progressDialog.getByRole('button', { name: 'Close', exact: true }).click();
  await expect(selectionToolbar).not.toBeVisible();

  await selectFolder(page, 'ms-move-target');
  await expect(page.getByRole('heading', { name: 'ms-move-hello', exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'ms-move-another', exact: true })).toBeVisible();
});

test('keeps selection across view modes and clears it with Escape', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'ms-view-hello');
  await importBookAs(page, anotherFixturePath, 'ms-view-another');
  // Toggle both books individually with Meta rather than a Shift range: the
  // shelf is shared, so other tests' books can sit between these two in the
  // list and a range would sweep them in. Two explicit picks select exactly
  // these two, which is what this test then carries across view modes.
  await page.locator('.book-list-row', { hasText: 'ms-view-hello' }).click({ modifiers: ['Meta'] });
  await page.locator('.book-list-row', { hasText: 'ms-view-another' }).click({ modifiers: ['Meta'] });
  await expect(page.getByRole('toolbar', { name: 'Selected books actions' }).getByText('2 selected')).toBeVisible();

  await page.getByRole('button', { name: 'List', exact: true }).click();
  await page.getByRole('menuitemradio', { name: 'Card', exact: true }).click();
  await expect(page.locator('.book-card-view[aria-selected="true"]')).toHaveCount(2);

  await page.keyboard.press('Escape');
  await expect(page.getByRole('toolbar', { name: 'Selected books actions' })).not.toBeVisible();
  await expect(page).toHaveURL(/\/books/);
});

test('keeps selectable title rows keyboard-operable', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'ms-keyboard-book');

  await page.getByRole('button', { name: 'List', exact: true }).click();
  await page.getByRole('menuitemradio', { name: 'Title', exact: true }).click();
  const row = page.locator('.book-title-row', { hasText: 'ms-keyboard-book' });
  await row.focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/books\/[^/]+$/);

  await page.goBack();
  await expect(row).toBeVisible();
  await row.focus();
  await page.keyboard.press('Space');
  await expect(page).toHaveURL(/\/books\/[^/]+$/);
});

test('drags an unselected book alone and a selected group through the batch worker', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'ms-batch-hello');
  await importBookAs(page, anotherFixturePath, 'ms-batch-another');
  await addFolder(page, 'ms-batch-target');
  await selectAllBooks(page);

  await page.getByLabel('Select ms-batch-hello', { exact: true }).check({ force: true });
  const selectionToolbar = page.getByRole('toolbar', { name: 'Selected books actions' });
  const targetRow = folderRow(page, 'ms-batch-target');
  const anotherRow = page.locator('.book-list-row', { hasText: 'ms-batch-another' });

  const singleTransfer = await emptyDataTransfer(page);
  try {
    await anotherRow.dispatchEvent('dragstart', { dataTransfer: singleTransfer });
    await targetRow.dispatchEvent('dragenter', { dataTransfer: singleTransfer });
    await targetRow.dispatchEvent('dragover', { dataTransfer: singleTransfer });
    await targetRow.dispatchEvent('drop', { dataTransfer: singleTransfer });
  } finally {
    await singleTransfer.dispose();
  }

  await expect(targetRow.locator('.sidebar-nav-count')).toHaveText('1');
  await expect(selectionToolbar.getByText('1 selected')).toBeVisible();

  await page.getByLabel('Select ms-batch-another', { exact: true }).check({ force: true });
  const helloRow = page.locator('.book-list-row', { hasText: 'ms-batch-hello' });
  const groupTransfer = await emptyDataTransfer(page);
  try {
    await helloRow.dispatchEvent('dragstart', { dataTransfer: groupTransfer });
    await targetRow.dispatchEvent('dragenter', { dataTransfer: groupTransfer });
    await targetRow.dispatchEvent('dragover', { dataTransfer: groupTransfer });
    await targetRow.dispatchEvent('drop', { dataTransfer: groupTransfer });
  } finally {
    await groupTransfer.dispose();
  }

  const progressDialog = page.getByRole('dialog', { name: 'Move selected books' });
  await expect(progressDialog.getByText('2 books completed.')).toBeVisible();
  await progressDialog.getByRole('button', { name: 'Close', exact: true }).click();
  await expect(selectionToolbar).not.toBeVisible();
  await expect(targetRow.locator('.sidebar-nav-count')).toHaveText('2');
});
