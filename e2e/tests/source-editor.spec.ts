import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';

test('should edit source content and see the change reflected in the reader', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    // Open detail page then navigate to source editor
    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await page.getByRole('button', { name: 'Edit Sources' }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+\/sources$/);

    // Wait for the source to finish loading
    await expect(page.getByText('No pending changes').first()).toBeVisible();

    const textarea = page.locator('.source-content-textarea');
    await expect(textarea).toBeEnabled();

    // Append unique content to the existing source
    const original = await textarea.inputValue();
    await textarea.fill(`${original}\nEdited by E2E source editor.`);

    // Status should flip to "Unsaved changes" and Save button should enable
    await expect(page.getByText('Unsaved changes').first()).toBeVisible();
    const saveButton = page.getByRole('button', { name: 'Save*' });
    await expect(saveButton).toBeEnabled();
    await saveButton.click();

    // After save the topbar shows "Source saved."
    await expect(page.getByText('Source saved.')).toBeVisible();

    // Go back to the detail page, then open the reader
    await page.getByRole('button', { name: 'Back' }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await page.getByRole('button', { name: 'Read' }).click();
    await expect(page).toHaveURL(/\/reader\/[^/]+$/);

    // Reader should display the appended line
    await expect(page.getByText('Edited by E2E source editor.')).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('should create a new source, set it as current, and see its content in the reader', async ({
  page
}) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    // Open source editor
    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await page.getByRole('button', { name: 'Edit Sources' }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+\/sources$/);

    // Wait for initial source to load
    await expect(page.getByText('No pending changes').first()).toBeVisible();

    // Create a new source
    const newBtn = page.getByRole('button', { name: 'New' });
    await newBtn.click();

    // Wait for the entire creation cycle to settle:
    //   - "New" button re-enabled means creating=false and loadSource finished
    //   - textarea enabled and empty confirms the new source is active
    //   - "No pending changes" confirms isDirty=false (content===initialContent==='')
    await expect(newBtn).toBeEnabled();
    const textarea = page.locator('.source-content-textarea');
    await expect(textarea).toBeEnabled();
    await expect(textarea).toHaveValue('');
    await expect(page.getByText('No pending changes').first()).toBeVisible();

    // Type unique content; use keyboard.type so Vue's controlled :value binding
    // stays in sync across individual input events rather than a single bulk fill
    await textarea.click();
    await page.keyboard.type('This is the second source.');

    // Save the new source
    await expect(page.getByText('Unsaved changes').first()).toBeVisible();
    const saveButton = page.getByRole('button', { name: 'Save*' });
    await expect(saveButton).toBeEnabled();
    await saveButton.click();
    await expect(page.getByText('Source saved.')).toBeVisible();

    // The server may or may not auto-set a newly created source as current.
    // Click "Set as current" only if it is present; otherwise proceed directly.
    if (await page.getByRole('button', { name: 'Set as current' }).isVisible()) {
      await page.getByRole('button', { name: 'Set as current' }).click();
      await expect(page.getByText('Current source updated.')).toBeVisible();
    }

    // Open the reader and verify the new source is rendered
    await page.getByRole('button', { name: 'Back' }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await page.getByRole('button', { name: 'Read' }).click();
    await expect(page).toHaveURL(/\/reader\/[^/]+$/);

    await expect(page.getByText('This is the second source.')).toBeVisible();
  } finally {
    await server.dispose();
  }
});
