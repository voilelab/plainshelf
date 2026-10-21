import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { expectDesktopShellEngaged, openDesktopAt } from './support/desktop';

const getServer = useServer();

test('desktop shell stays engaged across in-app navigation', async ({ page }) => {
  const { baseUrl } = getServer();

  await openDesktopAt(page, baseUrl, '/books');
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
  await expectDesktopShellEngaged(page);

  await page.getByRole('link', { name: 'Home' }).click();
  await expect(page).toHaveURL(/\/home$/);
  await expectDesktopShellEngaged(page);

  await page.getByRole('link', { name: 'Settings' }).click();
  await expect(page).toHaveURL(/\/settings$/);
  await expectDesktopShellEngaged(page);

  await page.getByRole('tab', { name: 'Shelves' }).click();
  await expect(page.getByRole('button', { name: 'Create shelf' })).toBeVisible();
  await expect(
    page.getByText('Shelves are managed by the server configuration.')
  ).toHaveCount(0);
});
