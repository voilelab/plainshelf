import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { helloFixturePath, importBookAs } from './support/books';

const getServer = useServer();

test('should edit book metadata and see the updated values on the detail page', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'metadata-edit-book');

  await page.locator('.book-list-row').getByRole('heading', { name: 'metadata-edit-book', exact: true }).click();
  await expect(page).toHaveURL(/\/books\/[^/]+$/);
  const detailURL = page.url();
  const historyLength = await page.evaluate(() => window.history.length);

  await page.getByRole('button', { name: 'More' }).click();
  await page.getByRole('menuitem', { name: 'Edit book details' }).click();
  await expect(page).toHaveURL(detailURL);
  await expect(page.getByRole('dialog', { name: 'Edit metadata' })).toBeVisible();

  const titleInput = page.getByPlaceholder('Book title');
  await titleInput.fill('metadata-edit-updated');

  await page.goBack();
  await expect(page.getByRole('dialog', { name: 'Discard unsaved changes?' })).toBeVisible();
  await page.getByRole('button', { name: 'Keep editing' }).click();
  await expect(page).toHaveURL(detailURL);
  await expect(titleInput).toHaveValue('metadata-edit-updated');

  const authorsInput = page.getByPlaceholder('Author A, Author B');
  await authorsInput.fill('Alice, Bob');

  // Reka UI 2.10.5 SelectTrigger renders a button with role combobox; Playwright 1.62.1 selectOption requires a native select.
  const languageTrigger = page.locator('.edit-form').getByLabel('Language');
  await languageTrigger.click();
  await page.getByRole('option', { name: 'English' }).click();
  await expect(languageTrigger).toHaveText('English');

  await expect(page.locator('.edit-form').getByLabel('Format')).toHaveCount(0);

  const tagInput = page.getByPlaceholder('Type a tag and press Enter');
  await tagInput.fill('e2e-tag');
  await tagInput.press('Enter');
  await expect(page.getByText('e2e-tag')).toBeVisible();

  await page.getByPlaceholder('Notes about this book').fill('This is an E2E comment.');

  await page.getByRole('radio', { name: '4 stars' }).click();

  await page.getByRole('button', { name: 'Save metadata' }).click();

  await expect(page).toHaveURL(detailURL);
  await expect(page.getByRole('dialog', { name: 'Edit metadata' })).toBeHidden();
  expect(await page.evaluate(() => window.history.length)).toBe(historyLength);
  await expect(page.getByText('Book details saved.')).toBeVisible();

  await expect(page.getByRole('heading', { name: 'metadata-edit-updated' })).toBeVisible();
  await expect(page.getByText('Alice, Bob')).toBeVisible();
  await expect(page.getByText('e2e-tag')).toBeVisible();
  await expect(page.getByRole('article').getByText('English')).toBeVisible();
  await expect(page.getByText('TXT', { exact: true })).toBeVisible();
  await expect(page.getByText('★★★★☆')).toBeVisible();
  await expect(page.getByText('This is an E2E comment.')).toBeVisible();
});
