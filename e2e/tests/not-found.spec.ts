import { expect, test } from '@playwright/test';
import { startServer } from './support/server';

// The route table had no catch-all, so an unknown address mounted the shell
// around an empty <RouterView> — a blank content area that reads as a broken
// app rather than a bad link.
test('an unknown address renders the not-found page instead of a blank shell', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books/maintenance/does-not-exist`);

    await expect(page.getByRole('heading', { name: 'Page not found' })).toBeVisible();
    await expect(page.getByText('/books/maintenance/does-not-exist')).toBeVisible();

    // The shell stays usable, and the escape hatch actually goes somewhere.
    await page.getByRole('link', { name: 'Back to the library' }).click();
    await expect(page).toHaveURL(/\/books(\?|$)/);
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
  } finally {
    await server.dispose();
  }
});
