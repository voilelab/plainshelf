import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';

// hello.txt is 74 characters, so it sits far below the default 1000 threshold
// and far above a threshold of 1.
test('low character count page filters books by the threshold input', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    await page
      .locator('#sidebar-section-maintenance')
      .getByRole('link', { name: 'Low Character Count' })
      .click();

    await expect(page).toHaveURL(/\/books\/maintenance\/low-char-count/);
    await expect(page.getByRole('heading', { name: 'Low Character Count' })).toBeVisible();

    const bookRow = page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true });
    await expect(bookRow).toBeVisible();

    const thresholdInput = page.getByLabel('Max characters');
    await expect(thresholdInput).toHaveValue('1000');

    await thresholdInput.fill('1');
    await thresholdInput.blur();

    await expect(page).toHaveURL(/maxChars=1(&|$)/);
    await expect(page.getByText('No books under this character count.')).toBeVisible();

    await thresholdInput.fill('5000');
    await thresholdInput.blur();

    await expect(bookRow).toBeVisible();
  } finally {
    await server.dispose();
  }
});
