import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';

// per-test server: this test's whole point is the empty-trash flow — it asserts
// the exact whole-trash count ("Permanently delete all 1 books in the trash?")
// and the terminal "The trash is now empty." state, both of which require a
// pristine shelf with nothing else in the trash. A shared shelf would let other
// books accumulate and break those counts, so it keeps its own per-test server.
test('should empty the trash through a background task and report its progress', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    // Move the book to the trash from its detail page.
    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await page.getByRole('button', { name: 'More' }).click();
    await page.getByRole('menuitem', { name: 'Move to Trash' }).click();
    const deleteDialog = page.getByRole('dialog', { name: 'Confirm delete' });
    await expect(deleteDialog).toBeVisible();
    await deleteDialog.getByRole('button', { name: 'Delete', exact: true }).click();

    await page.goto(`${server.baseUrl}/trash`);
    await expect(page.getByRole('cell', { name: 'hello', exact: true })).toBeVisible();

    const emptyButton = page.getByRole('button', { name: 'Empty trash', exact: true }).first();
    await expect(emptyButton).toBeEnabled();
    await emptyButton.click();

    const dialog = page.getByRole('dialog', { name: 'Empty trash' });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText('Permanently delete all 1 books in the trash?')).toBeVisible();

    await dialog.getByRole('button', { name: 'Empty trash', exact: true }).click();

    // The progress bar appears and the sweep finishes at 100%.
    const progressBar = dialog.getByRole('progressbar');
    await expect(progressBar).toBeVisible();
    await expect(progressBar).toHaveAttribute('aria-valuenow', '100');
    await expect(dialog.getByText('The trash is now empty.')).toBeVisible();

    await dialog.getByRole('button', { name: 'Close', exact: true }).click();

    await expect(page.getByText('Trash is empty.')).toBeVisible();

    // The button stays available on an empty-looking trash: the listing hides
    // books whose metadata cannot be read, so the client cannot know the trash
    // is actually empty. In that case the prompt must not promise a count.
    await expect(emptyButton).toBeEnabled();
    await emptyButton.click();
    await expect(dialog.getByText(/Permanently delete everything in the trash\?/)).toBeVisible();
  } finally {
    await server.dispose();
  }
});
