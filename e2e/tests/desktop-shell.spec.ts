import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import {
  desktopHistoryControls,
  expectDesktopShellEngaged,
  openDesktopAt
} from './support/desktop';

const getServer = useServer();

// isWailsRuntime() used to re-read the query string on every call, so the first
// in-app navigation — which drops `?desktop-shell-preview=1` — silently turned
// desktop mode off for the rest of the session. The flag is latched now; these
// assertions are what keeps it latched.
test('desktop shell stays engaged across in-app navigation', async ({ page }) => {
  const { baseUrl } = getServer();

  await openDesktopAt(page, baseUrl, '/books');
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
  await expectDesktopShellEngaged(page);

  // A RouterLink click is a router.push: the URL loses the preview query
  // without a page load, which is exactly the case that used to disengage.
  await page.getByRole('link', { name: 'Home' }).click();
  await expect(page).toHaveURL(/\/home$/);
  await expectDesktopShellEngaged(page);

  await page.getByRole('link', { name: 'Settings' }).click();
  await expect(page).toHaveURL(/\/settings$/);
  await expectDesktopShellEngaged(page);

  // Settings gates shelf management on the same runtime check, so its desktop
  // branch proves the flag reached more than App.vue. The "server managed"
  // note is the non-desktop branch and must stay absent.
  await page.getByRole('tab', { name: 'Shelves' }).click();
  await expect(page.getByRole('button', { name: 'Add shelf' })).toBeVisible();
  await expect(
    page.getByText('Shelves are managed by the server configuration.')
  ).toHaveCount(0);
});

test('web mode shows no desktop chrome', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

  await expect(desktopHistoryControls(page)).toHaveCount(0);
  await page.getByRole('link', { name: 'Settings' }).click();
  await page.getByRole('tab', { name: 'Shelves' }).click();
  await expect(
    page.getByText('Shelves are managed by the server configuration.')
  ).toBeVisible();
  await expect(page.getByRole('button', { name: 'Add shelf' })).toHaveCount(0);
});
