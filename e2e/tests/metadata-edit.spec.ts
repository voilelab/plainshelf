import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { helloFixturePath, importBookAs } from './support/books';

const getServer = useServer();

test('should edit book metadata and see the updated values on the detail page', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  // The shelf is shared across the file, so import under a unique title rather
  // than the fixed "hello" name.
  await importBookAs(page, helloFixturePath, 'metadata-edit-book');

  await page.locator('.book-list-row').getByRole('heading', { name: 'metadata-edit-book', exact: true }).click();
  await expect(page).toHaveURL(/\/books\/[^/]+$/);
  const detailURL = page.url();
  const historyLength = await page.evaluate(() => window.history.length);

  await page.getByRole('button', { name: 'More' }).click();
  await page.getByRole('menuitem', { name: 'Edit book details' }).click();
  await expect(page).toHaveURL(detailURL);
  await expect(page.getByRole('dialog', { name: 'Edit metadata' })).toBeVisible();

  // Fill metadata fields
  const titleInput = page.getByPlaceholder('Book title');
  await titleInput.fill('metadata-edit-updated');

  const authorsInput = page.getByPlaceholder('Author A, Author B');
  await authorsInput.fill('Alice, Bob');

  // Select English from the reka-ui language combobox (scope to the edit form
  // to avoid the sidebar UI-language select, which also has a "Language" label).
  // reka-ui's Select renders a button[role=combobox] trigger plus a portaled
  // listbox, so we open it and click the matching option instead of the native
  // <select>-only selectOption() API.
  const languageTrigger = page.locator('.edit-form').getByLabel('Language');
  await languageTrigger.click();
  await page.getByRole('option', { name: 'English' }).click();
  await expect(languageTrigger).toHaveText('English');

  // Format belongs to the source and is intentionally absent from general
  // book metadata editing. Conversions live in Manage sources.
  await expect(page.locator('.edit-form').getByLabel('Format')).toHaveCount(0);

  // Add a tag
  const tagInput = page.getByPlaceholder('Type a tag and press Enter');
  await tagInput.fill('e2e-tag');
  await tagInput.press('Enter');
  await expect(page.getByText('e2e-tag')).toBeVisible();

  // Fill comment
  await page.getByPlaceholder('Notes about this book').fill('This is an E2E comment.');

  // Select a star rating
  await page.getByRole('radio', { name: '4 stars' }).click();

  await page.getByRole('button', { name: 'Save metadata' }).click();

  // Saving closes the modal in place: neither the route nor browser history
  // changes, and the detail view receives the updated Book response directly.
  await expect(page).toHaveURL(detailURL);
  await expect(page.getByRole('dialog', { name: 'Edit metadata' })).toBeHidden();
  expect(await page.evaluate(() => window.history.length)).toBe(historyLength);
  await expect(page.getByText('Book details saved.')).toBeVisible();

  // Verify updated values are visible on the detail page
  await expect(page.getByRole('heading', { name: 'metadata-edit-updated' })).toBeVisible();
  await expect(page.getByText('Alice, Bob')).toBeVisible();
  await expect(page.getByText('e2e-tag')).toBeVisible();
  // Scoped to the detail card: the book's language now reads "English" in an
  // English UI, which is also what the topbar's UI-language switcher shows.
  await expect(page.getByRole('article').getByText('English')).toBeVisible();
  await expect(page.getByText('TXT', { exact: true })).toBeVisible();
  await expect(page.getByText('★★★★☆')).toBeVisible();
  await expect(page.getByText('This is an E2E comment.')).toBeVisible();
});
