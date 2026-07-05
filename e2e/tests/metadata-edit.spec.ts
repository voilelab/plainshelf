import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';

test('should edit book metadata and see the updated values on the detail page', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);

    await page.getByRole('button', { name: 'Edit metadata' }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+\/edit$/);

    // Fill metadata fields
    const titleInput = page.getByPlaceholder('Book title');
    await titleInput.fill('Updated Hello');

    const authorsInput = page.getByPlaceholder('Author A, Author B');
    await authorsInput.fill('Alice, Bob');

    // Select English from the reka-ui language combobox (scope to the edit form
    // to avoid the sidebar UI-language select, which also has a "Language" label).
    // reka-ui's Select renders a button[role=combobox] trigger plus a portaled
    // listbox, so we open it and click the matching option instead of the native
    // <select>-only selectOption() API.
    const languageTrigger = page.locator('.edit-form').getByLabel('Language');
    await languageTrigger.click();
    await page.getByRole('option', { name: '英文' }).click();
    await expect(languageTrigger).toHaveText('英文');

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

    // Should redirect to detail page with saved=1
    await expect(page).toHaveURL(/\/books\/[^/]+\?saved=1$/);
    await expect(page.getByText('Metadata saved.')).toBeVisible();

    // Verify updated values are visible on the detail page
    await expect(page.getByRole('heading', { name: 'Updated Hello' })).toBeVisible();
    await expect(page.getByText('Alice, Bob')).toBeVisible();
    await expect(page.getByText('e2e-tag')).toBeVisible();
    await expect(page.getByText('英文')).toBeVisible();
    await expect(page.getByText('★★★★☆')).toBeVisible();
    await expect(page.getByText('This is an E2E comment.')).toBeVisible();
  } finally {
    await server.dispose();
  }
});
