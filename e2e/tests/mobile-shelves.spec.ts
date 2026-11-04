import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import {
  addMobileShelf,
  connectMobile,
  MOBILE_PREVIEW_QUERY,
  reopenMobileAt,
  waitForMobileApp
} from './support/mobile';


const getServer = useServer();

test('starts without a shelf, then keeps several and switches between them', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books?${MOBILE_PREVIEW_QUERY}`);
  await expect(page.getByRole('heading', { name: 'No shelf selected' })).toBeVisible();
  await page.locator('.no-shelf-panel').getByRole('link', { name: 'Manage shelves' }).click();
  await expect(page).toHaveURL(/\/settings\?.*tab=shelves/);
  await expect(page.locator('.mobile-shelves-empty')).toBeVisible();
  await page.getByRole('button', { name: 'Add a shelf' }).click();
  await expect(page.getByRole('heading', { name: 'Connect to PlainShelf' })).toBeVisible();
  await page.getByRole('button', { name: 'Cancel' }).click();
  await expect(page).toHaveURL(new RegExp(`/settings\\?.*${MOBILE_PREVIEW_QUERY}`));

  await connectMobile(page, baseUrl);
  await addMobileShelf(page, baseUrl, { name: 'Second shelf', shelfName: 'Default Shelf' });

  await reopenMobileAt(page, baseUrl, '/connect');
  const rows = page.locator('.mobile-shelves-item');
  await expect(rows).toHaveCount(2);
  await expect(rows.filter({ hasText: 'Second shelf' })).toContainText('In use');

  await expect(rows.first()).toContainText('PlainShelf server');

  await rows.first().getByRole('button', { name: 'Use this shelf' }).click();
  await expect(page).toHaveURL(/\/books(\?|$)/);
  await waitForMobileApp(page);
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

  await reopenMobileAt(page, baseUrl, '/connect');
  await expect(
    page.locator('.mobile-shelves-item').filter({ hasText: 'Second shelf' })
  ).not.toContainText('In use');
});
