import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { helloFixturePath, importBookAs } from './support/books';

const getServer = useServer();

test('should move a book to trash and restore it back to the library', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'trash-restore-book');

  const row = page.locator('.book-list-row').getByRole('heading', { name: 'trash-restore-book', exact: true });
  await row.click();
  await expect(page).toHaveURL(/\/books\/[^/]+$/);

  await page.getByRole('button', { name: 'More' }).click();
  await page.getByRole('menuitem', { name: 'Move to Trash' }).click();
  const deleteDialog = page.getByRole('alertdialog', { name: 'Confirm delete' });
  await expect(deleteDialog).toBeVisible();
  await deleteDialog.getByRole('button', { name: 'Delete', exact: true }).click();

  await expect(page).toHaveURL(/\/books(\?|$)/);
  await expect(
    page.locator('.book-list-row').getByRole('heading', { name: 'trash-restore-book', exact: true })
  ).toHaveCount(0);

  await page.goto(`${baseUrl}/trash`);
  await expect(page.getByRole('heading', { name: 'Trash' })).toBeVisible();
  const trashRow = page.getByRole('row').filter({
    has: page.getByRole('cell', { name: 'trash-restore-book', exact: true })
  });
  await expect(trashRow).toBeVisible();

  await trashRow.getByRole('button', { name: 'Restore' }).click();
  await expect(page.getByRole('cell', { name: 'trash-restore-book', exact: true })).toHaveCount(0);

  await page.goto(`${baseUrl}/books`);
  await expect(
    page.locator('.book-list-row').getByRole('heading', { name: 'trash-restore-book', exact: true })
  ).toBeVisible();
});
