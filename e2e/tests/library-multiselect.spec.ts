import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { anotherFixturePath, helloFixturePath, importBookAs } from './support/books';
import { addFolder, selectAllBooks, selectFolder } from './support/folders';

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
